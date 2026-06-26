package operations_test

import (
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmslib "github.com/smartcontractkit/mcms"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	mcmschangesets "github.com/smartcontractkit/cld-changesets/legacy/mcms/changesets"
	evmops "github.com/smartcontractkit/cld-changesets/legacy/mcms/operations"
	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
)

func TestCloneTransactOptsWithGas(t *testing.T) {
	t.Parallel()

	orig := &bind.TransactOpts{GasLimit: 100, GasPrice: big.NewInt(123)}

	cloned := evmops.CloneTransactOptsWithGas(orig, 200, 456)
	assert.NotSame(t, orig, cloned)
	assert.Equal(t, uint64(200), cloned.GasLimit)
	assert.Equal(t, big.NewInt(456), cloned.GasPrice)

	cloned2 := evmops.CloneTransactOptsWithGas(orig, 0, 0)
	assert.Equal(t, orig.GasLimit, cloned2.GasLimit)
	assert.Equal(t, orig.GasPrice, cloned2.GasPrice)

	assert.Nil(t, evmops.CloneTransactOptsWithGas(nil, 1, 1))
}

func TestGasBoostConfigsForChainMap(t *testing.T) {
	t.Parallel()

	chainMap := map[uint64]string{1: "a", 2: "b"}
	gasBoostConfigs := map[uint64]cldfproposalutils.GasBoostConfig{
		1: {InitialGasLimit: 10},
	}
	cfgs := evmops.GasBoostConfigsForChainMap(chainMap, gasBoostConfigs)
	assert.Len(t, cfgs, 2)
	assert.NotNil(t, cfgs[1])
	assert.Nil(t, cfgs[2])

	assert.Empty(t, evmops.GasBoostConfigsForChainMap[string](chainMap, nil))
	assert.Empty(t, evmops.GasBoostConfigsForChainMap[string](nil, gasBoostConfigs))
}

func TestGetBoostedGasForAttempt(t *testing.T) {
	t.Parallel()

	cfg := cldfproposalutils.GasBoostConfig{}
	limit, price := evmops.GetBoostedGasForAttempt(cfg, 0)
	assert.Equal(t, uint64(200_000), limit)
	assert.Equal(t, uint64(20_000_000_000), price)

	limit, price = evmops.GetBoostedGasForAttempt(cfg, 2)
	assert.Equal(t, uint64(200_000+2*50_000), limit)
	assert.Equal(t, uint64(20_000_000_000+2*10_000_000_000), price)

	cfg = cldfproposalutils.GasBoostConfig{
		InitialGasLimit:   1000,
		GasLimitIncrement: 100,
		InitialGasPrice:   2000,
		GasPriceIncrement: 100,
	}
	limit, price = evmops.GetBoostedGasForAttempt(cfg, 3)
	assert.Equal(t, uint64(1000+3*100), limit)
	assert.Equal(t, uint64(2000+3*100), price)
}

func TestRetryWithGasBoost(t *testing.T) {
	t.Parallel()

	cfg := &cldfproposalutils.GasBoostConfig{InitialGasLimit: 1000, GasLimitIncrement: 100}
	assert.NotNil(t, evmops.RetryDeploymentWithGasBoost[any](cfg))
	assert.NotNil(t, evmops.RetryDeploymentWithGasBoost[string](nil))
	assert.NotNil(t, evmops.RetryCallWithGasBoost[any](cfg))
	assert.NotNil(t, evmops.RetryCallWithGasBoost[string](nil))
}

func TestContractOpts_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		opts       *evmops.ContractOpts
		isZkSyncVM bool
		err        string
	}{
		{
			desc:       "valid evm opts",
			opts:       &evmops.ContractOpts{Version: semver.MustParse("1.0.0"), EVMBytecode: []byte{0x01}},
			isZkSyncVM: false,
		},
		{
			desc:       "valid zksyncvm opts",
			opts:       &evmops.ContractOpts{Version: semver.MustParse("1.0.0"), ZkSyncVMBytecode: []byte{0x05}},
			isZkSyncVM: true,
		},
		{
			desc: "nil version",
			opts: &evmops.ContractOpts{},
			err:  "version must be defined",
		},
		{
			desc:       "missing evm bytecode",
			opts:       &evmops.ContractOpts{Version: semver.MustParse("1.0.0")},
			isZkSyncVM: false,
			err:        "evm bytecode must be defined",
		},
		{
			desc:       "missing zkSyncVM bytecode",
			opts:       &evmops.ContractOpts{Version: semver.MustParse("1.0.0")},
			isZkSyncVM: true,
			err:        "zkSyncVM bytecode must be defined",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			err := test.opts.Validate(test.isZkSyncVM)
			if test.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.err)
			}
		})
	}
}

func TestNewEVMDeployOperation_Errors(t *testing.T) {
	t.Parallel()

	sel := chain_selectors.TEST_90000001.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{sel}),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)
	env := rt.Environment()
	chain := env.BlockChains.EVMChains()[sel]
	bundle := env.OperationsBundle

	noArgs := func(any) []any { return []any{} }

	t.Run("nil metadata", func(t *testing.T) {
		t.Parallel()
		op := evmops.NewEVMDeployOperation[any]("op-nil-meta", semver.MustParse("1.0.0"), "d", "T", nil,
			&evmops.ContractOpts{Version: semver.MustParse("1.0.0"), EVMBytecode: []byte{0x01}}, noArgs)
		_, err := operations.ExecuteOperation(bundle, op, chain, evmops.EVMDeployInput[any]{ChainSelector: sel})
		require.ErrorContains(t, err, "contract metadata must be provided")
	})

	t.Run("no contract opts", func(t *testing.T) {
		t.Parallel()
		op := evmops.NewEVMDeployOperation[any]("op-no-opts", semver.MustParse("1.0.0"), "d",
			mcmscontracts.CallProxy, bindings.CallProxyMetaData, nil, noArgs)
		_, err := operations.ExecuteOperation(bundle, op, chain, evmops.EVMDeployInput[any]{ChainSelector: sel})
		require.ErrorContains(t, err, "must define ContractOpts")
	})

	t.Run("invalid contract opts", func(t *testing.T) {
		t.Parallel()
		op := evmops.NewEVMDeployOperation[any]("op-bad-opts", semver.MustParse("1.0.0"), "d",
			mcmscontracts.CallProxy, bindings.CallProxyMetaData, nil, noArgs)
		_, err := operations.ExecuteOperation(bundle, op, chain, evmops.EVMDeployInput[any]{
			ChainSelector: sel,
			ContractOpts:  &evmops.ContractOpts{},
		})
		require.ErrorContains(t, err, "version must be defined")
	})

	t.Run("deploy failure", func(t *testing.T) {
		t.Parallel()
		badArgs := func(any) []any { return []any{"not-an-address"} }
		op := evmops.NewEVMDeployOperation[any]("op-deploy-fail", semver.MustParse("1.0.0"), "d",
			mcmscontracts.CallProxy, bindings.CallProxyMetaData,
			&evmops.ContractOpts{Version: semver.MustParse("1.0.0"), EVMBytecode: common.FromHex(bindings.CallProxyBin)},
			badArgs)
		_, err := operations.ExecuteOperation(bundle, op, chain,
			evmops.EVMDeployInput[any]{ChainSelector: sel})
		require.Error(t, err)
	})
}

func TestAddEVMCallSequenceToCSOutput_SequenceError(t *testing.T) {
	t.Parallel()

	env, err := environment.New(t.Context(),
		environment.WithEVMSimulatedN(t, 1),
	)
	require.NoError(t, err)

	csOutput := cldf.ChangesetOutput{}
	seqReport := operations.SequenceReport[string, map[uint64][]evmops.EVMCallOutput]{}
	seqErr := errors.New("sequence failed")

	result, err := evmops.AddEVMCallSequenceToCSOutput(
		*env,
		csOutput,
		seqReport,
		seqErr,
		nil,
		nil,
		"test",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute")
	assert.Contains(t, err.Error(), "sequence failed")
	assert.Equal(t, seqReport.ExecutionReports, result.Reports)
}

func TestAddEVMCallSequenceToCSOutput_NoMCMS(t *testing.T) {
	t.Parallel()

	env, err := environment.New(t.Context(),
		environment.WithEVMSimulatedN(t, 1),
	)
	require.NoError(t, err)

	csOutput := cldf.ChangesetOutput{}
	seqReport := operations.SequenceReport[string, map[uint64][]evmops.EVMCallOutput]{}

	result, err := evmops.AddEVMCallSequenceToCSOutput(
		*env,
		csOutput,
		seqReport,
		nil,
		nil,
		nil,
		"test",
	)

	require.NoError(t, err)
	assert.Equal(t, seqReport.ExecutionReports, result.Reports)
}

func TestAddEVMCallSequenceToCSOutput_AllConfirmed(t *testing.T) {
	t.Parallel()

	env, err := environment.New(t.Context(),
		environment.WithEVMSimulatedN(t, 1),
	)
	require.NoError(t, err)

	csOutput := cldf.ChangesetOutput{}
	seqReport := operations.SequenceReport[string, map[uint64][]evmops.EVMCallOutput]{}
	mcmsCfg := &cldfproposalutils.TimelockConfig{}

	result, err := evmops.AddEVMCallSequenceToCSOutput(
		*env,
		csOutput,
		seqReport,
		nil,
		map[uint64]evmstate.MCMSWithTimelockState{},
		mcmsCfg,
		"test",
	)

	require.NoError(t, err)
	assert.Equal(t, seqReport.ExecutionReports, result.Reports)
	assert.Nil(t, result.MCMSTimelockProposals)
}

func TestAddEVMCallSequenceToCSOutput_ProposalCombination(t *testing.T) {
	t.Parallel()

	selector1 := chain_selectors.TEST_90000001.Selector
	selector2 := chain_selectors.TEST_90000002.Selector
	selectors := []uint64{selector1, selector2}

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, selectors),
	))
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(mcmschangesets.DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector1: cldftesthelpers.SingleGroupTimelockConfig(t),
			selector2: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	mcmsStateByChain := loadEVMMCMSStateByChain(t, rt.Environment(), selectors)

	existingProposal1 := mcmslib.TimelockProposal{
		BaseProposal: mcmslib.BaseProposal{Description: "First proposal"},
		Operations: []mcmstypes.BatchOperation{{
			ChainSelector: mcmstypes.ChainSelector(selector1),
			Transactions: []mcmstypes.Transaction{{
				To:               common.HexToAddress("0x1111111111111111111111111111111111111111").String(),
				Data:             []byte("data1"),
				AdditionalFields: json.RawMessage(`{"value": 0}`),
			}},
		}},
	}

	existingProposal2 := mcmslib.TimelockProposal{
		BaseProposal: mcmslib.BaseProposal{Description: "Second proposal"},
		Operations: []mcmstypes.BatchOperation{{
			ChainSelector: mcmstypes.ChainSelector(selector2),
			Transactions: []mcmstypes.Transaction{{
				To:               common.HexToAddress("0x1111112222222222222222222222222222222222").String(),
				Data:             []byte("data2"),
				AdditionalFields: json.RawMessage(`{"value": 0}`),
			}},
		}},
	}

	csOutput := cldf.ChangesetOutput{
		MCMSTimelockProposals: []mcmslib.TimelockProposal{existingProposal1, existingProposal2},
	}

	seqReport := operations.SequenceReport[string, map[uint64][]evmops.EVMCallOutput]{
		Report: operations.Report[string, map[uint64][]evmops.EVMCallOutput]{
			Output: map[uint64][]evmops.EVMCallOutput{
				selector2: {{
					To:           common.HexToAddress("0x3333333333333333333333333333333333333333"),
					Data:         []byte("new_call_data"),
					ContractType: "TestContract",
					Confirmed:    false,
				}},
			},
		},
	}

	mcmsCfg := &cldfproposalutils.TimelockConfig{
		MinDelay:   0 * time.Second,
		MCMSAction: mcmstypes.TimelockActionSchedule,
	}

	result, err := evmops.AddEVMCallSequenceToCSOutput(
		rt.Environment(),
		csOutput,
		seqReport,
		nil,
		mcmsStateByChain,
		mcmsCfg,
		"Third proposal",
	)
	require.NoError(t, err)
	assert.Equal(t, seqReport.ExecutionReports, result.Reports)

	require.Len(t, result.MCMSTimelockProposals, 1)
	aggregatedProposal := result.MCMSTimelockProposals[0]
	assert.Equal(t, "First proposal, Second proposal, Third proposal", aggregatedProposal.Description)
	assert.NotEmpty(t, aggregatedProposal.Operations)
}

func loadEVMMCMSStateByChain(t *testing.T, env cldf.Environment, selectors []uint64) map[uint64]evmstate.MCMSWithTimelockState {
	t.Helper()

	statePtrs, err := evmstate.MaybeLoadMCMSWithTimelockState(env, selectors)
	require.NoError(t, err)

	out := make(map[uint64]evmstate.MCMSWithTimelockState, len(statePtrs))
	for sel, st := range statePtrs {
		require.NotNilf(t, st, "expected non-nil MCMS state for chain %d", sel)
		require.NoErrorf(t, st.Validate(), "MCMS state for chain %d failed validation", sel)
		out[sel] = *st
	}

	return out
}
