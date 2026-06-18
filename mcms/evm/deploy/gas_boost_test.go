package evmdeploy

import (
	"testing"

	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/stretchr/testify/require"
)

func TestToDeployGasBoost(t *testing.T) {
	t.Parallel()

	require.Nil(t, toDeployGasBoost(nil))

	cfg := &cldfproposalutils.GasBoostConfig{
		InitialGasLimit:   1_000_000,
		GasLimitIncrement: 50_000,
		InitialGasPrice:   5_000_000_000,
		GasPriceIncrement: 1_000_000_000,
	}
	got := toDeployGasBoost(cfg)
	require.NotNil(t, got)
	require.Equal(t, uint64(1_000_000), got.InitialGasLimit)
	require.Equal(t, uint64(50_000), *got.GasLimitIncrement)
	require.Equal(t, uint64(5_000_000_000), got.InitialGasPrice)
	require.Equal(t, uint64(1_000_000_000), *got.GasPriceIncrement)
}

func TestUint64PtrIfNonZero(t *testing.T) {
	t.Parallel()

	require.Nil(t, uint64PtrIfNonZero(0))
	v := uint64PtrIfNonZero(42)
	require.NotNil(t, v)
	require.Equal(t, uint64(42), *v)
}
