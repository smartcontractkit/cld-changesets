package evmsetconfig

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	mcmsevm "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

func TestOpEVMSetConfigMCM_missingDeployerKey(t *testing.T) {
	t.Parallel()

	_, err := operations.ExecuteOperation(
		optest.NewBundle(t),
		OpEVMSetConfigMCM,
		cldf_evm.Chain{Selector: chainselectors.TEST_90000001.Selector},
		OpEVMSetConfigInput{
			NoSend: false,
			Target: MCMSetConfigTarget{
				Address: common.Address{1},
			},
		},
	)
	require.ErrorContains(t, err, "missing deployer key")
}

func TestCloneTransactOptsWithGas(t *testing.T) {
	t.Parallel()

	require.Nil(t, cloneTransactOptsWithGas(nil, 100, 200))

	opts := &bind.TransactOpts{GasLimit: 1, GasPrice: big.NewInt(1)}
	got := cloneTransactOptsWithGas(opts, 0, 0)
	require.Equal(t, uint64(1), got.GasLimit)
	require.Equal(t, int64(1), got.GasPrice.Int64())
	require.NotSame(t, opts, got)

	got = cloneTransactOptsWithGas(opts, 500_000, 30_000_000_000)
	require.Equal(t, uint64(500_000), got.GasLimit)
	require.Equal(t, uint64(30_000_000_000), got.GasPrice.Uint64())
}

func TestGetBoostedGasForAttempt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       cldfproposalutils.GasBoostConfig
		attempt   uint
		wantLimit uint64
		wantPrice uint64
	}{
		{
			name:      "defaults attempt zero",
			attempt:   0,
			wantLimit: 200_000,
			wantPrice: 20_000_000_000,
		},
		{
			name:      "defaults attempt two",
			attempt:   2,
			wantLimit: 300_000,
			wantPrice: 40_000_000_000,
		},
		{
			name: "custom config",
			cfg: cldfproposalutils.GasBoostConfig{
				InitialGasLimit:   100_000,
				GasLimitIncrement: 10_000,
				InitialGasPrice:   1_000,
				GasPriceIncrement: 500,
			},
			attempt:   3,
			wantLimit: 130_000,
			wantPrice: 2_500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotLimit, gotPrice := getBoostedGasForAttempt(tt.cfg, tt.attempt)
			require.Equal(t, tt.wantLimit, gotLimit)
			require.Equal(t, tt.wantPrice, gotPrice)
		})
	}
}

func TestRetrySetConfigWithGasBoost(t *testing.T) {
	t.Parallel()

	require.NotNil(t, retrySetConfigWithGasBoost(nil))
	require.NotNil(t, retrySetConfigWithGasBoost(&cldfproposalutils.GasBoostConfig{
		InitialGasLimit:   100_000,
		GasLimitIncrement: 10_000,
		InitialGasPrice:   1_000,
		GasPriceIncrement: 500,
	}))
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
				transferEVMMCMSToTimelock(t, rt, selector, refs)
			}

			cfg := cldftesthelpers.SingleGroupMCMS(t)
			cfg.Signers = append(cfg.Signers, refs.Timelock)
			cfg.Quorum = 2

			report, err := operations.ExecuteOperation(
				rt.Environment().OperationsBundle,
				OpEVMSetConfigMCM,
				chain,
				OpEVMSetConfigInput{
					Target: MCMSetConfigTarget{
						Address:      refs.Canceller,
						Config:       cfg,
						ContractType: mcmscontracts.CancellerManyChainMultisig,
					},
					NoSend: tt.noSend,
				},
			)
			require.NoError(t, err)
			require.Equal(t, refs.Canceller, report.Output.To)
			require.NotEmpty(t, report.Output.Data)
			require.Equal(t, !tt.noSend, report.Output.Confirmed)

			if tt.noSend {
				batch, err := evmCallOutputsToBatch(selector, []EVMCallOutput{report.Output})
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
