package evmdeploy

import (
	cldfevm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

func toDeployGasBoost(cfg *cldfproposalutils.GasBoostConfig) *opscontract.GasBoostConfig {
	if cfg == nil {
		return nil
	}

	return &opscontract.GasBoostConfig{
		InitialGasLimit:   cfg.InitialGasLimit,
		GasLimitIncrement: uint64PtrIfNonZero(cfg.GasLimitIncrement),
		InitialGasPrice:   cfg.InitialGasPrice,
		GasPriceIncrement: uint64PtrIfNonZero(cfg.GasPriceIncrement),
	}
}

func uint64PtrIfNonZero(v uint64) *uint64 {
	if v == 0 {
		return nil
	}

	return &v
}

func retryDeployWithGasBoost[ARGS any](cfg *cldfproposalutils.GasBoostConfig) operations.ExecuteOption[opscontract.DeployInput[ARGS], cldfevm.Chain] {
	return RetryDeployWithGasBoost[ARGS](cfg)
}

// RetryDeployWithGasBoost wraps framework deploy gas-boost retry for MCMS EVM operations.
func RetryDeployWithGasBoost[ARGS any](cfg *cldfproposalutils.GasBoostConfig) operations.ExecuteOption[opscontract.DeployInput[ARGS], cldfevm.Chain] {
	return opscontract.RetryDeployWithGasBoost[ARGS](toDeployGasBoost(cfg))
}

func retryWriteWithGasBoost[ARGS any](cfg *cldfproposalutils.GasBoostConfig) operations.ExecuteOption[opscontract.FunctionInput[ARGS], cldfevm.Chain] {
	return RetryWriteWithGasBoost[ARGS](cfg)
}

// RetryWriteWithGasBoost wraps framework write gas-boost retry for MCMS EVM operations.
func RetryWriteWithGasBoost[ARGS any](cfg *cldfproposalutils.GasBoostConfig) operations.ExecuteOption[opscontract.FunctionInput[ARGS], cldfevm.Chain] {
	return opscontract.RetryWriteWithGasBoost[ARGS](toDeployGasBoost(cfg))
}
