package operations

import (
	"github.com/smartcontractkit/cld-changesets/internal/evmownership"
)

type OpOwnershipDeps = evmownership.OpOwnershipDeps
type OpTransferOwnershipInput = evmownership.OpTransferOwnershipInput
type OpOwnershipOutput = evmownership.OpOwnershipOutput

var OpTransferOwnership = evmownership.OpTransferOwnership
var OpAcceptOwnership = evmownership.OpAcceptOwnership
