package operations

import (
	opevm "github.com/smartcontractkit/cld-changesets/legacy/mcms/internal/family/evm/operations"
)

// OpEVMTransferOwnership transfers ownership of an EVM contract.
//
// Re-exported from `legacy/mcms/internal` for backward compatibility with
// external repositories. Do not add new callers.
var OpEVMTransferOwnership = opevm.OpTransferOwnership

// OpEVMAcceptOwnership accepts ownership of an EVM contract.
//
// Re-exported from `legacy/mcms/internal` for backward compatibility with
// external repositories. Do not add new callers.
var OpEVMAcceptOwnership = opevm.OpAcceptOwnership
