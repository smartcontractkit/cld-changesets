package deploycustomtopology

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

var (
	sequenceRegistryMu sync.RWMutex
	sequenceRegistry   = make(map[string]Registration)
)

// Register adds a family deploy-custom-topology sequence. Panics on invalid
// input or duplicate registration.
func Register(reg Registration) {
	if reg.Family == "" {
		panic("mcms deploy-custom-topology: family is required")
	}
	if reg.Sequence == nil {
		panic(fmt.Sprintf("mcms deploy-custom-topology: sequence is required for family %q", reg.Family))
	}

	sequenceRegistryMu.Lock()
	defer sequenceRegistryMu.Unlock()

	if _, exists := sequenceRegistry[reg.Family]; exists {
		panic(fmt.Sprintf("mcms deploy-custom-topology: family %q already registered", reg.Family))
	}

	sequenceRegistry[reg.Family] = reg
}

// RegisteredFamilies returns the sorted list of families with a registered sequence.
func RegisteredFamilies() []string {
	sequenceRegistryMu.RLock()
	defer sequenceRegistryMu.RUnlock()

	families := make([]string, 0, len(sequenceRegistry))
	for family := range sequenceRegistry {
		families = append(families, family)
	}
	slices.Sort(families)

	return families
}

// SequenceForChainSelector returns the registered sequence for a chain selector.
func SequenceForChainSelector(chainSelector uint64) (*Sequence, error) {
	family, err := chain_selectors.GetSelectorFamily(chainSelector)
	if err != nil {
		return nil, err
	}

	return SequenceForFamily(family)
}

// SequenceForFamily returns the registered sequence for a chain family.
func SequenceForFamily(family string) (*Sequence, error) {
	reg, err := registrationForFamily(family)
	if err != nil {
		return nil, err
	}

	return reg.Sequence, nil
}

// VerifyForFamily runs the registered family verify hook, if any.
func VerifyForFamily(family string, env cldf.Environment, chains []ChainInput) error {
	reg, err := registrationForFamily(family)
	if err != nil {
		return err
	}
	if reg.Verify == nil {
		return nil
	}
	if err := reg.Verify(env, chains); err != nil {
		return fmt.Errorf("family %s: %w", family, err)
	}

	return nil
}

func registrationForFamily(family string) (Registration, error) {
	sequenceRegistryMu.RLock()
	reg, ok := sequenceRegistry[family]
	sequenceRegistryMu.RUnlock()

	if ok {
		return reg, nil
	}

	registered := RegisteredFamilies()
	if len(registered) == 0 {
		return Registration{}, fmt.Errorf(
			"mcms deploy-custom-topology: no sequence registered for family %q (none registered — blank-import mcms/<family>/deploy-custom-topology or mcms/changesets/deploy-custom-topology/all)",
			family,
		)
	}

	return Registration{}, fmt.Errorf(
		"mcms deploy-custom-topology: no sequence registered for family %q (registered: %s)",
		family,
		strings.Join(registered, ", "),
	)
}
