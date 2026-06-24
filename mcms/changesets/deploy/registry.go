package deploy

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"

	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

// Registration describes one chain family's MCMS deploy implementation. Its
// Sequence executes the per-chain deploy and returns newly deployed addresses
// via OnChainOutput.Metadata.Addresses.
type Registration = familyregistry.Registration[Sequence, ChainInput]

// Registry holds per-family MCMS deploy sequences.
var Registry = familyregistry.New[Sequence, ChainInput]("mcms deploy")

func registerAll(regs ...Registration) {
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
