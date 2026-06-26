package gasboost

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/stretchr/testify/require"
)

func TestCloneTransactOptsWithGas(t *testing.T) {
	t.Parallel()

	require.Nil(t, CloneTransactOptsWithGas(nil, 100, 200))

	opts := &bind.TransactOpts{GasLimit: 1, GasPrice: big.NewInt(1)}
	got := CloneTransactOptsWithGas(opts, 0, 0)
	require.Equal(t, uint64(1), got.GasLimit)
	require.Equal(t, int64(1), got.GasPrice.Int64())
	require.NotSame(t, opts, got)

	got = CloneTransactOptsWithGas(opts, 500_000, 30_000_000_000)
	require.Equal(t, uint64(500_000), got.GasLimit)
	require.Equal(t, uint64(30_000_000_000), got.GasPrice.Uint64())
}

func TestToContractConfig(t *testing.T) {
	t.Parallel()

	require.Nil(t, ToContractConfig(nil))

	cfg := &cldfproposalutils.GasBoostConfig{
		InitialGasLimit:   1_000_000,
		GasLimitIncrement: 50_000,
		InitialGasPrice:   5_000_000_000,
		GasPriceIncrement: 1_000_000_000,
	}
	got := ToContractConfig(cfg)
	require.NotNil(t, got)
	require.Equal(t, uint64(1_000_000), got.InitialGasLimit)
	require.Equal(t, uint64(50_000), *got.GasLimitIncrement)
	require.Equal(t, uint64(5_000_000_000), got.InitialGasPrice)
	require.Equal(t, uint64(1_000_000_000), *got.GasPriceIncrement)
}

func TestToContractConfig_zeroIncrements(t *testing.T) {
	t.Parallel()

	got := ToContractConfig(&cldfproposalutils.GasBoostConfig{
		InitialGasLimit:   1_000_000,
		GasLimitIncrement: 0,
		InitialGasPrice:   5_000_000_000,
		GasPriceIncrement: 0,
	})
	require.NotNil(t, got)
	require.Nil(t, got.GasLimitIncrement)
	require.Nil(t, got.GasPriceIncrement)
}

func TestRetryOptions(t *testing.T) {
	t.Parallel()

	cfg := &cldfproposalutils.GasBoostConfig{
		InitialGasLimit:   100_000,
		GasLimitIncrement: 10_000,
		InitialGasPrice:   1_000,
		GasPriceIncrement: 500,
	}

	require.NotNil(t, RetryDeploy[struct{}](nil))
	require.NotNil(t, RetryDeploy[struct{}](cfg))
	require.NotNil(t, RetryWrite[struct{}](nil))
	require.NotNil(t, RetryWrite[struct{}](cfg))
	require.NotNil(t, RetryWithGasBoost[retryInput](nil))
	require.NotNil(t, RetryWithGasBoost[retryInput](cfg))
}

type retryInput struct {
	GasLimit uint64
	GasPrice uint64
}

func (in retryInput) GasBoostValues() (gasLimit, gasPrice uint64) {
	return in.GasLimit, in.GasPrice
}

func (in retryInput) WithGasBoost(gasLimit, gasPrice uint64) retryInput {
	in.GasLimit = gasLimit
	in.GasPrice = gasPrice

	return in
}
