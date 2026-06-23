package evmownership

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"

	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
)

// Ownable is the minimal interface for an OpenZeppelin Ownable2Step contract used
// by the transfer/accept-ownership operations.
type Ownable = evmstate.Ownable

type OpOwnershipDeps struct {
	Chain    cldf_evm.Chain
	OwnableC Ownable
}

type OpTransferOwnershipInput struct {
	ChainSelector   uint64         `json:"chainSelector"`   // Chain selector for the EVM chain
	TimelockAddress common.Address `json:"timelockAddress"` // Address of the EVM Timelock contract
	Address         common.Address `json:"address"`         // Address of the contract for which ownership is being transferred
}

type OpOwnershipOutput struct {
	Tx *gethtypes.Transaction `json:"tx"`
}

// OpTransferOwnership transfers ownership of an ownable contract to the timelock,
// sending the transaction with the deployer key.
var OpTransferOwnership = operations.NewOperation(
	"evm-transfer-ownership",
	semver.MustParse("1.0.0"),
	"Transfer ownership of an ownable contract to the specified address",
	func(b operations.Bundle, deps OpOwnershipDeps, in OpTransferOwnershipInput) (OpOwnershipOutput, error) {
		tx, err := deps.OwnableC.TransferOwnership(deps.Chain.DeployerKey, common.HexToAddress(in.TimelockAddress.Hex()))
		_, err = cldf.ConfirmIfNoError(deps.Chain, tx, err)
		if err != nil {
			return OpOwnershipOutput{Tx: tx}, fmt.Errorf(
				"failed to transfer ownership of contract %s: %w",
				in.Address.Hex(),
				err,
			)
		}

		return OpOwnershipOutput{
			Tx: tx,
		}, nil
	})

// OpAcceptOwnership simulates the acceptOwnership() call (no send) so the resulting
// calldata can be routed through the timelock as an MCMS transaction.
var OpAcceptOwnership = operations.NewOperation(
	"evm-accept-ownership",
	semver.MustParse("1.0.0"),
	"Accepts ownership of an ownable contract via the Timelock contract",
	func(b operations.Bundle, deps OpOwnershipDeps, in OpTransferOwnershipInput) (OpOwnershipOutput, error) {
		tx, err := deps.OwnableC.AcceptOwnership(cldf.SimTransactOpts())
		if err != nil {
			return OpOwnershipOutput{Tx: tx}, fmt.Errorf(
				"failed to accept ownership of contract %s: %w",
				in.Address.Hex(),
				err,
			)
		}

		return OpOwnershipOutput{
			Tx: tx,
		}, nil
	})
