package firedrill

import (
	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

// Registration describes one chain family's MCMS fire-drill implementation.
type Registration = familyregistry.Registration[Sequence, ChainInput]

// Registry holds per-family fire-drill sequences.
var Registry = familyregistry.New[Sequence, ChainInput]("mcms firedrill")
