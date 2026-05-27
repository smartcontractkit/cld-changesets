package changesets

import (
	"fmt"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

func linkTokenTypeAndVersion() cldf.TypeAndVersion {
	return cldf.NewTypeAndVersion(linkcontracts.LinkToken, semvers.V1_0_0)
}

func staticLinkTokenTypeAndVersion() cldf.TypeAndVersion {
	return cldf.NewTypeAndVersion(linkcontracts.StaticLinkToken, semvers.V1_0_0)
}

func saveAddressRef(ds datastore.MutableDataStore, chainSelector uint64, address string, tv cldf.TypeAndVersion, qualifier string) error {
	return ds.Addresses().Add(datastore.AddressRef{
		ChainSelector: chainSelector,
		Address:       address,
		Type:          datastore.ContractType(tv.Type.String()),
		Version:       &tv.Version,
		Qualifier:     qualifier,
		Labels:        datastore.NewLabelSet(),
	})
}

func validateSelectorsFamily(chains []uint64, family string) error {
	for _, chain := range chains {
		selectorFamily, err := chainsel.GetSelectorFamily(chain)
		if err != nil {
			return fmt.Errorf("failed to get family for chain selector %d: %w", chain, err)
		}
		if selectorFamily != family {
			return fmt.Errorf("chain selector %d is not in the %s family", chain, family)
		}
	}

	return nil
}

func validateNoExistingContract(e cldf.Environment, chains []uint64, tv cldf.TypeAndVersion, qualifier string) error {
	if e.DataStore == nil {
		return nil
	}

	for _, chain := range chains {
		refs := e.DataStore.Addresses().Filter(
			datastore.AddressRefByChainSelector(chain),
			datastore.AddressRefByType(datastore.ContractType(tv.Type.String())),
			datastore.AddressRefByQualifier(qualifier),
		)
		for _, ref := range refs {
			if ref.Version == nil || ref.Version.Equal(&tv.Version) {
				return fmt.Errorf("%s contract already exists for chain selector %d in datastore", tv.Type, chain)
			}
		}
	}

	return nil
}
