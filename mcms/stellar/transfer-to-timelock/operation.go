package stellartransfertotimelock

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	mcmsstellar "github.com/smartcontractkit/mcms/sdk/stellar"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
)

const acceptOwnershipFunction = "accept_ownership"

// OwnableContract identifies a Stellar MCMS contract to transfer.
type OwnableContract struct {
	Address string
	Type    cldf.ContractType
}

// OpTransferToTimelockInput is the input for transferring one Stellar MCMS contract to the timelock.
type OpTransferToTimelockInput struct {
	Contract            OwnableContract
	Timelock            string
	OnlyAcceptOwnership bool
}

// OpTransferToTimelockOutput is the output of a single contract transfer operation.
type OpTransferToTimelockOutput struct {
	BatchOps []mcmstypes.BatchOperation
}

// OpTransferToTimelock transfers one ownable Stellar MCMS contract to the timelock.
var OpTransferToTimelock = operations.NewOperation(
	"stellar-transfer-to-timelock",
	semver.MustParse("1.0.0"),
	"Transfer ownable Stellar MCMS contract ownership to the MCMS timelock",
	func(b operations.Bundle, chain cldfstellar.Chain, in OpTransferToTimelockInput) (OpTransferToTimelockOutput, error) {
		var out OpTransferToTimelockOutput

		if !mcmsstellar.IsAddress(in.Contract.Address) {
			return out, fmt.Errorf("invalid Stellar contract address %q", in.Contract.Address)
		}
		if !mcmsstellar.IsAddress(in.Timelock) {
			return out, fmt.Errorf("invalid Stellar timelock address %q", in.Timelock)
		}

		deployer, err := stellardeployment.NewDeployerFromChain(chain)
		if err != nil {
			return out, fmt.Errorf("create Stellar deployer for chain %d: %w", chain.Selector, err)
		}

		inspector := mcmsstellar.NewInspectorFromInvoker(deployer)
		owner, err := inspector.GetOwner(b.GetContext(), in.Contract.Address)
		if err != nil {
			return out, fmt.Errorf("read owner for %s: %w", in.Contract.Address, err)
		}
		if owner == nil {
			return out, fmt.Errorf("contract %s has no owner", in.Contract.Address)
		}
		if *owner == in.Timelock {
			b.Logger.Infof("contract %s already owned by timelock", in.Contract.Address)
			return out, nil
		}

		pendingOwner, err := inspector.GetPendingOwner(b.GetContext(), in.Contract.Address)
		if err != nil {
			return out, fmt.Errorf("read pending owner for %s: %w", in.Contract.Address, err)
		}
		if pendingOwner != nil && *pendingOwner != in.Timelock {
			return out, fmt.Errorf(
				"contract %s has unexpected pending owner %s",
				in.Contract.Address,
				*pendingOwner,
			)
		}

		if !in.OnlyAcceptOwnership {
			if *owner != deployer.SignerAddress() {
				return out, fmt.Errorf(
					"contract %s owner %s does not match deployer %s",
					in.Contract.Address,
					*owner,
					deployer.SignerAddress(),
				)
			}

			if pendingOwner == nil {
				if _, err = mcmsstellar.NewConfigurer(deployer).TransferOwnership(
					b.GetContext(),
					in.Contract.Address,
					in.Timelock,
				); err != nil {
					return out, fmt.Errorf("transfer ownership of %s to timelock: %w", in.Contract.Address, err)
				}

				pendingOwner, err = inspector.GetPendingOwner(b.GetContext(), in.Contract.Address)
				if err != nil {
					return out, fmt.Errorf("read pending owner for %s after transfer: %w", in.Contract.Address, err)
				}
				if pendingOwner == nil || *pendingOwner != in.Timelock {
					return out, fmt.Errorf("contract %s ownership transfer to timelock was not recorded", in.Contract.Address)
				}
			}
		}

		if pendingOwner == nil {
			return out, fmt.Errorf(
				"contract %s has no pending owner; transfer ownership before accepting",
				in.Contract.Address,
			)
		}

		acceptOwnership, err := mcmsstellar.NewBatchOperation(
			mcmstypes.ChainSelector(chain.Selector),
			in.Contract.Address,
			acceptOwnershipFunction,
			nil,
			string(in.Contract.Type),
			nil,
		)
		if err != nil {
			return out, fmt.Errorf("create accept ownership MCMS operation for %s: %w", in.Contract.Address, err)
		}

		out.BatchOps = append(out.BatchOps, acceptOwnership)

		return out, nil
	},
)
