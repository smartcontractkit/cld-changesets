package evmsetconfig

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
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
