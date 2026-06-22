package evmfiredrill

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	firedrill "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill"
)

func validateMCMS(e cldf.Environment, in firedrill.ChainInput) error {
	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyEVM)
	if !ok {
		return fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilyEVM)
	}

	if _, err := reader.GetTimelockRef(e, in.ChainSelector, in.MCMS); err != nil {
		return fmt.Errorf("validate timelock ref for chain %d: %w", in.ChainSelector, err)
	}
	if _, err := reader.GetMCMSRef(e, in.ChainSelector, in.MCMS); err != nil {
		return fmt.Errorf("validate mcms ref for chain %d: %w", in.ChainSelector, err)
	}

	return nil
}
