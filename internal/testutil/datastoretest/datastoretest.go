// Package datastoretest provides lightweight fakes for datastore interfaces in unit tests.
package datastoretest

import (
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
)

// NewDataStore returns a read-only DataStore backed by the given address refs.
func NewDataStore(refs []cldfdatastore.AddressRef) cldfdatastore.DataStore {
	return fakeDataStore{store: fakeAddressRefStore{refs: refs}}
}

type fakeAddressRefStore struct {
	refs []cldfdatastore.AddressRef
}

var _ cldfdatastore.AddressRefStore = fakeAddressRefStore{}

func (f fakeAddressRefStore) Fetch() ([]cldfdatastore.AddressRef, error) {
	return f.refs, nil
}

func (f fakeAddressRefStore) Get(key cldfdatastore.AddressRefKey) (cldfdatastore.AddressRef, error) {
	for _, ref := range f.refs {
		if ref.Key().Equals(key) {
			return ref, nil
		}
	}

	return cldfdatastore.AddressRef{}, cldfdatastore.ErrAddressRefNotFound
}

func (f fakeAddressRefStore) Filter(filters ...cldfdatastore.FilterFunc[cldfdatastore.AddressRefKey, cldfdatastore.AddressRef]) []cldfdatastore.AddressRef {
	refs := f.refs
	for _, filter := range filters {
		refs = filter(refs)
	}

	return refs
}

type fakeDataStore struct {
	store fakeAddressRefStore
}

var _ cldfdatastore.DataStore = fakeDataStore{}

func (f fakeDataStore) Addresses() cldfdatastore.AddressRefStore { return f.store }

func (f fakeDataStore) ChainMetadata() cldfdatastore.ChainMetadataStore { return nil }

func (f fakeDataStore) ContractMetadata() cldfdatastore.ContractMetadataStore { return nil }

func (f fakeDataStore) EnvMetadata() cldfdatastore.EnvMetadataStore { return nil }
