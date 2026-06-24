package transfertotimelock

import (
	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

// Registration describes one chain family's transfer-to-timelock implementation.
type Registration = familyregistry.Registration[Sequence, ChainInput]

// Registry holds per-family transfer-to-timelock sequences.
var Registry = familyregistry.New[Sequence, ChainInput]("mcms transfer-to-timelock")
