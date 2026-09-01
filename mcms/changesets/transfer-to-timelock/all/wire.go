// Package all blank-imports built-in MCMS transfer-to-timelock families and readers.
package all

import (
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/transfer-to-timelock"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/transfer-to-timelock"
	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/transfer-to-timelock"
)
