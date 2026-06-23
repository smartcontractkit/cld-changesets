package deploycustomtopology

import (
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/familyregistry"
)

// Sequences is the deploy-custom-topology family sequence registry.
var Sequences = familyregistry.New[Sequence, ChainInput](familyregistry.Options{
	Name:               "deploy-custom-topology",
	NoneRegisteredHint: "blank-import mcms/<family>/deploy-custom-topology or mcms/changesets/deploy-custom-topology/all",
})
