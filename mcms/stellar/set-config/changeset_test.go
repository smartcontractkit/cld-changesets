package stellarsetconfig_test

import (
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"

	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"

	mcmsstellar "github.com/smartcontractkit/mcms/sdk/stellar"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	stellartestutils "github.com/smartcontractkit/cld-changesets/mcms/stellar/internal"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"

	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/deploy"
	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/transfer-to-timelock"
)

//nolint:paralleltest // subtests share one localnet deployment and must run in order
func TestChangeset_Stellar(t *testing.T) {
	selector := chainselectors.STELLAR_LOCALNET.Selector
	rt := stellartestutils.NewStellarRuntime(t, selector)

	initialCfg := cldftesthelpers.SingleGroupTimelockConfig(t)
	stellartestutils.DeployMCMSWithTimelock(
		t,
		rt,
		selector,
		initialCfg,
	)

	chain, ok := rt.Environment().BlockChains.StellarChains()[selector]
	require.True(t, ok)

	deployer, err := stellardeployment.NewDeployerFromChain(chain)
	require.NoError(t, err)

	inspector := mcmsstellar.NewInspectorFromInvoker(deployer)

	proposer := stellartestutils.ResolveContract(
		t,
		rt.Environment(),
		selector,
		mcmscontracts.ProposerManyChainMultisig,
		"",
	)
	canceller := stellartestutils.ResolveContract(
		t,
		rt.Environment(),
		selector,
		mcmscontracts.CancellerManyChainMultisig,
		"",
	)
	bypasser := stellartestutils.ResolveContract(
		t,
		rt.Environment(),
		selector,
		mcmscontracts.BypasserManyChainMultisig,
		"",
	)

	t.Run("governed no-op does not build proposal", func(t *testing.T) {
		root, _, err := inspector.GetRoot(t.Context(), proposer)
		require.NoError(t, err)
		require.Equal(t, common.Hash{}, root)

		taskID, err := runtime.ExecChangeset(
			rt,
			setconfig.Changeset{},
			setConfigInput(
				[]setconfig.ContractSetConfig{
					{
						Ref: stellartestutils.ContractRef(
							selector,
							mcmscontracts.ProposerManyChainMultisig,
							"",
						),
						Config: initialCfg.Proposer,
					},
				},
				newMCMSInput(
					mcmstypes.TimelockActionSchedule,
					"Stellar set-config no-op",
					"",
				),
			),
		)
		require.NoError(t, err)

		if output, ok := rt.State().Outputs[taskID]; ok {
			require.Empty(t, output.MCMSTimelockProposals)
		}

		assertStellarConfigEquals(
			t,
			inspector,
			proposer,
			initialCfg.Proposer,
		)
	})

	t.Run("governed builds proposal without execute", func(t *testing.T) {
		newCfg := cldftesthelpers.SingleGroupMCMS(t)
		newCfg.Signers = append(
			newCfg.Signers,
			createStellarMCMSSigner(t),
		)
		newCfg.Quorum = 2

		taskID, err := runtime.ExecChangeset(
			rt,
			setconfig.Changeset{},
			setConfigInput(
				[]setconfig.ContractSetConfig{
					{
						Ref: stellartestutils.ContractRef(
							selector,
							mcmscontracts.CancellerManyChainMultisig,
							"",
						),
						Config: newCfg,
					},
				},
				newMCMSInput(
					mcmstypes.TimelockActionBypass,
					"Stellar set-config proposal only",
					"",
				),
			),
		)
		require.NoError(t, err)

		output, ok := rt.State().Outputs[taskID]
		require.True(t, ok)
		require.Len(t, output.MCMSTimelockProposals, 1)
		require.NotEmpty(
			t,
			output.MCMSTimelockProposals[0].Operations,
		)

		// Governed proposal construction must not mutate the target.
		assertStellarConfigEquals(
			t,
			inspector,
			canceller,
			initialCfg.Canceller,
		)
	})

	t.Run("direct send updates config", func(t *testing.T) {
		newCfg := cldftesthelpers.SingleGroupMCMS(t)
		newCfg.Signers = append(
			newCfg.Signers,
			createStellarMCMSSigner(t),
		)
		newCfg.Quorum = 2

		err := rt.Exec(
			runtime.ChangesetTask(
				setconfig.Changeset{},
				setConfigInput(
					[]setconfig.ContractSetConfig{
						{
							Ref: stellartestutils.ContractRef(
								selector,
								mcmscontracts.BypasserManyChainMultisig,
								"",
							),
							Config: newCfg,
						},
					},
					nil,
				),
			),
		)
		require.NoError(t, err)

		assertStellarConfigEquals(
			t,
			inspector,
			bypasser,
			newCfg,
		)
	})

	// Runs last: it rewrites the config of all three MCMs. Per-role configs
	// are deliberately distinct (EVM set-config test pattern: quorums 2/1/3
	// with different signer sets) so a swapped proposer/canceller/bypasser
	// pairing anywhere in the pipeline fails the read-back; identical configs
	// would be swap-blind.
	t.Run("direct send applies distinct configs per role", func(t *testing.T) {
		cfgProposer := cldftesthelpers.SingleGroupMCMS(t)
		cfgProposer.Signers = append(cfgProposer.Signers, createStellarMCMSSigner(t))
		cfgProposer.Quorum = 2

		cfgCanceller := cldftesthelpers.SingleGroupMCMS(t)

		cfgBypasser := cldftesthelpers.SingleGroupMCMS(t)
		cfgBypasser.Signers = append(
			cfgBypasser.Signers,
			createStellarMCMSSigner(t),
			createStellarMCMSSigner(t),
		)
		cfgBypasser.Quorum = 3

		err := rt.Exec(
			runtime.ChangesetTask(
				setconfig.Changeset{},
				setConfigInput(
					[]setconfig.ContractSetConfig{
						{
							Ref: stellartestutils.ContractRef(
								selector,
								mcmscontracts.ProposerManyChainMultisig,
								"",
							),
							Config: cfgProposer,
						},
						{
							Ref: stellartestutils.ContractRef(
								selector,
								mcmscontracts.CancellerManyChainMultisig,
								"",
							),
							Config: cfgCanceller,
						},
						{
							Ref: stellartestutils.ContractRef(
								selector,
								mcmscontracts.BypasserManyChainMultisig,
								"",
							),
							Config: cfgBypasser,
						},
					},
					nil,
				),
			),
		)
		require.NoError(t, err)

		assertStellarConfigEquals(t, inspector, proposer, cfgProposer)
		assertStellarConfigEquals(t, inspector, canceller, cfgCanceller)
		assertStellarConfigEquals(t, inspector, bypasser, cfgBypasser)
	})
}

// TestChangeset_Stellar_NonZeroRoot proves that a set-config with an unchanged
// config is NOT a silent no-op when the contract holds a non-zero root: the
// operation must still run set_config so clear_root wipes the stale root. The
// non-zero root arises exactly as in production — by executing a governed
// proposal (here: the transfer-to-timelock acceptance) through the proposer.
//
// This test deliberately uses its own localnet container: it transfers
// ownership to the timelock, which would break TestChangeset_Stellar's
// direct-send subtests if the deployment were shared.
//
//nolint:paralleltest // needs its own localnet container; serialized to avoid parallel-container flake
func TestChangeset_Stellar_NonZeroRoot(t *testing.T) {
	selector := chainselectors.STELLAR_LOCALNET.Selector
	rt := stellartestutils.NewStellarRuntime(t, selector)

	initialCfg := cldftesthelpers.SingleGroupTimelockConfig(t)
	initialCfg.TimelockMinDelay = big.NewInt(0)

	stellartestutils.DeployMCMSWithTimelock(
		t,
		rt,
		selector,
		initialCfg,
	)

	chain, ok := rt.Environment().BlockChains.StellarChains()[selector]
	require.True(t, ok)

	deployer, err := stellardeployment.NewDeployerFromChain(chain)
	require.NoError(t, err)

	inspector := mcmsstellar.NewInspectorFromInvoker(deployer)

	proposer := stellartestutils.ResolveContract(
		t,
		rt.Environment(),
		selector,
		mcmscontracts.ProposerManyChainMultisig,
		"",
	)

	// Transfer ownership to the timelock and execute the acceptance proposal.
	// Executing it calls set_root on the proposer, leaving a non-zero root.
	err = rt.Exec(
		runtime.ChangesetTask(
			transfertotimelock.Changeset{},
			stellarGovernedTransferInput(selector),
		),
	)
	require.NoError(t, err)

	err = rt.Exec(
		runtime.SignAndExecuteProposalsTask(
			[]*ecdsa.PrivateKey{
				cldftesthelpers.TestXXXMCMSSigner,
			},
		),
	)
	require.NoError(t, err)

	root, _, err := inspector.GetRoot(t.Context(), proposer)
	require.NoError(t, err)
	require.NotEqual(t, common.Hash{}, root)

	// Same config + non-zero root: a proposal must still be built.
	taskID, err := runtime.ExecChangeset(
		rt,
		setconfig.Changeset{},
		setConfigInput(
			[]setconfig.ContractSetConfig{
				{
					Ref: stellartestutils.ContractRef(
						selector,
						mcmscontracts.ProposerManyChainMultisig,
						"",
					),
					Config: initialCfg.Proposer,
				},
			},
			newMCMSInput(
				mcmstypes.TimelockActionSchedule,
				"Stellar set-config clears stale root",
				"",
			),
		),
	)
	require.NoError(t, err)

	output, ok := rt.State().Outputs[taskID]
	require.True(t, ok)
	require.Len(t, output.MCMSTimelockProposals, 1)
	require.NotEmpty(t, output.MCMSTimelockProposals[0].Operations)

	// Executing the proposal runs set_config with clear_root=true.
	err = rt.Exec(
		runtime.SignAndExecuteProposalsTask(
			[]*ecdsa.PrivateKey{
				cldftesthelpers.TestXXXMCMSSigner,
			},
		),
	)
	require.NoError(t, err)

	root, _, err = inspector.GetRoot(t.Context(), proposer)
	require.NoError(t, err)
	require.Equal(t, common.Hash{}, root)

	assertStellarConfigEquals(
		t,
		inspector,
		proposer,
		initialCfg.Proposer,
	)
}

func stellarGovernedTransferInput(selector uint64) transfertotimelock.Input {
	return transfertotimelock.Input{
		Cfg: transfertotimelock.Config{
			ContractsByChain: map[uint64][]refkey.RefKey{
				selector: {
					stellartestutils.ContractRef(
						selector,
						mcmscontracts.ProposerManyChainMultisig,
						"",
					),
					stellartestutils.ContractRef(
						selector,
						mcmscontracts.CancellerManyChainMultisig,
						"",
					),
					stellartestutils.ContractRef(
						selector,
						mcmscontracts.BypasserManyChainMultisig,
						"",
					),
				},
			},
		},
		MCMS: &cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil: uint32( //nolint:gosec // test timestamp
				time.Now().
					Add(2 * time.Hour).
					UTC().
					Unix(),
			),
			TimelockDelay: mcmstypes.NewDuration(0),
			Description:   "Transfer Stellar MCMS ownership to timelock",
		},
	}
}

func setConfigInput(
	targets []setconfig.ContractSetConfig,
	mcms *cldf.MCMSTimelockProposalInput,
) setconfig.Input {
	return setconfig.Input{
		Cfg: setconfig.Config{
			Targets: targets,
		},
		MCMS: mcms,
	}
}

func newMCMSInput(
	action mcmstypes.TimelockAction,
	description string,
	qualifier string,
) *cldf.MCMSTimelockProposalInput {
	delay := mcmstypes.NewDuration(time.Second)
	if action == mcmstypes.TimelockActionBypass {
		delay = mcmstypes.NewDuration(0)
	}

	return &cldf.MCMSTimelockProposalInput{
		TimelockAction: action,
		ValidUntil: uint32( //nolint:gosec // test timestamp
			time.Now().
				Add(2 * time.Hour).
				UTC().
				Unix(),
		),
		TimelockDelay: delay,
		Qualifier:     qualifier,
		Description:   description,
	}
}

func createStellarMCMSSigner(t *testing.T) common.Address {
	t.Helper()

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	return crypto.PubkeyToAddress(key.PublicKey)
}

func assertStellarConfigEquals(
	t *testing.T,
	inspector *mcmsstellar.Inspector,
	address string,
	want mcmstypes.Config,
) {
	t.Helper()

	got, err := inspector.GetConfig(t.Context(), address)
	require.NoError(t, err)
	require.True(t, want.Equals(got))
}
