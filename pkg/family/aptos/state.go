package aptos

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/aptos-labs/aptos-go-sdk"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

const (
	// todo: move to CLDF?
	AptosMCMSType cldf.ContractType = "AptosManyChainMultisig"
)

// LoadMCMSAddresses tries to load the mcms addresses for all given chain selectors from the environment.
// mcmsContractVersion is the semver that must match each candidate address ref's Version field.
// If no mcms address can be found for any given chain selector, an error will be returned.
func LoadMCMSAddresses(env cldf.Environment, chainSelectors []uint64, mcmsContractVersion semver.Version) (map[uint64]aptos.AccountAddress, error) {
	result := make(map[uint64]aptos.AccountAddress)
	for _, selector := range chainSelectors {
		var mcmsAddress aptos.AccountAddress
		found := false
		refs := env.DataStore.Addresses().Filter(datastore.AddressRefByChainSelector(selector))
		for _, ref := range refs {
			if ref.Type != datastore.ContractType(AptosMCMSType) || ref.Version == nil || !ref.Version.Equal(&mcmsContractVersion) {
				continue
			}
			if err := mcmsAddress.ParseStringRelaxed(ref.Address); err != nil {
				return nil, fmt.Errorf(
					"failed to parse MCMS address for Aptos chain selector %d (type=%s, version=%s, address=%s): %w",
					selector, ref.Type, ref.Version.String(), ref.Address, err,
				)
			}
			found = true

			break
		}
		if !found {
			return nil, fmt.Errorf("no MCMS address found for Aptos chain selector: %d", selector)
		}
		result[selector] = mcmsAddress
	}

	return result, nil
}
