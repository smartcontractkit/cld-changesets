// Package keys provides JSON-friendly mirror types for the framework datastore key interfaces.
// Conversion to framework keys goes through the framework's public constructors, keeping this
// package datastore-implementation-agnostic.
package keys

import (
	"github.com/Masterminds/semver/v3"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
)

// ChainMetadataKey is a JSON-serializable mirror of cldfdatastore.ChainMetadataKey.
type ChainMetadataKey struct {
	ChainSelector uint64 `json:"chainSelector"`
}

// ToFrameworkKey returns the equivalent cldfdatastore.ChainMetadataKey.
func (k ChainMetadataKey) ToFrameworkKey() cldfdatastore.ChainMetadataKey {
	return cldfdatastore.NewChainMetadataKey(k.ChainSelector)
}

// ContractMetadataKey is a JSON-serializable mirror of cldfdatastore.ContractMetadataKey.
type ContractMetadataKey struct {
	ChainSelector uint64 `json:"chainSelector"`
	Address       string `json:"address"`
}

// ToFrameworkKey returns the equivalent cldfdatastore.ContractMetadataKey.
func (k ContractMetadataKey) ToFrameworkKey() cldfdatastore.ContractMetadataKey {
	return cldfdatastore.NewContractMetadataKey(k.ChainSelector, k.Address)
}

// AddressRefKey is a JSON-serializable mirror of cldfdatastore.AddressRefKey.
type AddressRefKey struct {
	ChainSelector uint64                     `json:"chainSelector"`
	Type          cldfdatastore.ContractType `json:"type"`
	Version       *semver.Version            `json:"version"`
	Qualifier     string                     `json:"qualifier"`
}

// ToFrameworkKey returns the equivalent cldfdatastore.AddressRefKey, or
// cldfdatastore.ErrAddressRefVersionRequired if Version is nil.
func (k AddressRefKey) ToFrameworkKey() (cldfdatastore.AddressRefKey, error) {
	if k.Version == nil {
		return nil, cldfdatastore.ErrAddressRefVersionRequired
	}

	return cldfdatastore.NewAddressRefKey(k.ChainSelector, k.Type, k.Version, k.Qualifier), nil
}
