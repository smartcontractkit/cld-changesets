package transfertomcms

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

// Registration describes one chain family's transfer-to-MCMS implementation.
type Registration struct {
	Family string
	// Sequence executes the per-chain transfer-to-MCMS operations sequence.
	Sequence *Sequence
	// Verify performs family-specific validation across all chains in the input.
	// It is called during VerifyPreconditions. Optional — nil means no extra checks.
	Verify func(env cldf.Environment, chains []ChainInput) error
}

var (
	sequenceRegistryMu sync.RWMutex
	sequenceRegistry   = make(map[string]Registration)
)

// Register adds a family transfer-to-MCMS sequence. Panics on invalid input or duplicate registration.
func Register(reg Registration) {
	if reg.Family == "" {
		panic("mcms transfer-to-mcms: family is required")
	}
	if reg.Sequence == nil {
		panic(fmt.Sprintf("mcms transfer-to-mcms: sequence is required for family %q", reg.Family))
	}

	sequenceRegistryMu.Lock()
	defer sequenceRegistryMu.Unlock()

	if _, exists := sequenceRegistry[reg.Family]; exists {
		panic(fmt.Sprintf("mcms transfer-to-mcms: family %q already registered", reg.Family))
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
	family, err := chainselectors.GetSelectorFamily(chainSelector)
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
			"mcms transfer-to-mcms: no sequence registered for family %q (none registered — blank-import mcms/<family>/transfer-to-mcms or mcms/changesets/transfer-to-mcms/all)",
			family,
		)
	}

	return Registration{}, fmt.Errorf(
		"mcms transfer-to-mcms: no sequence registered for family %q (registered: %s)",
		family,
		strings.Join(registered, ", "),
	)
}
