package deploy

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"

	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

// Registration describes one chain family's MCMS deploy implementation.
type Registration = familyregistry.Registration[ChainInput, *Sequence]

// Registry holds registered per-family MCMS deploy sequences.
var Registry = familyregistry.New[ChainInput, *Sequence]("mcms deploy")

// RegisterFamilies registers one or more chain-family deploy implementations.
// Each family may only be registered once; duplicate registration panics.
//
// External teams call this once at startup from their own module:
//
//	deploy.RegisterFamilies(aptosimpl.Registration())
//
// Built-in families (EVM) register themselves automatically via their package
// init function when imported:
//
//	import _ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy"
func RegisterFamilies(regs ...Registration) {
	for _, reg := range regs {
		Registry.Register(reg)
	}
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
