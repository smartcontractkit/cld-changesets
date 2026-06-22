package gasboost

import (
	cldfevm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// ToContractConfig converts proposal gas-boost settings to framework contract gas-boost config.
func ToContractConfig(cfg *cldfproposalutils.GasBoostConfig) *opscontract.GasBoostConfig {
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

// RetryDeploy wraps framework deploy gas-boost retry using proposal gas-boost config.
func RetryDeploy[ARGS any](cfg *cldfproposalutils.GasBoostConfig) operations.ExecuteOption[opscontract.DeployInput[ARGS], cldfevm.Chain] {
	return opscontract.RetryDeployWithGasBoost[ARGS](ToContractConfig(cfg))
}

// RetryWrite wraps framework write gas-boost retry using proposal gas-boost config.
func RetryWrite[ARGS any](cfg *cldfproposalutils.GasBoostConfig) operations.ExecuteOption[opscontract.FunctionInput[ARGS], cldfevm.Chain] {
	return opscontract.RetryWriteWithGasBoost[ARGS](ToContractConfig(cfg))
}

// RetryWithGasBoost wraps framework gas-boost retry for custom GasOverridable inputs.
func RetryWithGasBoost[IN opscontract.GasOverridable[IN]](cfg *cldfproposalutils.GasBoostConfig) operations.ExecuteOption[IN, cldfevm.Chain] {
	return opscontract.RetryWithGasBoost[IN](ToContractConfig(cfg))
}

func uint64PtrIfNonZero(v uint64) *uint64 {
	if v == 0 {
		return nil
	}

	return &v
}
