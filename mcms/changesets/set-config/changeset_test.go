package setconfig_test

import (
	"crypto/ecdsa"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/mcms/sdk/evm"
	"github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"

	// TODO: remove legacymcms import once remaining MCMS changesets are migrated out of legacy/mcms/changesets.
	legacymcms "github.com/smartcontractkit/cld-changesets/legacy/mcms/changesets"
	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
	solstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	solchangesets "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/changesets"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config/all"
)

//nolint:paralleltest // global mcm.SetProgramID state and shared Solana CTF container setup
func TestChangeset_VerifyPreconditions(t *testing.T) {
	evmSelector := chain_selectors.TEST_90000001.Selector

	rt := newEVMRuntimeWithDeploy(t, evmSelector)
	env := rt.Environment()

	validCfg := cldftesthelpers.SingleGroupMCMS(t)
	invalidCfg := cldftesthelpers.SingleGroupMCMS(t)
	invalidCfg.Quorum = 0

	validTargets := mcmsTargets(evmSelector, validCfg, validCfg, validCfg)
	validMCMS := newMCMSInput(mcmstypes.TimelockActionSchedule, "valid proposal", "")

	evmSelectorStr := strconv.FormatUint(evmSelector, 10)

	tests := []struct {
		name    string
		input   setconfig.Input
		wantErr string
	}{
		{
			name: "valid MCMS input",
			input: setConfigInput(
				validTargets,
				validMCMS,
			),
		},
		{
			name: "valid direct-send input",
			input: setConfigInput(
				validTargets,
				nil,
			),
		},
		{
			name:    "no targets",
			input:   setConfigInput(nil, nil),
			wantErr: "no set-config targets provided",
		},
		{
			name: "chain not in environment",
			input: setConfigInput(
				mcmsTargets(chain_selectors.TEST_90000002.Selector, validCfg, validCfg, validCfg),
				nil,
			),
			wantErr: fmt.Sprintf("family %s: EVM chain %d not found in environment", chain_selectors.FamilyEVM, chain_selectors.TEST_90000002.Selector),
		},
		{
			name: "invalid proposer config",
			input: setConfigInput(
				mcmsTargets(evmSelector, invalidCfg, validCfg, validCfg),
				validMCMS,
			),
			wantErr: "targets[0]: invalid config: invalid MCMS config: Quorum must be greater than 0",
		},
		{
			name: "invalid canceller config",
			input: setConfigInput(
				mcmsTargets(evmSelector, validCfg, invalidCfg, validCfg),
				validMCMS,
			),
			wantErr: "targets[1]: invalid config: invalid MCMS config: Quorum must be greater than 0",
		},
		{
			name: "invalid bypasser config",
			input: setConfigInput(
				mcmsTargets(evmSelector, validCfg, validCfg, invalidCfg),
				validMCMS,
			),
			wantErr: "targets[2]: invalid config: invalid MCMS config: Quorum must be greater than 0",
		},
		{
			name: "partial targets only proposer",
			input: setConfigInput(
				[]setconfig.ContractSetConfig{
					{
						Ref:    contractRef(evmSelector, mcmscontracts.ProposerManyChainMultisig, ""),
						Config: validCfg,
					},
				},
				nil,
			),
		},
		{
			name: "MCMS input missing valid until",
			input: setConfigInput(
				validTargets,
				&cldf.MCMSTimelockProposalInput{
					TimelockAction: mcmstypes.TimelockActionSchedule,
					TimelockDelay:  mcmstypes.NewDuration(time.Second),
				},
			),
			wantErr: "invalid MCMS timelock proposal input: invalid MCMS timelock proposal input: valid until must be set",
		},
		{
			name: "MCMS schedule action requires positive delay",
			input: setConfigInput(
				validTargets,
				&cldf.MCMSTimelockProposalInput{
					TimelockAction: mcmstypes.TimelockActionSchedule,
					ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec
					TimelockDelay:  mcmstypes.NewDuration(0),
				},
			),
			wantErr: "invalid MCMS timelock proposal input: invalid MCMS timelock proposal input: timelock delay must be positive for schedule action",
		},
		{
			name: "ref missing from datastore",
			input: setConfigInput(
				[]setconfig.ContractSetConfig{
					{
						Ref:    contractRef(evmSelector, mcmscontracts.ProposerManyChainMultisig, "does-not-exist"),
						Config: validCfg,
					},
				},
				nil,
			),
			wantErr: fmt.Sprintf(
				"targets[0]: address ref %s_ProposerManyChainMultiSig_1.0.0_does-not-exist: no such address ref can be found for the provided key",
				evmSelectorStr,
			),
		},
	}

	cs := setconfig.Changeset{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := cs.VerifyPreconditions(env, tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChangeset_VerifyPreconditions_Solana(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := newSolanaVerifyPreconditionsEnv(t, selector)

	validCfg := cldftesthelpers.SingleGroupMCMS(t)
	validTargets := mcmsTargets(selector, validCfg, validCfg, validCfg)

	cs := setconfig.Changeset{}
	for _, tt := range []struct {
		name  string
		input setconfig.Input
	}{
		{
			name:  "valid direct-send input",
			input: setConfigInput(validTargets, nil),
		},
		{
			name:  "valid MCMS input",
			input: setConfigInput(validTargets, newMCMSInput(mcmstypes.TimelockActionSchedule, "valid solana proposal", "")),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, cs.VerifyPreconditions(env, tt.input))
		})
	}
}

func TestChangeset_EVM(t *testing.T) {
	t.Parallel()

	selector1 := chain_selectors.TEST_90000001.Selector
	selector2 := chain_selectors.TEST_90000002.Selector

	rt := newEVMRuntimeWithDeploy(t, selector1, selector2)
	chain1 := rt.Environment().BlockChains.EVMChains()[selector1]
	chain2 := rt.Environment().BlockChains.EVMChains()[selector2]
	transferEVMMCMSToTimelock(t, rt, selector2)

	for _, tt := range []struct { //nolint:paralleltest // shared runtime state
		name       string
		chain      cldf_evm.Chain
		selector   uint64
		useMCMS    bool
		extraTasks []runtime.Executable
	}{
		{
			name:     "direct send",
			chain:    chain1,
			selector: selector1,
			useMCMS:  false,
		},
		{
			name:     "MCMS proposal",
			chain:    chain2,
			selector: selector2,
			useMCMS:  true,
			extraTasks: []runtime.Executable{
				runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mcmsState, _ := evmMCMSChainState(t, rt, tt.selector)
			originalCfg := cldftesthelpers.SingleGroupMCMS(t)

			var targets []setconfig.ContractSetConfig
			var mcmsInput *cldf.MCMSTimelockProposalInput
			if tt.useMCMS {
				cfgCanceller := cldftesthelpers.SingleGroupMCMS(t)
				cfgCanceller.Signers = append(cfgCanceller.Signers, mcmsState.BypasserMcm.Address())
				cfgCanceller.Quorum = 2
				targets = []setconfig.ContractSetConfig{
					{
						Ref:    contractRef(tt.selector, mcmscontracts.CancellerManyChainMultisig, ""),
						Config: cfgCanceller,
					},
				}
				mcmsInput = newMCMSInput(mcmstypes.TimelockActionBypass, "Set config proposal", "")
			} else {
				timelockAddress := mcmsState.Timelock.Address()

				cfgProposer := cldftesthelpers.SingleGroupMCMS(t)
				cfgProposer.Signers = append(cfgProposer.Signers, timelockAddress)
				cfgProposer.Quorum = 2
				cfgCanceller := cldftesthelpers.SingleGroupMCMS(t)
				cfgBypasser := cldftesthelpers.SingleGroupMCMS(t)
				cfgBypasser.Signers = append(cfgBypasser.Signers, timelockAddress)
				cfgBypasser.Signers = append(cfgBypasser.Signers, mcmsState.ProposerMcm.Address())
				cfgBypasser.Quorum = 3

				targets = mcmsTargets(tt.selector, cfgProposer, cfgCanceller, cfgBypasser)
			}

			tasks := make([]runtime.Executable, 0, 1+len(tt.extraTasks))
			tasks = append(tasks, runtime.ChangesetTask(setconfig.Changeset{}, setConfigInput(
				targets,
				mcmsInput,
			)))
			tasks = append(tasks, tt.extraTasks...)

			execErr := rt.Exec(tasks...)
			require.NoError(t, execErr)

			inspector := evm.NewInspector(tt.chain.Client)

			if tt.useMCMS {
				cfgCanceller := targets[0].Config
				newConf, err := inspector.GetConfig(t.Context(), mcmsState.CancellerMcm.Address().Hex())
				require.NoError(t, err)
				require.ElementsMatch(t, cfgCanceller.Signers, newConf.Signers)
				require.Equal(t, cfgCanceller.Quorum, newConf.Quorum)

				proposerConf, err := inspector.GetConfig(t.Context(), mcmsState.ProposerMcm.Address().Hex())
				require.NoError(t, err)
				require.ElementsMatch(t, originalCfg.Signers, proposerConf.Signers)
				require.Equal(t, originalCfg.Quorum, proposerConf.Quorum)

				bypasserConf, err := inspector.GetConfig(t.Context(), mcmsState.BypasserMcm.Address().Hex())
				require.NoError(t, err)
				require.ElementsMatch(t, originalCfg.Signers, bypasserConf.Signers)
				require.Equal(t, originalCfg.Quorum, bypasserConf.Quorum)

				return
			}

			cfgProposer := targets[0].Config
			cfgCanceller := targets[1].Config
			cfgBypasser := targets[2].Config

			newConf, err := inspector.GetConfig(t.Context(), mcmsState.ProposerMcm.Address().Hex())
			require.NoError(t, err)
			require.ElementsMatch(t, cfgProposer.Signers, newConf.Signers)
			require.Equal(t, cfgProposer.Quorum, newConf.Quorum)

			newConf, err = inspector.GetConfig(t.Context(), mcmsState.BypasserMcm.Address().Hex())
			require.NoError(t, err)
			require.ElementsMatch(t, cfgBypasser.Signers, newConf.Signers)
			require.Equal(t, cfgBypasser.Quorum, newConf.Quorum)

			newConf, err = inspector.GetConfig(t.Context(), mcmsState.CancellerMcm.Address().Hex())
			require.NoError(t, err)
			require.ElementsMatch(t, cfgCanceller.Signers, newConf.Signers)
			require.Equal(t, cfgCanceller.Quorum, newConf.Quorum)
		})
	}
}

func TestChangeset_EVM_PartialTargets(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector

	rt := newEVMRuntimeWithDeploy(t, selector)
	mcmsState, chain := evmMCMSChainState(t, rt, selector)

	cfgProposer := cldftesthelpers.SingleGroupMCMS(t)
	cfgProposer.Signers = append(cfgProposer.Signers, mcmsState.Timelock.Address())
	cfgProposer.Quorum = 2

	err := rt.Exec(
		runtime.ChangesetTask(setconfig.Changeset{}, setConfigInput(
			[]setconfig.ContractSetConfig{
				{
					Ref:    contractRef(selector, mcmscontracts.ProposerManyChainMultisig, ""),
					Config: cfgProposer,
				},
			},
			nil,
		)),
	)
	require.NoError(t, err)

	inspector := evm.NewInspector(chain.Client)
	originalCfg := cldftesthelpers.SingleGroupMCMS(t)

	proposerConf, err := inspector.GetConfig(t.Context(), mcmsState.ProposerMcm.Address().Hex())
	require.NoError(t, err)
	require.ElementsMatch(t, cfgProposer.Signers, proposerConf.Signers)
	require.Equal(t, cfgProposer.Quorum, proposerConf.Quorum)

	cancellerConf, err := inspector.GetConfig(t.Context(), mcmsState.CancellerMcm.Address().Hex())
	require.NoError(t, err)
	require.ElementsMatch(t, originalCfg.Signers, cancellerConf.Signers)
	require.Equal(t, originalCfg.Quorum, cancellerConf.Quorum)

	bypasserConf, err := inspector.GetConfig(t.Context(), mcmsState.BypasserMcm.Address().Hex())
	require.NoError(t, err)
	require.ElementsMatch(t, originalCfg.Signers, bypasserConf.Signers)
	require.Equal(t, originalCfg.Quorum, bypasserConf.Quorum)
}

func TestChangeset_EVM_Qualifier(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector
	selectorStr := strconv.FormatUint(selector, 10)
	cllccipQualifier := "CLLCCIP"
	rmnmcmsQualifier := "RMNMCMS"

	rt := newEVMRuntime(t, selector)

	cllccipConfig := cldftesthelpers.SingleGroupTimelockConfig(t)
	cllccipConfig.Qualifier = &cllccipQualifier
	rmnmcmsConfig := cldftesthelpers.SingleGroupTimelockConfig(t)
	rmnmcmsConfig.Qualifier = &rmnmcmsQualifier

	err := rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(legacymcms.DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cllccipConfig,
		}),
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(legacymcms.DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: rmnmcmsConfig,
		}),
	)
	require.NoError(t, err)

	cllccipState, err := evmstate.MaybeLoadMCMSWithTimelockStateWithQualifier(rt.Environment(), []uint64{selector}, cllccipQualifier)
	require.NoError(t, err)
	require.NotNil(t, cllccipState[selector])

	cfgProposer := cldftesthelpers.SingleGroupMCMS(t)
	cfgProposer.Signers = append(cfgProposer.Signers, cllccipState[selector].Timelock.Address())
	cfgProposer.Quorum = 2

	for _, tt := range []struct {
		name      string
		qualifier string
		wantErr   string
	}{
		{name: "CLLCCIP qualifier", qualifier: cllccipQualifier},
		{name: "RMNMCMS qualifier", qualifier: rmnmcmsQualifier},
		{
			name:      "missing qualifier",
			qualifier: "",
			wantErr: fmt.Sprintf(
				"family evm: validate timelock ref for chain %s: multiple address refs matched query: expected exactly 1 ref matching query {ChainSelector: %s, Type: %s}, found 2",
				selectorStr,
				selectorStr,
				mcmscontracts.RBACTimelock,
			),
		},
		{
			name:      "unknown qualifier",
			qualifier: "does-not-exist",
			wantErr: fmt.Sprintf(
				"family evm: validate timelock ref for chain %s: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %s, Type: %s, Qualifier: does-not-exist}, found 0",
				selectorStr,
				selectorStr,
				mcmscontracts.RBACTimelock,
			),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := setConfigInput(
				[]setconfig.ContractSetConfig{
					{
						Ref:    contractRef(selector, mcmscontracts.ProposerManyChainMultisig, tt.qualifier),
						Config: cfgProposer,
					},
				},
				newMCMSInput(mcmstypes.TimelockActionSchedule, "qualifier test", tt.qualifier),
			)

			err := setconfig.Changeset{}.VerifyPreconditions(rt.Environment(), input)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChangeset_EVM_BuildsProposalWithoutExecute(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector
	rt := newEVMRuntimeWithDeployAndTransfer(t, selector)

	cfg := cldftesthelpers.SingleGroupMCMS(t)
	taskID, err := runtime.ExecChangeset(rt, setconfig.Changeset{}, setConfigInput(
		mcmsTargets(selector, cfg, cfg, cfg),
		newMCMSInput(mcmstypes.TimelockActionSchedule, "proposal only", ""),
	))
	require.NoError(t, err)

	output, ok := rt.State().Outputs[taskID]
	require.True(t, ok)
	require.Len(t, output.MCMSTimelockProposals, 1)
	require.NotEmpty(t, output.MCMSTimelockProposals[0].Operations)
}

//nolint:paralleltest // global mcm.SetProgramID state and shared Solana CTF container setup
func TestChangeset_Solana(t *testing.T) {
	selector := chain_selectors.TEST_22222222222222222222222222222222222222222222.Selector
	rt := newSolanaRuntimeWithDeploy(t, selector)
	chain := rt.Environment().BlockChains.SolanaChains()[selector]

	addrs, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)
	mcmsState, err := solstate.MaybeLoadMCMSWithTimelockChainState(chain, addrs)
	require.NoError(t, err)
	soltestutils.FundSignerPDAs(t, chain, mcmsState)

	inspector := solana.NewInspector(chain.Client)
	signer1Key, signer1Addr := createSolSigner(t)
	_, signer2Addr := createSolSigner(t)

	newCfgProposer := cldftesthelpers.SingleGroupMCMS(t)
	newCfgProposer.Signers = append(newCfgProposer.Signers, signer1Addr)
	newCfgProposer.Quorum = 2
	newCfgCanceller := cldftesthelpers.SingleGroupMCMS(t)
	newCfgBypasser := cldftesthelpers.SingleGroupMCMS(t)
	newCfgBypasser.Signers = append(newCfgBypasser.Signers, signer1Addr)
	newCfgBypasser.Quorum = 2

	t.Run("direct send", func(t *testing.T) { //nolint:paralleltest // shared runtime state
		err = rt.Exec(
			runtime.ChangesetTask(setconfig.Changeset{}, setConfigInput(
				mcmsTargets(selector, newCfgProposer, newCfgCanceller, newCfgBypasser),
				nil,
			)),
		)
		require.NoError(t, err)

		assertSolConfigEquals(t, inspector, mcmsState.McmProgram, mcmsState.ProposerMcmSeed, newCfgProposer)
		assertSolConfigEquals(t, inspector, mcmsState.McmProgram, mcmsState.BypasserMcmSeed, newCfgBypasser)
		assertSolConfigEquals(t, inspector, mcmsState.McmProgram, mcmsState.CancellerMcmSeed, newCfgCanceller)
	})

	t.Run("MCMS proposal", func(t *testing.T) { //nolint:paralleltest // shared runtime state
		err = rt.Exec(
			runtime.ChangesetTask(solchangesets.TransferMCMSToTimelockSolana{}, solchangesets.TransferMCMSToTimelockSolanaConfig{
				Chains:  []uint64{selector},
				MCMSCfg: cldfproposalutils.TimelockConfig{MinDelay: time.Second},
			}),
			runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner, signer1Key}),
		)
		require.NoError(t, err)

		newCfgProposer.Signers = append(newCfgProposer.Signers, signer2Addr)
		newCfgProposer.Quorum = 3
		newCfgBypasser.Signers = append(newCfgBypasser.Signers, signer2Addr)
		newCfgBypasser.Quorum = 3

		err = rt.Exec(
			runtime.ChangesetTask(setconfig.Changeset{}, setConfigInput(
				mcmsTargets(selector, newCfgProposer, newCfgCanceller, newCfgBypasser),
				newMCMSInput(mcmstypes.TimelockActionSchedule, "solana set config", ""),
			)),
			runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner, signer1Key}),
		)
		require.NoError(t, err)

		assertSolConfigEquals(t, inspector, mcmsState.McmProgram, mcmsState.ProposerMcmSeed, newCfgProposer)
		assertSolConfigEquals(t, inspector, mcmsState.McmProgram, mcmsState.BypasserMcmSeed, newCfgBypasser)
		assertSolConfigEquals(t, inspector, mcmsState.McmProgram, mcmsState.CancellerMcmSeed, newCfgCanceller)
	})
}

func TestChangeset_VerifyPreconditions_zeroRefChainSelector(t *testing.T) {
	t.Parallel()

	cfg := cldftesthelpers.SingleGroupMCMS(t)
	input := setConfigInput(
		[]setconfig.ContractSetConfig{
			{
				Ref: refkey.RefKey{
					ChainSelector: 0,
					Type:          contractRef(chain_selectors.TEST_90000001.Selector, mcmscontracts.ProposerManyChainMultisig, "").Type,
					Version:       &semvers.V1_0_0,
				},
				Config: cfg,
			},
		},
		nil,
	)

	err := setconfig.Changeset{}.VerifyPreconditions(cldf.Environment{}, input)
	require.Error(t, err)
	require.EqualError(t, err, "targets[0]: ref chain selector is required")
}

func TestChangeset_Apply_unsupportedFamily(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.APTOS_TESTNET.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)

	_, err := setconfig.Changeset{}.Apply(cldf.Environment{}, setConfigInput(
		[]setconfig.ContractSetConfig{
			{
				Ref: refkey.New(
					selector,
					contractRef(chain_selectors.TEST_90000001.Selector, mcmscontracts.ProposerManyChainMultisig, "").Type,
					&semvers.V1_0_0,
					"",
				),
				Config: cfg,
			},
		},
		nil,
	))
	require.ErrorContains(t, err, fmt.Sprintf("chain selector %d:", selector))
	require.ErrorContains(t, err, `no sequence registered for family "aptos"`)
}

func assertSolConfigEquals(
	t *testing.T, inspector *solana.Inspector, programID solanago.PublicKey, seed solstate.PDASeed, want mcmstypes.Config,
) {
	t.Helper()

	cfg, err := inspector.GetConfig(t.Context(), solana.ContractAddress(programID, solana.PDASeed(seed)))
	require.NoError(t, err)
	require.ElementsMatch(t, want.Signers, cfg.Signers)
	require.Equal(t, want.Quorum, cfg.Quorum)
}

func createSolSigner(t *testing.T) (*ecdsa.PrivateKey, common.Address) {
	t.Helper()

	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	publicKey := key.Public().(*ecdsa.PublicKey)

	return key, crypto.PubkeyToAddress(*publicKey)
}
