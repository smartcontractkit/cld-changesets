package firedrill

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

// Registration describes one chain family's MCMS fire-drill implementation.
type Registration struct {
	Family string
	// Sequence executes the per-chain fire-drill operations sequence.
	Sequence *Sequence
	// Verify performs family-specific validation across all chains in the input.
	// It is called during VerifyPreconditions. Optional — nil means no extra checks.
	Verify func(env cldf.Environment, chains []ChainInput) error
}

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Registration)
)

// Register adds a family fire-drill sequence. Panics on invalid input or duplicate registration.
func Register(reg Registration) {
	if reg.Family == "" {
		panic("mcms firedrill: family is required")
	}
	if reg.Sequence == nil {
		panic(fmt.Sprintf("mcms firedrill: sequence is required for family %q", reg.Family))
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	if _, exists := registry[reg.Family]; exists {
		panic(fmt.Sprintf("mcms firedrill: family %q already registered", reg.Family))
	}

	registry[reg.Family] = reg
}

// RegisteredFamilies returns the sorted list of families with a registered sequence.
func RegisteredFamilies() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	families := make([]string, 0, len(registry))
	for family := range registry {
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

func verifyForFamily(family string, env cldf.Environment, chains []ChainInput) error {
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
	registryMu.RLock()
	reg, ok := registry[family]
	registryMu.RUnlock()

	if ok {
		return reg, nil
	}

	registered := RegisteredFamilies()
	if len(registered) == 0 {
		return Registration{}, fmt.Errorf(
			"mcms firedrill: no sequence registered for family %q (none registered — blank-import mcms/<family>/firedrill or mcms/changesets/firedrill/all)",
			family,
		)
	}

	return Registration{}, fmt.Errorf(
		"mcms firedrill: no sequence registered for family %q (registered: %s)",
		family,
		strings.Join(registered, ", "),
	)
}
