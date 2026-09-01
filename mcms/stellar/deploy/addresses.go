package stellardeploy

import (
	"errors"
	"fmt"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

func loadAddressRef(ds datastore.DataStore, chainSelector uint64, contractType cldf.ContractType, qualifier string) (datastore.AddressRef, bool, error) {
	if ds == nil {
		return datastore.AddressRef{}, false, nil
	}

	ref, err := ds.Addresses().Get(datastore.NewAddressRefKey(
		chainSelector,
		datastore.ContractType(contractType),
		&semvers.V1_0_0,
		qualifier,
	))
	if err == nil {
		return ref, true, nil
	}
	if errors.Is(err, datastore.ErrAddressRefNotFound) {
		return datastore.AddressRef{}, false, nil
	}

	return datastore.AddressRef{}, false, fmt.Errorf("load %s address ref on chain %d with qualifier %q: %w", contractType, chainSelector, qualifier, err)
}

func newAddressRef(chainSelector uint64, contractType cldf.ContractType, address string, qualifier string, label string) datastore.AddressRef {
	labels := datastore.NewLabelSet()
	if label != "" {
		labels.Add(label)
	}

	return datastore.AddressRef{
		ChainSelector: chainSelector,
		Address:       address,
		Type:          datastore.ContractType(contractType),
		Version:       &semvers.V1_0_0,
		Qualifier:     qualifier,
		Labels:        labels,
	}
}
