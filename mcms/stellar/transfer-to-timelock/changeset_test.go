package stellartransfertotimelock_test

import (
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

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
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"

	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/deploy"
	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/readers"
)

func TestChangeset_Stellar(t *testing.T) {
	selector := chainselectors.STELLAR_LOCALNET.Selector
	rt := stellartestutils.NewStellarRuntime(t, selector)

	cfg := cldftesthelpers.SingleGroupTimelockConfig(t)
	cfg.TimelockMinDelay = big.NewInt(0)

	stellartestutils.DeployMCMSWithTimelock(
		t,
		rt,
		selector,
		cfg,
	)

	chain, ok := rt.Environment().BlockChains.StellarChains()[selector]
	require.True(t, ok)

	deployer, err := stellardeployment.NewDeployerFromChain(chain)
	require.NoError(t, err)

	inspector := mcmsstellar.NewInspectorFromInvoker(deployer)
	timelockInspector := mcmsstellar.NewTimelockInspectorFromInvoker(deployer)

	timelock := stellartestutils.ResolveContract(
		t,
		rt.Environment(),
		selector,
		mcmscontracts.RBACTimelock,
		"",
	)
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

	contracts := []ownedContract{
		{name: "proposer", address: proposer},
		{name: "canceller", address: canceller},
		{name: "bypasser", address: bypasser},
	}

	deployerAddress := deployer.SignerAddress()

	t.Run("timelock roles", func(t *testing.T) {
		assertTimelockRoles(
			t,
			timelockInspector,
			timelock,
			proposer,
			canceller,
			bypasser,
		)
	})

	t.Run("initial ownership", func(t *testing.T) {
		assertOwnershipState(
			t,
			inspector,
			contracts,
			deployerAddress,
			nil,
		)
	})

	t.Run("only accept ownership fails without pending transfer", func(t *testing.T) {
		err := rt.Exec(
			runtime.ChangesetTask(
				transfertotimelock.Changeset{},
				stellarTransferInput(selector, true),
			),
		)
		require.Error(t, err)
		require.ErrorContains(t, err, "pending")

		assertOwnershipState(
			t,
			inspector,
			contracts,
			deployerAddress,
			nil,
		)
	})

	t.Run("transfers ownership and builds accept proposal", func(t *testing.T) {
		taskID, err := runtime.ExecChangeset(
			rt,
			transfertotimelock.Changeset{},
			stellarTransferInput(selector, false),
		)
		require.NoError(t, err)

		output, ok := rt.State().Outputs[taskID]
		require.True(t, ok)
		require.Len(t, output.MCMSTimelockProposals, 1)
		require.NotEmpty(
			t,
			output.MCMSTimelockProposals[0].Operations,
		)

		require.Len(t, rt.State().GetPendingProposals(), 1)

		assertOwnershipState(
			t,
			inspector,
			contracts,
			deployerAddress,
			&timelock,
		)
	})

	t.Run("executes accept ownership proposal", func(t *testing.T) {
		require.Len(t, rt.State().GetPendingProposals(), 1)

		err := rt.Exec(
			runtime.SignAndExecuteProposalsTask(
				[]*ecdsa.PrivateKey{
					cldftesthelpers.TestXXXMCMSSigner,
				},
			),
		)
		require.NoError(t, err)

		require.Empty(t, rt.State().GetPendingProposals())

		assertOwnershipState(
			t,
			inspector,
			contracts,
			timelock,
			nil,
		)
	})

	t.Run("rerun after ownership accepted is no-op", func(t *testing.T) {
		taskID, err := runtime.ExecChangeset(
			rt,
			transfertotimelock.Changeset{},
			stellarTransferInput(selector, false),
		)
		require.NoError(t, err)

		if output, ok := rt.State().Outputs[taskID]; ok {
			require.Empty(t, output.MCMSTimelockProposals)
		}

		require.Empty(t, rt.State().GetPendingProposals())

		assertOwnershipState(
			t,
			inspector,
			contracts,
			timelock,
			nil,
		)
	})
}

type ownedContract struct {
	name    string
	address string
}

func assertOwnershipState(
	t *testing.T,
	inspector *mcmsstellar.Inspector,
	contracts []ownedContract,
	wantOwner string,
	wantPendingOwner *string,
) {
	t.Helper()

	for _, contract := range contracts {
		owner, err := inspector.GetOwner(
			t.Context(),
			contract.address,
		)
		require.NoError(t, err)
		require.NotNil(t, owner)
		require.Equal(t, wantOwner, *owner, contract.name)

		pendingOwner, err := inspector.GetPendingOwner(
			t.Context(),
			contract.address,
		)
		require.NoError(t, err)

		if wantPendingOwner == nil {
			require.Nil(t, pendingOwner, contract.name)
			continue
		}

		require.NotNil(t, pendingOwner, contract.name)
		require.Equal(
			t,
			*wantPendingOwner,
			*pendingOwner,
			contract.name,
		)
	}
}

func assertTimelockRoles(
	t *testing.T,
	inspector *mcmsstellar.TimelockInspector,
	timelock string,
	proposer string,
	canceller string,
	bypasser string,
) {
	t.Helper()

	proposers, err := inspector.GetProposers(
		t.Context(),
		timelock,
	)
	require.NoError(t, err)
	require.ElementsMatch(
		t,
		[]string{proposer},
		proposers,
	)

	cancellers, err := inspector.GetCancellers(
		t.Context(),
		timelock,
	)
	require.NoError(t, err)
	require.ElementsMatch(
		t,
		[]string{
			proposer,
			canceller,
			bypasser,
		},
		cancellers,
	)

	bypassers, err := inspector.GetBypassers(
		t.Context(),
		timelock,
	)
	require.NoError(t, err)
	require.ElementsMatch(
		t,
		[]string{bypasser},
		bypassers,
	)
}

func stellarTransferInput(
	selector uint64,
	onlyAcceptOwnership bool,
) transfertotimelock.Input {
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
			OnlyAcceptOwnership: onlyAcceptOwnership,
		},
		MCMS: &cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil: uint32(
				time.Now().
					Add(2 * time.Hour).
					UTC().
					Unix(),
			), //nolint:gosec // test timestamp
			TimelockDelay: mcmstypes.NewDuration(0),
			Description:   "Transfer Stellar MCMS ownership to timelock",
		},
	}
}
