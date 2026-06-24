// Package familyregistry provides a generic, thread-safe registry mapping a
// string key (typically a chain-selectors family) to a per-family implementation.
//
// Changeset packages use it to register family-specific operations sequences and
// optional verify hooks without duplicating registry boilerplate.
package familyregistry

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

// Registration describes one family's implementation, where S is the operations
// sequence type and C is the per-chain input type.
type Registration[S any, C any] struct {
	// Family is the chain-selectors family string (e.g. chainselectors.FamilyEVM).
	Family string
	// Sequence executes the per-chain operations sequence.
	Sequence *S
	// Verify performs family-specific validation across all chains in the input.
	// It is called during VerifyPreconditions. Optional — nil means no extra checks.
	Verify func(env cldf.Environment, chains []C) error
}

// Registry is a thread-safe map of family to Registration.
type Registry[S any, C any] struct {
	label   string
	mu      sync.RWMutex
	entries map[string]Registration[S, C]
}

// New returns an empty registry. label prefixes panic and error messages.
func New[S any, C any](label string) *Registry[S, C] {
	return &Registry[S, C]{
		label:   label,
		entries: make(map[string]Registration[S, C]),
	}
}

// Register adds a family implementation. It panics if the family string is
// empty, Sequence is nil, or the family is already registered — all of which
// indicate a programming error at startup.
func (r *Registry[S, C]) Register(reg Registration[S, C]) {
	if reg.Family == "" {
		panic(r.label + ": family is required")
	}
	if reg.Sequence == nil {
		panic(fmt.Sprintf("%s: sequence is required for family %q", r.label, reg.Family))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[reg.Family]; exists {
		panic(fmt.Sprintf("%s: family %q already registered", r.label, reg.Family))
	}

	r.entries[reg.Family] = reg
}

// RegisteredFamilies returns the sorted list of families with a registration.
func (r *Registry[S, C]) RegisteredFamilies() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	families := make([]string, 0, len(r.entries))
	for family := range r.entries {
		families = append(families, family)
	}
	slices.Sort(families)

	return families
}

// SequenceForChainSelector returns the registered sequence for a chain selector.
func (r *Registry[S, C]) SequenceForChainSelector(chainSelector uint64) (*S, error) {
	family, err := chainselectors.GetSelectorFamily(chainSelector)
	if err != nil {
		return nil, err
	}

	return r.SequenceForFamily(family)
}

// SequenceForFamily returns the registered sequence for a chain family.
func (r *Registry[S, C]) SequenceForFamily(family string) (*S, error) {
	reg, err := r.Get(family)
	if err != nil {
		return nil, err
	}

	return reg.Sequence, nil
}

// VerifyForFamily runs the registered family verify hook, if any. It returns
// nil when the family registers no hook, and wraps hook errors with the family.
func (r *Registry[S, C]) VerifyForFamily(family string, env cldf.Environment, chains []C) error {
	reg, err := r.Get(family)
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

// Get returns the Registration for a chain family, or an error listing the
// registered families when the family is not found.
func (r *Registry[S, C]) Get(family string) (Registration[S, C], error) {
	r.mu.RLock()
	reg, ok := r.entries[family]
	r.mu.RUnlock()

	if ok {
		return reg, nil
	}

	registered := r.RegisteredFamilies()
	if len(registered) == 0 {
		return Registration[S, C]{}, fmt.Errorf(
			"%s: no sequence registered for family %q (none registered)",
			r.label, family,
		)
	}

	return Registration[S, C]{}, fmt.Errorf(
		"%s: no sequence registered for family %q (registered: %s)",
		r.label, family, strings.Join(registered, ", "),
	)
}
