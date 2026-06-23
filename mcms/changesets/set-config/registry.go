package setconfig

import (
	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

// Registration describes one chain family's MCMS set-config implementation.
type Registration = familyregistry.Registration[ChainInput, *Sequence]

// Registry holds registered per-family set-config sequences.
var Registry = familyregistry.New[ChainInput, *Sequence]("mcms set-config")
