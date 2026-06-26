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
	"github.com/smartcontractkit/cld-changesets/internal/testutil/datastoretest"
)

func TestLoadDeployedAddresses(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	v100 := semvers.V1_0_0
	v090 := semver.MustParse("0.9.0")

	bypasserV100 := common.HexToAddress("0x00000000000000000000000000000000000000b1")
	bypasserOld := common.HexToAddress("0x00000000000000000000000000000000000000b3")
	proposerV100 := common.HexToAddress("0x00000000000000000000000000000000000000c1")

	t.Run("nil datastore", func(t *testing.T) {
		t.Parallel()
		addrs, err := loadDeployedAddresses(nil, selector, "")
		require.NoError(t, err)
		require.Equal(t, deployedAddresses{}, addrs)
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

		addrs, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.NoError(t, err)
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

		addrs, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.NoError(t, err)
		require.Equal(t, common.Address{}, addrs.Bypasser)
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

		addrs, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.NoError(t, err)
		require.Equal(t, common.Address{}, addrs.Proposer)

		addrs, err = loadDeployedAddresses(ds.Seal(), selector, "prod")
		require.NoError(t, err)
		require.Equal(t, proposerV100, addrs.Proposer)
	})

	t.Run("duplicate refs", func(t *testing.T) {
		t.Parallel()

		_, err := loadDeployedAddresses(datastoretest.NewDataStore([]cldfdatastore.AddressRef{
			{
				ChainSelector: selector,
				Address:       bypasserV100.Hex(),
				Type:          cldfdatastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
				Version:       &v100,
			},
			{
				ChainSelector: selector,
				Address:       bypasserOld.Hex(),
				Type:          cldfdatastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
				Version:       &v100,
			},
		}), selector, "")
		require.ErrorIs(t, err, cldfdatastore.ErrAddressRefQueryAmbiguous)
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

	got, ok, err := findDeployedAddress(ds.Addresses(), selector, mcmscontracts.RBACTimelock, "")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, addr, got)
}
