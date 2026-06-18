package evmsetconfig

import (
	"context"
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmsevm "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"

	// TODO: remove legacymcms import once remaining MCMS changesets are migrated out of legacy/mcms/changesets.
	legacymcms "github.com/smartcontractkit/cld-changesets/legacy/mcms/changesets"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
	evmreaders "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
)

func TestRunEVMSetConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		transferToMCMS bool
	}{
		{name: "direct send", transferToMCMS: false},
		{name: "MCMS proposal", transferToMCMS: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			selector := chainselectors.TEST_90000001.Selector
			rt := newEVMSetConfigRuntime(t, selector)
			refs := evmSetConfigRefs(t, rt.Environment(), selector)
			chain := rt.Environment().BlockChains.EVMChains()[selector]

			if tt.transferToMCMS {
				transferEVMMCMSToTimelock(t, rt, selector, refs)
			}

			proposerCfg := cldftesthelpers.SingleGroupMCMS(t)
			proposerCfg.Signers = append(proposerCfg.Signers, refs.Timelock)
			proposerCfg.Quorum = 2

			bypasserCfg := cldftesthelpers.SingleGroupMCMS(t)
			bypasserCfg.Signers = append(bypasserCfg.Signers, refs.Proposer)
			bypasserCfg.Quorum = 2

			cancellerCfg := cldftesthelpers.SingleGroupMCMS(t)
			cancellerCfg.Signers = append(cancellerCfg.Signers, refs.Bypasser)
			cancellerCfg.Quorum = 2

			targets := []setconfig.ContractSetConfig{
				{
					Ref:    refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, ""),
					Config: proposerCfg,
				},
				{
					Ref:    refkey.New(selector, datastore.ContractType(mcmscontracts.BypasserManyChainMultisig), &semvers.V1_0_0, ""),
					Config: bypasserCfg,
				},
			}
			if tt.transferToMCMS {
				targets = []setconfig.ContractSetConfig{
					{
						Ref:    refkey.New(selector, datastore.ContractType(mcmscontracts.CancellerManyChainMultisig), &semvers.V1_0_0, ""),
						Config: cancellerCfg,
					},
				}
			}

			out, err := runEVMSetConfig(
				rt.Environment().OperationsBundle,
				setconfig.Deps{
					BlockChains: rt.Environment().BlockChains,
					DataStore:   rt.Environment().DataStore,
				},
				setconfig.ChainInput{
					ChainSelector: selector,
					Targets:       targets,
				},
			)
			require.NoError(t, err)

			if tt.transferToMCMS {
				require.Len(t, out.BatchOps, 1)
				require.Len(t, out.BatchOps[0].Transactions, 1)
				require.NoError(t, rt.Exec(
					newTimelockProposalTask(out.BatchOps, "set config sequence test"),
					runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
				))
			} else {
				require.Empty(t, out.BatchOps)
			}

			inspector := mcmsevm.NewInspector(chain.Client)
			if tt.transferToMCMS {
				assertEVMConfigEquals(t, inspector, refs.Canceller, cancellerCfg)
			} else {
				assertEVMConfigEquals(t, inspector, refs.Proposer, proposerCfg)
				assertEVMConfigEquals(t, inspector, refs.Bypasser, bypasserCfg)
			}
		})
	}
}

func newEVMSetConfigRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(legacymcms.DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	return rt
}

type evmMCMSRefs struct {
	Timelock  common.Address
	Proposer  common.Address
	Canceller common.Address
	Bypasser  common.Address
}

func evmSetConfigRefs(t *testing.T, env cldf.Environment, selector uint64) evmMCMSRefs {
	t.Helper()

	reader := evmreaders.Reader{}
	timelock, err := reader.GetTimelockRef(env, selector, cldf.MCMSTimelockProposalInput{})
	require.NoError(t, err)
	proposer, err := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
	})
	require.NoError(t, err)
	canceller, err := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionCancel,
	})
	require.NoError(t, err)
	bypasser, err := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionBypass,
	})
	require.NoError(t, err)

	return evmMCMSRefs{
		Timelock:  common.HexToAddress(timelock.Address),
		Proposer:  common.HexToAddress(proposer.Address),
		Canceller: common.HexToAddress(canceller.Address),
		Bypasser:  common.HexToAddress(bypasser.Address),
	}
}

func transferEVMMCMSToTimelock(t *testing.T, rt *runtime.Runtime, selector uint64, refs evmMCMSRefs) {
	t.Helper()

	err := rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(legacymcms.TransferToMCMSWithTimelockV2), legacymcms.TransferToMCMSWithTimelockConfig{
			ContractsByChain: map[uint64][]common.Address{
				selector: {
					refs.Proposer,
					refs.Bypasser,
					refs.Canceller,
				},
			},
		}),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)
}

type timelockProposalTask struct {
	id          string
	batchOps    []mcmstypes.BatchOperation
	description string
}

func newTimelockProposalTask(batchOps []mcmstypes.BatchOperation, description string) timelockProposalTask {
	return timelockProposalTask{
		id:          ksuid.New().String(),
		batchOps:    batchOps,
		description: description,
	}
}

func (t timelockProposalTask) ID() string {
	return t.id
}

func (t timelockProposalTask) Run(e cldf.Environment, state *runtime.State) error {
	out, err := cldf.NewOutputBuilder(e, datastore.NewMemoryDataStore()).
		WithTimelockProposal(cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionBypass,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			TimelockDelay:  mcmstypes.NewDuration(0),
			Description:    t.description,
		}, t.batchOps).
		Build()
	if err != nil {
		return err
	}

	return state.MergeChangesetOutput(t.id, out)
}

func assertEVMConfigEquals(t *testing.T, inspector *mcmsevm.Inspector, address common.Address, want mcmstypes.Config) {
	t.Helper()

	got, err := inspector.GetConfig(t.Context(), address.Hex())
	require.NoError(t, err)
	require.ElementsMatch(t, want.Signers, got.Signers)
	require.Equal(t, want.Quorum, got.Quorum)
}

func TestSetConfigTargets(t *testing.T) {
	t.Parallel()

	const selector uint64 = 90000001
	cfg := cldftesthelpers.SingleGroupMCMS(t)
	validRef := refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, "")

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "0x0000000000000000000000000000000000000100",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       semver.MustParse("1.0.0"),
	}))
	env := cldf.Environment{
		DataStore:  ds.Seal(),
		GetContext: context.Background,
	}

	got, err := setConfigTargets(env, []setconfig.ContractSetConfig{
		{Ref: validRef, Config: cfg},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "0x0000000000000000000000000000000000000100", got[0].Address.Hex())
	require.Equal(t, cfg, got[0].Config)

	_, err = setConfigTargets(env, []setconfig.ContractSetConfig{
		{Ref: refkey.New(selector, datastore.ContractType(mcmscontracts.CancellerManyChainMultisig), &semvers.V1_0_0, ""), Config: cfg},
	})
	require.ErrorContains(t, err, "targets[0]:")

	invalidDS := datastore.NewMemoryDataStore()
	require.NoError(t, invalidDS.Addresses().Add(datastore.AddressRef{
		Address:       "not-an-address",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       semver.MustParse("1.0.0"),
	}))
	_, err = setConfigTargets(cldf.Environment{DataStore: invalidDS.Seal(), GetContext: context.Background}, []setconfig.ContractSetConfig{
		{Ref: validRef, Config: cfg},
	})
	require.ErrorContains(t, err, `invalid EVM address "not-an-address"`)
}

func TestRunEVMSetConfig_errors(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)

	_, err := runEVMSetConfig(
		optest.NewBundle(t),
		setconfig.Deps{BlockChains: chain.NewBlockChains(nil)},
		setconfig.ChainInput{ChainSelector: selector},
	)
	require.ErrorContains(t, err, "EVM chain")

	deps := setconfig.Deps{
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldf_evm.Chain{Selector: selector},
		}),
		DataStore: datastore.NewMemoryDataStore().Seal(),
	}
	_, err = runEVMSetConfig(
		optest.NewBundle(t),
		deps,
		setconfig.ChainInput{
			ChainSelector: selector,
			Targets: []setconfig.ContractSetConfig{
				{
					Ref:    refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, ""),
					Config: cfg,
				},
			},
		},
	)
	require.ErrorContains(t, err, "targets[0]:")
}

func TestRunEVMSetConfig_success(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	rt := newEVMSetConfigRuntime(t, selector)
	_ = evmSetConfigRefs(t, rt.Environment(), selector)
	cfg := cldftesthelpers.SingleGroupMCMS(t)
	proposerRef := refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, "")

	out, err := runEVMSetConfig(
		rt.Environment().OperationsBundle,
		setconfig.Deps{
			BlockChains: rt.Environment().BlockChains,
			DataStore:   rt.Environment().DataStore,
		},
		setconfig.ChainInput{
			ChainSelector: selector,
			Targets:       []setconfig.ContractSetConfig{{Ref: proposerRef, Config: cfg}},
		},
	)
	require.NoError(t, err)
	require.Empty(t, out.BatchOps)
}
