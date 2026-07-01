package evmsetconfig_test

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmsevm "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	deploy "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy"
	evmreaders "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
	evmsetconfig "github.com/smartcontractkit/cld-changesets/mcms/evm/set-config"
)

func newEVMSetConfigRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
			ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
				selector: cldftesthelpers.SingleGroupTimelockConfig(t),
			},
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

func TestOpEVMSetConfigMCM_missingDeployerKey(t *testing.T) {
	t.Parallel()

	_, err := operations.ExecuteOperation(
		optest.NewBundle(t),
		evmsetconfig.OpEVMSetConfigMCM,
		cldf_evm.Chain{Selector: chainselectors.TEST_90000001.Selector},
		evmsetconfig.OpEVMSetConfigInput{
			NoSend: false,
			Target: evmsetconfig.MCMSetConfigTarget{
				Address: common.Address{1},
			},
		},
	)
	require.ErrorContains(t, err, "missing deployer key")
}

func TestOpEVMSetConfigInputGasOverridable(t *testing.T) {
	t.Parallel()

	in := evmsetconfig.OpEVMSetConfigInput{GasLimit: 100, GasPrice: 200}
	gotLimit, gotPrice := in.GasBoostValues()
	require.Equal(t, uint64(100), gotLimit)
	require.Equal(t, uint64(200), gotPrice)

	boosted := in.WithGasBoost(500, 600)
	require.Equal(t, uint64(500), boosted.GasLimit)
	require.Equal(t, uint64(600), boosted.GasPrice)
}

func TestOpEVMSetConfigMCM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		noSend bool
	}{
		{name: "direct send", noSend: false},
		{name: "MCMS proposal", noSend: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			selector := chainselectors.TEST_90000001.Selector
			rt := newEVMSetConfigRuntime(t, selector)
			refs := evmSetConfigRefs(t, rt.Environment(), selector)
			chain := rt.Environment().BlockChains.EVMChains()[selector]

			if tt.noSend {
				transferEVMMCMSToTimelock(t, rt, selector)
			}

			cfg := cldftesthelpers.SingleGroupMCMS(t)
			cfg.Signers = append(cfg.Signers, refs.Timelock)
			cfg.Quorum = 2

			report, err := operations.ExecuteOperation(
				rt.Environment().OperationsBundle,
				evmsetconfig.OpEVMSetConfigMCM,
				chain,
				evmsetconfig.OpEVMSetConfigInput{
					Target: evmsetconfig.MCMSetConfigTarget{
						Address:      refs.Canceller,
						Config:       cfg,
						ContractType: mcmscontracts.CancellerManyChainMultisig,
					},
					NoSend: tt.noSend,
				},
			)
			require.NoError(t, err)
			require.Equal(t, refs.Canceller.Hex(), report.Output.Tx.To)
			require.NotEmpty(t, report.Output.Tx.Data)
			require.Equal(t, !tt.noSend, report.Output.Executed())

			if tt.noSend {
				batch, err := opscontract.NewBatchOperationFromWrites([]opscontract.WriteOutput{report.Output})
				require.NoError(t, err)
				require.Len(t, batch.Transactions, 1)
				require.NoError(t, rt.Exec(
					newTimelockProposalTask([]mcmstypes.BatchOperation{batch}, "evm set config operation test"),
					runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
				))
			}

			assertEVMConfigEquals(t, mcmsevm.NewInspector(chain.Client), refs.Canceller, cfg)
		})
	}
}
