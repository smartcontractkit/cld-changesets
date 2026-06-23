package operations

import (
	evmtransferownership "github.com/smartcontractkit/cld-changesets/mcms/evm/transfer-ownership"
)

type OpOwnershipDeps = evmtransferownership.OpOwnershipDeps
type OpTransferOwnershipInput = evmtransferownership.OpTransferOwnershipInput
type OpOwnershipOutput = evmtransferownership.OpOwnershipOutput

var OpTransferOwnership = evmtransferownership.OpTransferOwnership
var OpAcceptOwnership = evmtransferownership.OpAcceptOwnership
