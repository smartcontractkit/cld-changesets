package deploycustomtopology

import (
	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

// Registration describes one chain family's deploy-custom-topology implementation.
type Registration = familyregistry.Registration[Sequence, ChainInput]

// Registry holds per-family deploy-custom-topology sequences.
var Registry = familyregistry.New[Sequence, ChainInput]("mcms deploy-custom-topology")
