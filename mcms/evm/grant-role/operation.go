package evmgrantrole

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmsevm "github.com/smartcontractkit/mcms/sdk/evm"
	mcmsbindings "github.com/smartcontractkit/mcms/sdk/evm/bindings"

	"github.com/smartcontractkit/cld-changesets/mcms/evm/internal/gasboost"
)

type GrantRoleTarget struct {
	Timelock common.Address       `json:"timelock"`
	Role     mcmssdk.TimelockRole `json:"role"`
	Address  common.Address       `json:"address"`
}

type OpEVMGrantRoleInput struct {
	Target   GrantRoleTarget `json:"target"`
	NoSend   bool            `json:"noSend"`
	GasPrice uint64          `json:"gasPrice"`
	GasLimit uint64          `json:"gasLimit"`
}

func (in OpEVMGrantRoleInput) GasBoostValues() (gasLimit, gasPrice uint64) {
	return in.GasLimit, in.GasPrice
}

func (in OpEVMGrantRoleInput) WithGasBoost(gasLimit, gasPrice uint64) OpEVMGrantRoleInput {
	in.GasLimit = gasLimit
	in.GasPrice = gasPrice

	return in
}

var OpEVMGrantRole = operations.NewOperation(
	"evm-timelock-grant-role",
	semver.MustParse("1.0.0"),
	"Grants an RBACTimelock role to one EVM address via the MCMS SDK timelock configurer",
	func(b operations.Bundle, deps cldf_evm.Chain, in OpEVMGrantRoleInput) (opscontract.WriteOutput, error) {
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

		configurer := mcmsevm.NewTimelockConfigurer(deps.Client, opts)
		res, err := configurer.GrantRole(b.GetContext(), in.Target.Timelock.Hex(), in.Target.Role, in.Target.Address.Hex())
		if err != nil {
			return opscontract.WriteOutput{}, fmt.Errorf("failed to grant role %s to %s on %s: %w",
				in.Target.Role.String(), in.Target.Address.Hex(), in.Target.Timelock.Hex(), err)
		}

		tx, err := rawTransaction(res.RawData)
		if err != nil {
			return opscontract.WriteOutput{}, err
		}

		out := writeOutputFromGrant(deps.Selector, in.Target.Timelock, tx)
		if in.NoSend {
			return out, nil
		}

		if _, err = cldf.ConfirmIfNoErrorWithABI(deps, tx, mcmsbindings.RBACTimelockABI, nil); err != nil {
			return opscontract.WriteOutput{}, fmt.Errorf("failed to confirm grant role tx against %s: %w", in.Target.Timelock.Hex(), err)
		}
		b.Logger.Infow("GrantRole tx confirmed", "txHash", tx.Hash().Hex(), "timelock", in.Target.Timelock.Hex())

		out.ExecInfo = &opscontract.ExecInfo{Hash: tx.Hash().Hex()}

		return out, nil
	},
)

func writeOutputFromGrant(chainSelector uint64, timelock common.Address, tx *types.Transaction) opscontract.WriteOutput {
	return opscontract.WriteOutput{
		ChainSelector: chainSelector,
		Tx: mcmsevm.NewTransaction(
			timelock,
			tx.Data(),
			big.NewInt(0),
			string(mcmscontracts.RBACTimelock),
			[]string{},
		),
	}
}

func rawTransaction(raw any) (*types.Transaction, error) {
	switch tx := raw.(type) {
	case *types.Transaction:
		return tx, nil
	default:
		return nil, fmt.Errorf("unexpected raw data type %T from GrantRole", raw)
	}
}

func retryGrantRoleWithGasBoost(cfg *cldfproposalutils.GasBoostConfig) operations.ExecuteOption[OpEVMGrantRoleInput, cldf_evm.Chain] {
	if cfg == nil {
		return operations.WithRetry[OpEVMGrantRoleInput, cldf_evm.Chain]()
	}

	return gasboost.RetryWithGasBoost[OpEVMGrantRoleInput](cfg)
}

func validateGrantRoleTarget(target GrantRoleTarget) error {
	if target.Timelock == (common.Address{}) {
		return errors.New("timelock address must not be zero")
	}
	if !target.Role.Valid() {
		return errors.New("role is unsupported")
	}
	if target.Address == (common.Address{}) {
		return errors.New("address must not be zero")
	}

	return nil
}
