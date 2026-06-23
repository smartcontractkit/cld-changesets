// Package evmtransferownership houses the EVM operations for transferring and
// accepting ownership of an ownable contract via the Timelock. It lives
// alongside where its dedicated changeset would (once it is added) so all
// related code is colocated.
package evmtransferownership

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/cld-changesets/internal/evmownership"
)

type Ownable = evmownership.Ownable
type OpOwnershipDeps = evmownership.OpOwnershipDeps
type OpTransferOwnershipInput = evmownership.OpTransferOwnershipInput
type OpOwnershipOutput = evmownership.OpOwnershipOutput

var OpTransferOwnership = evmownership.OpTransferOwnership
var OpAcceptOwnership = evmownership.OpAcceptOwnership

func LoadOwnable(addr common.Address, backend bind.ContractBackend) (common.Address, Ownable, error) {
	return evmownership.LoadOwnable(addr, backend)
}

func NewOwnable2Step(addr common.Address, backend bind.ContractBackend) Ownable {
	return evmownership.NewOwnable2Step(addr, backend)
}
