package familyregistry

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

// Registration describes one chain family's MCMS changeset sequence implementation.
type Registration[S any, C any] struct {
	Family string
	// Sequence executes the per-chain operations sequence.
	Sequence *S
	// Verify performs family-specific validation across all chains in the input.
	// It is called during VerifyPreconditions. Optional — nil means no extra checks.
	Verify func(env cldf.Environment, chains []C) error
}

// Options configures a family sequence registry.
type Options struct {
	Name               string
	NoneRegisteredHint string
}

// Registry holds family sequence registrations for a changeset.
type Registry[S any, C any] struct {
	opts Options
	mu   sync.RWMutex
	m    map[string]Registration[S, C]
}

// New creates an empty family sequence registry.
func New[S any, C any](opts Options) *Registry[S, C] {
	return &Registry[S, C]{
		opts: opts,
		m:    make(map[string]Registration[S, C]),
	}
}

// Register adds a family sequence. Panics on invalid input or duplicate registration.
func (r *Registry[S, C]) Register(reg Registration[S, C]) {
	prefix := fmt.Sprintf("mcms %s", r.opts.Name)
	if reg.Family == "" {
		panic(fmt.Sprintf("%s: family is required", prefix))
	}
	if reg.Sequence == nil {
		panic(fmt.Sprintf("%s: sequence is required for family %q", prefix, reg.Family))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.m[reg.Family]; exists {
		panic(fmt.Sprintf("%s: family %q already registered", prefix, reg.Family))
	}

	r.m[reg.Family] = reg
}

// RegisteredFamilies returns the sorted list of families with a registered sequence.
func (r *Registry[S, C]) RegisteredFamilies() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	families := make([]string, 0, len(r.m))
	for family := range r.m {
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
	reg, err := r.registrationForFamily(family)
	if err != nil {
		return nil, err
	}

	return reg.Sequence, nil
}

// VerifyForFamily runs the registered family verify hook, if any.
func (r *Registry[S, C]) VerifyForFamily(family string, env cldf.Environment, chains []C) error {
	reg, err := r.registrationForFamily(family)
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

func (r *Registry[S, C]) registrationForFamily(family string) (Registration[S, C], error) {
	r.mu.RLock()
	reg, ok := r.m[family]
	r.mu.RUnlock()

	if ok {
		return reg, nil
	}

	prefix := fmt.Sprintf("mcms %s", r.opts.Name)
	registered := r.RegisteredFamilies()
	if len(registered) == 0 {
		return Registration[S, C]{}, fmt.Errorf(
			"%s: no sequence registered for family %q (none registered — %s)",
			prefix,
			family,
			r.opts.NoneRegisteredHint,
		)
	}

	return Registration[S, C]{}, fmt.Errorf(
		"%s: no sequence registered for family %q (registered: %s)",
		prefix,
		family,
		strings.Join(registered, ", "),
	)
}
