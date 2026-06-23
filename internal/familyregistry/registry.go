package familyregistry

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

// Registration describes one chain family's changeset sequence implementation.
type Registration[Input any, Sequence any] struct {
	Family string
	// Sequence executes the per-chain operations sequence for this family.
	Sequence Sequence
	// Verify performs family-specific validation across all chains in the input.
	// It is called during VerifyPreconditions. Optional — nil means no extra checks.
	Verify func(env cldf.Environment, in []Input) error
}

// Registry is a thread-safe family → sequence registry.
type Registry[Input any, Sequence any] struct {
	name string
	mu   sync.RWMutex
	m    map[string]Registration[Input, Sequence]
}

// New returns an empty family registry. name is the changeset label in errors
// (e.g. "mcms transfer-to-mcms").
func New[Input any, Sequence any](name string) *Registry[Input, Sequence] {
	return &Registry[Input, Sequence]{
		name: name,
		m:    make(map[string]Registration[Input, Sequence]),
	}
}

// Register adds a family sequence. Panics on invalid input or duplicate registration.
func (r *Registry[Input, Sequence]) Register(reg Registration[Input, Sequence]) {
	if reg.Family == "" {
		panic(r.name + ": family is required")
	}
	if isNil(reg.Sequence) {
		panic(fmt.Sprintf("%s: sequence is required for family %q", r.name, reg.Family))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.m[reg.Family]; exists {
		panic(fmt.Sprintf("%s: family %q already registered", r.name, reg.Family))
	}

	r.m[reg.Family] = reg
}

// RegisteredFamilies returns the sorted list of families with a registered sequence.
func (r *Registry[Input, Sequence]) RegisteredFamilies() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.registeredFamiliesLocked()
}

// RegistrationForFamily returns the registration for a chain family.
func (r *Registry[Input, Sequence]) RegistrationForFamily(family string) (Registration[Input, Sequence], error) {
	r.mu.RLock()
	reg, ok := r.m[family]
	if ok {
		r.mu.RUnlock()

		return reg, nil
	}
	registered := r.registeredFamiliesLocked()
	r.mu.RUnlock()

	if len(registered) == 0 {
		return Registration[Input, Sequence]{}, fmt.Errorf(
			"%s: no sequence registered for family %q (none registered — import a family package or the changeset all package)",
			r.name,
			family,
		)
	}

	return Registration[Input, Sequence]{}, fmt.Errorf(
		"%s: no sequence registered for family %q (registered: %s)",
		r.name,
		family,
		strings.Join(registered, ", "),
	)
}

// SequenceForFamily returns the registered sequence for a chain family.
func (r *Registry[Input, Sequence]) SequenceForFamily(family string) (Sequence, error) {
	reg, err := r.RegistrationForFamily(family)
	if err != nil {
		var zero Sequence
		return zero, err
	}

	return reg.Sequence, nil
}

// SequenceForChainSelector returns the registered sequence for a chain selector.
func (r *Registry[Input, Sequence]) SequenceForChainSelector(chainSelector uint64) (Sequence, error) {
	family, err := chainselectors.GetSelectorFamily(chainSelector)
	if err != nil {
		var zero Sequence
		return zero, err
	}

	return r.SequenceForFamily(family)
}

// VerifyForFamily runs the registered family verify hook, if any.
func (r *Registry[Input, Sequence]) VerifyForFamily(
	family string,
	env cldf.Environment,
	in []Input,
) error {
	reg, err := r.RegistrationForFamily(family)
	if err != nil {
		return err
	}
	if reg.Verify == nil {
		return nil
	}
	if err := reg.Verify(env, in); err != nil {
		return fmt.Errorf("family %s: %w", family, err)
	}

	return nil
}

func (r *Registry[Input, Sequence]) registeredFamiliesLocked() []string {
	families := make([]string, 0, len(r.m))
	for family := range r.m {
		families = append(families, family)
	}
	slices.Sort(families)

	return families
}

func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() { //nolint:exhaustive // only nil-able kinds are rejected as missing sequences
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}
