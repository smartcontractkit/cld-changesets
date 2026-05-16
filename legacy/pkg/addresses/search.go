package addresses

import (
	"fmt"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
)

// SearchAddress searches for a contract address in both AddressBook and DataStore
// Returns the address if found in either source
func SearchAddress(e cldf.Environment, chainSelector uint64, address string) (bool, error) {
	// Use the merged address loading from the EVM state function
	addressesChain, err := evmstate.AddressesForChain(e, chainSelector, "")
	if err != nil {
		return false, fmt.Errorf("failed to load addresses: %w", err)
	}

	// Search through merged addresses for the contract type
	for addr := range addressesChain {
		if addr == address {
			return true, nil
		}
	}

	return false, fmt.Errorf("%s not found", address)
}
