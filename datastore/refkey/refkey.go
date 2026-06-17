package refkey

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

// RefKey identifies a contract in the environment datastore without carrying its address.
type RefKey struct {
	ChainSelector uint64                     `json:"chainSelector"`
	Type          cldfdatastore.ContractType `json:"type"`
	Version       *semver.Version            `json:"version"`
	Qualifier     string                     `json:"qualifier,omitempty"`
}

// New constructs a serializable ref key.
func New(chainSelector uint64, contractType cldfdatastore.ContractType, version *semver.Version, qualifier string) RefKey {
	return RefKey{
		ChainSelector: chainSelector,
		Type:          contractType,
		Version:       version,
		Qualifier:     qualifier,
	}
}

// FromAddressRef derives a ref key from a full AddressRef.
func FromAddressRef(ref cldfdatastore.AddressRef) RefKey {
	return RefKey{
		ChainSelector: ref.ChainSelector,
		Type:          ref.Type,
		Version:       ref.Version,
		Qualifier:     ref.Qualifier,
	}
}

// Key converts the serializable key into a datastore AddressRefKey.
func (k RefKey) Key() (cldfdatastore.AddressRefKey, error) {
	if k.Version == nil {
		return nil, cldfdatastore.ErrAddressRefVersionRequired
	}

	return cldfdatastore.NewAddressRefKey(k.ChainSelector, k.Type, k.Version, k.Qualifier), nil
}

// Resolve loads the matching AddressRef from the environment datastore.
func (k RefKey) Resolve(e cldf.Environment) (cldfdatastore.AddressRef, error) {
	if e.DataStore == nil {
		return cldfdatastore.AddressRef{}, errors.New("missing datastore in environment")
	}

	key, err := k.Key()
	if err != nil {
		return cldfdatastore.AddressRef{}, err
	}

	ref, err := e.DataStore.Addresses().Get(key)
	if err != nil {
		return cldfdatastore.AddressRef{}, fmt.Errorf("address ref %v: %w", key, err)
	}

	return ref, nil
}
