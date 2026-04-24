package aptos

import (
	"fmt"

	"github.com/aptos-labs/aptos-go-sdk"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/pkg/common"
)

const (
	// todo: move to CLDF?
	AptosMCMSType cldf.ContractType = "AptosManyChainMultisig"
)

// LoadMCMSAddresses tries to load the mcms addresses for all given chain selectors from the environment.
// If no mcms address can be found for any given chain selector, an error will be returned.
func LoadMCMSAddresses(env cldf.Environment, chainSelectors []uint64) (map[uint64]aptos.AccountAddress, error) {
	result := make(map[uint64]aptos.AccountAddress)
	for _, selector := range chainSelectors {
		addresses, err := env.ExistingAddresses.AddressesForChain(selector) //nolint:staticcheck // SA1019: AddressBook deprecated; migrate to DataStore when Aptos MCMS refs live there.
		if err != nil {
			return nil, fmt.Errorf("failed to load addresses for Aptos chain %d: %w", selector, err)
		}
		var mcmsAddress aptos.AccountAddress
		for address, tv := range addresses {
			if tv.Equal(cldf.TypeAndVersion{
				Type:    AptosMCMSType,
				Version: common.Version1_6_0,
			}) {
				if err := mcmsAddress.ParseStringRelaxed(address); err != nil {
					return nil, fmt.Errorf(
						"failed to parse Aptos MCMS address for chain %d (type=%s, version=%s, address=%s): %w",
						selector, AptosMCMSType, common.Version1_6_0.String(), address, err,
					)
				}

				break
			}
		}
		if (mcmsAddress == aptos.AccountAddress{}) {
			return nil, fmt.Errorf("no MCMS address found for Aptos chain: %d", selector)
		}
		result[selector] = mcmsAddress
	}

	return result, nil
}
