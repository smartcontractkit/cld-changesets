package evmsetconfig

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	evmreaders "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
)

func validateMCMSIfPresent(e cldf.Environment, in setconfig.ChainInput) error {
	if in.MCMS == nil {
		return nil
	}

	input := *in.MCMS
	chainSelector := in.ChainSelector

	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyEVM)
	if !ok {
		return fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilyEVM)
	}

	if _, err := reader.GetTimelockRef(e, chainSelector, input); err != nil {
		return fmt.Errorf("validate timelock ref for chain %d: %w", chainSelector, err)
	}
	if _, err := reader.GetMCMSRef(e, chainSelector, input); err != nil {
		return fmt.Errorf("validate mcms ref for chain %d: %w", chainSelector, err)
	}

	evmReader, ok := reader.(evmreaders.CallProxyReader)
	if !ok {
		return fmt.Errorf("validate call proxy ref for chain %d: reader for family %q does not support call proxy lookup", chainSelector, chainselectors.FamilyEVM)
	}
	if _, err := evmReader.GetCallProxyRef(e, chainSelector, input.Qualifier); err != nil {
		return fmt.Errorf("validate call proxy ref for chain %d: %w", chainSelector, err)
	}

	return nil
}
