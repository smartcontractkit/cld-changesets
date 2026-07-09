package evmsetconfig_test

import (
	"crypto/ecdsa"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"

	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
	deploy "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"

	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/transfer-to-timelock"
)

func TestChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

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
		{name: "valid MCMS input", input: setConfigInput(validTargets, validMCMS)},
		{name: "valid direct-send input", input: setConfigInput(validTargets, nil)},
		{name: "no targets", input: setConfigInput(nil, nil), wantErr: "no set-config targets provided"},
		{
			name: "chain not in environment",
			input: setConfigInput(
				mcmsTargets(chain_selectors.TEST_90000002.Selector, validCfg, validCfg, validCfg),
				nil,
			),
			wantErr: fmt.Sprintf("family %s: EVM chain %d not found in environment", chain_selectors.FamilyEVM, chain_selectors.TEST_90000002.Selector),
		},
		{
			name:    "invalid proposer config",
			input:   setConfigInput(mcmsTargets(evmSelector, invalidCfg, validCfg, validCfg), validMCMS),
			wantErr: "targets[0]: invalid config: invalid MCMS config: Quorum must be greater than 0",
		},
		{
			name:    "invalid canceller config",
			input:   setConfigInput(mcmsTargets(evmSelector, validCfg, invalidCfg, validCfg), validMCMS),
			wantErr: "targets[1]: invalid config: invalid MCMS config: Quorum must be greater than 0",
		},
		{
			name:    "invalid bypasser config",
			input:   setConfigInput(mcmsTargets(evmSelector, validCfg, validCfg, invalidCfg), validMCMS),
			wantErr: "targets[2]: invalid config: invalid MCMS config: Quorum must be greater than 0",
		},
		{
			name: "partial targets only proposer",
			input: setConfigInput(
				[]setconfig.ContractSetConfig{
					{Ref: contractRef(evmSelector, mcmscontracts.ProposerManyChainMultisig, ""), Config: validCfg},
				},
				nil,
			),
		},
		{
			name: "MCMS input missing valid until",
			input: setConfigInput(validTargets, &cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionSchedule,
				TimelockDelay:  mcmstypes.NewDuration(time.Second),
			}),
			wantErr: "invalid MCMS timelock proposal input: invalid MCMS timelock proposal input: valid until must be set",
		},
		{
			name: "MCMS schedule action requires positive delay",
			input: setConfigInput(validTargets, &cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionSchedule,
				ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
				TimelockDelay:  mcmstypes.NewDuration(0),
			}),
			wantErr: "invalid MCMS timelock proposal input: invalid MCMS timelock proposal input: timelock delay must be positive for schedule action",
		},
		{
			name: "ref missing from datastore",
			input: setConfigInput(
				[]setconfig.ContractSetConfig{
					{Ref: contractRef(evmSelector, mcmscontracts.ProposerManyChainMultisig, "does-not-exist"), Config: validCfg},
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
				cfgCanceller.Signers = append(cfgCanceller.Signers, mcmsState.Bypasser)
				cfgCanceller.Quorum = 2
				targets = []setconfig.ContractSetConfig{
					{
						Ref:    contractRef(tt.selector, mcmscontracts.CancellerManyChainMultisig, ""),
						Config: cfgCanceller,
					},
				}
				mcmsInput = newMCMSInput(mcmstypes.TimelockActionBypass, "Set config proposal", "")
			} else {
				timelockAddress := mcmsState.Timelock

				cfgProposer := cldftesthelpers.SingleGroupMCMS(t)
				cfgProposer.Signers = append(cfgProposer.Signers, timelockAddress)
				cfgProposer.Quorum = 2
				cfgCanceller := cldftesthelpers.SingleGroupMCMS(t)
				cfgBypasser := cldftesthelpers.SingleGroupMCMS(t)
				cfgBypasser.Signers = append(cfgBypasser.Signers, timelockAddress)
				cfgBypasser.Signers = append(cfgBypasser.Signers, mcmsState.Proposer)
				cfgBypasser.Quorum = 3

				targets = mcmsTargets(tt.selector, cfgProposer, cfgCanceller, cfgBypasser)
			}

			tasks := make([]runtime.Executable, 0, 1+len(tt.extraTasks))
			tasks = append(tasks, runtime.ChangesetTask(setconfig.Changeset{}, setConfigInput(targets, mcmsInput)))
			tasks = append(tasks, tt.extraTasks...)

			execErr := rt.Exec(tasks...)
			require.NoError(t, execErr)

			inspector := evm.NewInspector(tt.chain.Client)

			if tt.useMCMS {
				cfgCanceller := targets[0].Config
				newConf, err := inspector.GetConfig(t.Context(), mcmsState.Canceller.Hex())
				require.NoError(t, err)
				require.ElementsMatch(t, cfgCanceller.Signers, newConf.Signers)
				require.Equal(t, cfgCanceller.Quorum, newConf.Quorum)

				proposerConf, err := inspector.GetConfig(t.Context(), mcmsState.Proposer.Hex())
				require.NoError(t, err)
				require.ElementsMatch(t, originalCfg.Signers, proposerConf.Signers)
				require.Equal(t, originalCfg.Quorum, proposerConf.Quorum)

				bypasserConf, err := inspector.GetConfig(t.Context(), mcmsState.Bypasser.Hex())
				require.NoError(t, err)
				require.ElementsMatch(t, originalCfg.Signers, bypasserConf.Signers)
				require.Equal(t, originalCfg.Quorum, bypasserConf.Quorum)

				return
			}

			cfgProposer := targets[0].Config
			cfgCanceller := targets[1].Config
			cfgBypasser := targets[2].Config

			newConf, err := inspector.GetConfig(t.Context(), mcmsState.Proposer.Hex())
			require.NoError(t, err)
			require.ElementsMatch(t, cfgProposer.Signers, newConf.Signers)
			require.Equal(t, cfgProposer.Quorum, newConf.Quorum)

			newConf, err = inspector.GetConfig(t.Context(), mcmsState.Bypasser.Hex())
			require.NoError(t, err)
			require.ElementsMatch(t, cfgBypasser.Signers, newConf.Signers)
			require.Equal(t, cfgBypasser.Quorum, newConf.Quorum)

			newConf, err = inspector.GetConfig(t.Context(), mcmsState.Canceller.Hex())
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
	cfgProposer.Signers = append(cfgProposer.Signers, mcmsState.Timelock)
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

	proposerConf, err := inspector.GetConfig(t.Context(), mcmsState.Proposer.Hex())
	require.NoError(t, err)
	require.ElementsMatch(t, cfgProposer.Signers, proposerConf.Signers)
	require.Equal(t, cfgProposer.Quorum, proposerConf.Quorum)

	cancellerConf, err := inspector.GetConfig(t.Context(), mcmsState.Canceller.Hex())
	require.NoError(t, err)
	require.ElementsMatch(t, originalCfg.Signers, cancellerConf.Signers)
	require.Equal(t, originalCfg.Quorum, cancellerConf.Quorum)

	bypasserConf, err := inspector.GetConfig(t.Context(), mcmsState.Bypasser.Hex())
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
		runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
			ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{selector: cllccipConfig},
		}),
		runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
			ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{selector: rmnmcmsConfig},
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

func contractRef(chainSelector uint64, contractType cldf.ContractType, qualifier string) refkey.RefKey {
	return refkey.New(chainSelector, cldfdatastore.ContractType(contractType), &semvers.V1_0_0, qualifier)
}

func mcmsTargets(
	chainSelector uint64,
	proposer, canceller, bypasser mcmstypes.Config,
) []setconfig.ContractSetConfig {
	return []setconfig.ContractSetConfig{
		{Ref: contractRef(chainSelector, mcmscontracts.ProposerManyChainMultisig, ""), Config: proposer},
		{Ref: contractRef(chainSelector, mcmscontracts.CancellerManyChainMultisig, ""), Config: canceller},
		{Ref: contractRef(chainSelector, mcmscontracts.BypasserManyChainMultisig, ""), Config: bypasser},
	}
}

func newMCMSInput(action mcmstypes.TimelockAction, description, qualifier string) *cldf.MCMSTimelockProposalInput {
	delay := mcmstypes.NewDuration(time.Second)
	if action == mcmstypes.TimelockActionBypass {
		delay = mcmstypes.NewDuration(0)
	}

	return &cldf.MCMSTimelockProposalInput{
		TimelockAction: action,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  delay,
		Qualifier:      qualifier,
		Description:    description,
	}
}

func setConfigInput(targets []setconfig.ContractSetConfig, mcms *cldf.MCMSTimelockProposalInput) setconfig.Input {
	return setconfig.Input{
		Cfg:  setconfig.Config{Targets: targets},
		MCMS: mcms,
	}
}

func newEVMRuntime(t *testing.T, selectors ...uint64) *runtime.Runtime {
	t.Helper()

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, selectors),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	return rt
}

func newEVMRuntimeWithDeploy(t *testing.T, selectors ...uint64) *runtime.Runtime {
	t.Helper()

	rt := newEVMRuntime(t, selectors...)

	cfg := cldftesthelpers.SingleGroupTimelockConfig(t)
	configByChain := make(map[uint64]cldfproposalutils.MCMSWithTimelockConfig, len(selectors))
	for _, selector := range selectors {
		configByChain[selector] = cfg
	}
	err := rt.Exec(runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{ConfigByChain: configByChain}))
	require.NoError(t, err)

	return rt
}

func transferEVMMCMSToTimelock(t *testing.T, rt *runtime.Runtime, selector uint64) {
	t.Helper()

	err := rt.Exec(
		runtime.ChangesetTask(transfertotimelock.Changeset{}, transfertotimelock.Input{
			Cfg: transfertotimelock.Config{
				ContractsByChain: map[uint64][]refkey.RefKey{
					selector: {
						contractRef(selector, mcmscontracts.ProposerManyChainMultisig, ""),
						contractRef(selector, mcmscontracts.BypasserManyChainMultisig, ""),
						contractRef(selector, mcmscontracts.CancellerManyChainMultisig, ""),
					},
				},
			},
			MCMS: &cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionBypass,
				ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
				TimelockDelay:  mcmstypes.NewDuration(0),
				Description:    "Transfer MCMS ownership to timelock",
			},
		}),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)
}

func newEVMRuntimeWithDeployAndTransfer(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	rt := newEVMRuntimeWithDeploy(t, selector)
	transferEVMMCMSToTimelock(t, rt, selector)

	return rt
}

type evmMCMSChainRefs struct {
	Timelock  common.Address
	Proposer  common.Address
	Canceller common.Address
	Bypasser  common.Address
}

func evmMCMSChainState(t *testing.T, rt *runtime.Runtime, selector uint64) (evmMCMSChainRefs, cldf_evm.Chain) {
	t.Helper()

	chain := rt.Environment().BlockChains.EVMChains()[selector]
	env := rt.Environment()

	resolve := func(contractType cldf.ContractType) common.Address {
		resolved, err := contractRef(selector, contractType, "").Resolve(env)
		require.NoError(t, err)

		return common.HexToAddress(resolved.Address)
	}

	return evmMCMSChainRefs{
		Timelock:  resolve(mcmscontracts.RBACTimelock),
		Proposer:  resolve(mcmscontracts.ProposerManyChainMultisig),
		Canceller: resolve(mcmscontracts.CancellerManyChainMultisig),
		Bypasser:  resolve(mcmscontracts.BypasserManyChainMultisig),
	}, chain
}
