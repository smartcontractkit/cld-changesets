package grantrole

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

// Registration describes one chain family's grant-role implementation.
type Registration = familyregistry.Registration[Sequence, SeqInput]

// Registry holds per-family grant-role sequences.
var Registry = familyregistry.New[Sequence, SeqInput]("mcms grant-role")

func groupByFamily(input Input) (map[string][]SeqInput, error) {
	byFamily := make(map[string][]SeqInput)
	for chainSelector, grants := range input.Cfg.GrantsByChain {
		family, err := chainselectors.GetSelectorFamily(chainSelector)
		if err != nil {
			return nil, fmt.Errorf("chain selector %d: %w", chainSelector, err)
		}
		byFamily[family] = append(byFamily[family], SeqInput{
			ChainSelector:  chainSelector,
			Grants:         grants,
			MCMS:           input.MCMS,
			GasBoostConfig: input.Cfg.GasBoostConfig,
		})
	}

	return byFamily, nil
}
