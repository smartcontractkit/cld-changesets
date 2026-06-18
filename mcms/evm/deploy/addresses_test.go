package evmdeploy

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

func TestLoadDeployedAddresses(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	v100 := semvers.V1_0_0
	v090 := semver.MustParse("0.9.0")

	bypasserV100 := common.HexToAddress("0x00000000000000000000000000000000000000b1")
	bypasserLegacy := common.HexToAddress("0x00000000000000000000000000000000000000b2")
	bypasserOld := common.HexToAddress("0x00000000000000000000000000000000000000b3")
	proposerV100 := common.HexToAddress("0x00000000000000000000000000000000000000c1")

	t.Run("nil datastore", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, deployedAddresses{}, loadDeployedAddresses(nil, selector, ""))
	})

	t.Run("matches v1.0.0", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       bypasserV100.Hex(),
			Type:          cldfdatastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
			Version:       &v100,
		}))

		addrs := loadDeployedAddresses(ds.Seal(), selector, "")
		require.Equal(t, bypasserV100, addrs.Bypasser)
	})

	t.Run("ignores older version", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       bypasserOld.Hex(),
			Type:          cldfdatastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
			Version:       v090,
		}))

		addrs := loadDeployedAddresses(ds.Seal(), selector, "")
		require.Equal(t, common.Address{}, addrs.Bypasser)
	})

	t.Run("falls back to legacy nil version", func(t *testing.T) {
		t.Parallel()

		store := fakeAddressRefStore{refs: []cldfdatastore.AddressRef{{
			ChainSelector: selector,
			Address:       bypasserLegacy.Hex(),
			Type:          cldfdatastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
		}}}

		got, ok := findDeployedAddress(store, selector, mcmscontracts.BypasserManyChainMultisig, "")
		require.True(t, ok)
		require.Equal(t, bypasserLegacy, got)
	})

	t.Run("prefers v1.0.0 over legacy nil version", func(t *testing.T) {
		t.Parallel()

		store := fakeAddressRefStore{refs: []cldfdatastore.AddressRef{
			{
				ChainSelector: selector,
				Address:       bypasserLegacy.Hex(),
				Type:          cldfdatastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
			},
			{
				ChainSelector: selector,
				Address:       bypasserV100.Hex(),
				Type:          cldfdatastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
				Version:       &v100,
			},
		}}

		got, ok := findDeployedAddress(store, selector, mcmscontracts.BypasserManyChainMultisig, "")
		require.True(t, ok)
		require.Equal(t, bypasserV100, got)
	})

	t.Run("respects qualifier", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       proposerV100.Hex(),
			Type:          cldfdatastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
			Version:       &v100,
			Qualifier:     "prod",
		}))

		addrs := loadDeployedAddresses(ds.Seal(), selector, "")
		require.Equal(t, common.Address{}, addrs.Proposer)

		addrs = loadDeployedAddresses(ds.Seal(), selector, "prod")
		require.Equal(t, proposerV100, addrs.Proposer)
	})
}

func TestFindDeployedAddress(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	v100 := semvers.V1_0_0
	addr := common.HexToAddress("0x00000000000000000000000000000000000000aa")

	ds := cldfdatastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
		ChainSelector: selector,
		Address:       addr.Hex(),
		Type:          cldfdatastore.ContractType(mcmscontracts.RBACTimelock),
		Version:       &v100,
	}))

	got, ok := findDeployedAddress(ds.Addresses(), selector, mcmscontracts.RBACTimelock, "")
	require.True(t, ok)
	require.Equal(t, addr, got)
}

type fakeAddressRefStore struct {
	refs []cldfdatastore.AddressRef
}

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
