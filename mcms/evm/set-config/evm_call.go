package evmsetconfig

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
)

// EVMCallOutput is the output structure for an EVM set-config operation.
type EVMCallOutput struct {
	To           common.Address    `json:"to"`
	Data         []byte            `json:"data"`
	ContractType cldf.ContractType `json:"contractType"`
	Confirmed    bool              `json:"confirmed"`
}

func cloneTransactOptsWithGas(opts *bind.TransactOpts, gasLimit uint64, gasPrice uint64) *bind.TransactOpts {
	if opts == nil {
		return nil
	}
	newOpts := *opts
	if gasLimit > 0 {
		newOpts.GasLimit = gasLimit
	}
	if gasPrice > 0 {
		newOpts.GasPrice = new(big.Int).SetUint64(gasPrice)
	}

	return &newOpts
}

func getBoostedGasForAttempt(cfg cldfproposalutils.GasBoostConfig, attempt uint) (gasLimit uint64, gasPrice uint64) {
	initialGasLimit := uint64(200_000)
	gasLimitIncrement := uint64(50_000)
	initialGasPrice := uint64(20_000_000_000)
	gasPriceIncrement := uint64(10_000_000_000)

	if cfg.InitialGasLimit > 0 {
		initialGasLimit = cfg.InitialGasLimit
	}
	if cfg.GasLimitIncrement > 0 {
		gasLimitIncrement = cfg.GasLimitIncrement
	}
	if cfg.InitialGasPrice > 0 {
		initialGasPrice = cfg.InitialGasPrice
	}
	if cfg.GasPriceIncrement > 0 {
		gasPriceIncrement = cfg.GasPriceIncrement
	}

	gasLimit = initialGasLimit + uint64(attempt)*gasLimitIncrement
	gasPrice = initialGasPrice + uint64(attempt)*gasPriceIncrement

	return
}
