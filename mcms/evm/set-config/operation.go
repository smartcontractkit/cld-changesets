package evmsetconfig

import (
	"fmt"
	"math/big"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmsevm "github.com/smartcontractkit/mcms/sdk/evm"
	mcmsbindings "github.com/smartcontractkit/mcms/sdk/evm/bindings"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/mcms/evm/internal/gasboost"
)

// MCMSetConfigTarget identifies one MCM contract and the config to apply.
type MCMSetConfigTarget struct {
	Address      common.Address    `json:"address"`
	Config       mcmstypes.Config  `json:"config"`
	ContractType cldf.ContractType `json:"contractType"`
}

// OpEVMSetConfigInput is the input for setting config on a single EVM MCM contract.
type OpEVMSetConfigInput struct {
	Target   MCMSetConfigTarget `json:"target"`
	NoSend   bool               `json:"noSend"`
	GasPrice uint64             `json:"gasPrice"`
	GasLimit uint64             `json:"gasLimit"`
}

func (in OpEVMSetConfigInput) GasBoostValues() (gasLimit, gasPrice uint64) {
	return in.GasLimit, in.GasPrice
}

func (in OpEVMSetConfigInput) WithGasBoost(gasLimit, gasPrice uint64) OpEVMSetConfigInput {
	in.GasLimit = gasLimit
	in.GasPrice = gasPrice

	return in
}

// OpEVMSetConfigMCM sets MCMS config on an EVM MCM contract via the MCMS SDK configurer.
var OpEVMSetConfigMCM = operations.NewOperation(
	"evm-mcm-set-config",
	semver.MustParse("1.0.0"),
	"Sets MCMS config on an EVM MCM contract",
	func(b operations.Bundle, deps cldf_evm.Chain, in OpEVMSetConfigInput) (opscontract.WriteOutput, error) {
		if !in.NoSend && deps.DeployerKey == nil {
			return opscontract.WriteOutput{}, fmt.Errorf("missing deployer key for chain %d", deps.Selector)
		}

		var opts *bind.TransactOpts
		if in.NoSend {
			opts = cldf.SimTransactOpts()
		} else {
			opts = gasboost.CloneTransactOptsWithGas(deps.DeployerKey, in.GasLimit, in.GasPrice)
		}
		if opts == nil {
			return opscontract.WriteOutput{}, fmt.Errorf("failed to build transact opts for chain %d", deps.Selector)
		}
		opts.Context = b.GetContext()

		configurer := mcmsevm.NewConfigurer(deps.Client, opts)
		res, err := configurer.SetConfig(b.GetContext(), in.Target.Address.Hex(), &in.Target.Config, false)
		if err != nil {
			return opscontract.WriteOutput{}, fmt.Errorf("failed to set config on %s: %w", in.Target.Address, err)
		}

		tx, ok := res.RawData.(*types.Transaction)
		if !ok {
			return opscontract.WriteOutput{}, fmt.Errorf("unexpected raw data type %T from SetConfig", res.RawData)
		}

		out := writeOutputFromSetConfig(deps.Selector, in.Target, tx)
		if in.NoSend {
			return out, nil
		}

		if _, err = cldf.ConfirmIfNoErrorWithABI(deps, tx, mcmsbindings.ManyChainMultiSigABI, err); err != nil {
			return opscontract.WriteOutput{}, fmt.Errorf("failed to confirm set config tx against %s: %w", in.Target.Address, err)
		}
		b.Logger.Infow("SetConfig tx confirmed", "txHash", res.Hash, "address", in.Target.Address.Hex())

		out.ExecInfo = &opscontract.ExecInfo{Hash: res.Hash}

		return out, nil
	},
)

func writeOutputFromSetConfig(chainSelector uint64, target MCMSetConfigTarget, tx *types.Transaction) opscontract.WriteOutput {
	return opscontract.WriteOutput{
		ChainSelector: chainSelector,
		Tx: mcmsevm.NewTransaction(
			target.Address,
			tx.Data(),
			big.NewInt(0),
			string(target.ContractType),
			[]string{},
		),
	}
}

func retrySetConfigWithGasBoost(cfg *cldfproposalutils.GasBoostConfig) operations.ExecuteOption[OpEVMSetConfigInput, cldf_evm.Chain] {
	if cfg == nil {
		return operations.WithRetry[OpEVMSetConfigInput, cldf_evm.Chain]()
	}

	return gasboost.RetryWithGasBoost[OpEVMSetConfigInput](cfg)
}
