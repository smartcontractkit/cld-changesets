package setconfig

import (
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/mcms/changesets/familyregistry"
)

var sequences = familyregistry.New[Sequence, ChainInput](familyregistry.Options{
	Name:               "set-config",
	NoneRegisteredHint: "blank-import mcms/<family>/set-config or mcms/changesets/set-config/all",
})

// Register adds a family set-config sequence. Panics on invalid input or duplicate registration.
func Register(reg Registration) {
	sequences.Register(reg)
}

// RegisteredFamilies returns the sorted list of families with a registered sequence.
func RegisteredFamilies() []string {
	return sequences.RegisteredFamilies()
}

// SequenceForChainSelector returns the registered sequence for a chain selector.
func SequenceForChainSelector(chainSelector uint64) (*Sequence, error) {
	return sequences.SequenceForChainSelector(chainSelector)
}

// SequenceForFamily returns the registered sequence for a chain family.
func SequenceForFamily(family string) (*Sequence, error) {
	return sequences.SequenceForFamily(family)
}

// VerifyForFamily runs the registered family verify hook, if any.
func VerifyForFamily(family string, env cldf.Environment, chains []ChainInput) error {
	return sequences.VerifyForFamily(family, env, chains)
}
