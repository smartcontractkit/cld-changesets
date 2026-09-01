// Package all blank-imports built-in MCMS set-config families and MCMS readers.
// Use it when a pipeline needs every supported chain family. For single-family
// pipelines, blank-import mcms/<family>/set-config and mcms/<family>/readers instead.
package all

import (
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/set-config"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/set-config"
	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/set-config"
)
