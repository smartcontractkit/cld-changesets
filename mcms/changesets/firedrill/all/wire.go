// Package all blank-imports built-in MCMS fire-drill families and MCMS readers.
// Use it when a pipeline needs every supported chain family. For single-family
// pipelines, blank-import mcms/<family>/firedrill and mcms/<family>/readers instead.
package all

import (
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/firedrill"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/firedrill"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)
