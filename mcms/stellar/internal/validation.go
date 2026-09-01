package stellarinternal

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

func ValidateMCMSRefs(env cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) error {
	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyStellar)
	if !ok {
		return fmt.Errorf("no MCMS reader registered for chain family %q", chainselectors.FamilyStellar)
	}

	if _, err := reader.GetTimelockRef(env, chainSelector, input); err != nil {
		return fmt.Errorf("resolve Stellar timelock for chain %d: %w", chainSelector, err)
	}

	if _, err := reader.GetMCMSRef(env, chainSelector, input); err != nil {
		return fmt.Errorf("resolve Stellar MCMS for chain %d: %w", chainSelector, err)
	}

	return nil
}
