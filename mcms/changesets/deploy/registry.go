package deploy

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Registration)
)

// Register adds a family deploy implementation to the registry.
// It panics if the family string is empty, Sequence is nil, or the family is
// already registered — all of which indicate a programming error at startup.
func Register(reg Registration) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if reg.Family == "" {
		panic("mcms deploy: family is required")
	}
	if reg.Sequence == nil {
		panic(fmt.Sprintf("mcms deploy: sequence is required for family %q", reg.Family))
	}
	if _, exists := registry[reg.Family]; exists {
		panic(fmt.Sprintf("mcms deploy: family %q already registered", reg.Family))
	}

	registry[reg.Family] = reg
}

func registerAll(regs ...Registration) {
	for _, reg := range regs {
		Register(reg)
	}
}

// get returns the Registration for a chain family.
// Returns an error listing registered families when the family is not found.
func get(family string) (Registration, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	reg, ok := registry[family]
	if !ok {
		registered := registeredFamiliesLocked()
		if len(registered) == 0 {
			return Registration{}, fmt.Errorf(
				"mcms deploy: no sequence registered for family %q (none registered — import a family package or call RegisterFamilies)",
				family,
			)
		}

		return Registration{}, fmt.Errorf(
			"mcms deploy: no sequence registered for family %q (registered: %s)",
			family,
			strings.Join(registered, ", "),
		)
	}

	return reg, nil
}

// RegisteredFamilies returns the sorted list of registered chain families.
func RegisteredFamilies() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	return registeredFamiliesLocked()
}

// groupByFamily groups deployment inputs by their chain-selectors family string.
func groupByFamily(cfgByChain map[uint64]cldfproposalutils.MCMSWithTimelockConfig) (map[string][]ChainInput, error) {
	byFamily := make(map[string][]ChainInput, len(cfgByChain))
	for selector, cfg := range cfgByChain {
		family, err := chainselectors.GetSelectorFamily(selector)
		if err != nil {
			return nil, fmt.Errorf("chain selector %d: %w", selector, err)
		}
		byFamily[family] = append(byFamily[family], ChainInput{
			ChainSelector: selector,
			Config:        cfg,
		})
	}

	return byFamily, nil
}

func registeredFamiliesLocked() []string {
	families := make([]string, 0, len(registry))
	for family := range registry {
		families = append(families, family)
	}
	slices.Sort(families)

	return families
}
