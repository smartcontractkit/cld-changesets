package firedrill

import (
	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

// Registration describes one chain family's MCMS fire-drill implementation.
type Registration = familyregistry.Registration[ChainInput, *Sequence]

// Registry holds registered per-family fire-drill sequences.
var Registry = familyregistry.New[ChainInput, *Sequence]("mcms firedrill")
