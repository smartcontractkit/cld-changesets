package setconfig

import (
	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

// Registration describes one chain family's MCMS set-config implementation.
type Registration = familyregistry.Registration[Sequence, ChainInput]

// Registry holds per-family set-config sequences.
var Registry = familyregistry.New[Sequence, ChainInput]("mcms set-config")
