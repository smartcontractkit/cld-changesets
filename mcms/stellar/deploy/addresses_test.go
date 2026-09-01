package stellardeploy

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

func TestNewAddressRef(t *testing.T) {
	t.Parallel()

	const (
		selector  uint64 = 123
		address          = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM"
		qualifier        = "qualified"
		label            = "test-label"
	)

	expectedLabels := datastore.NewLabelSet()
	expectedLabels.Add(label)

	got := newAddressRef(
		selector,
		mcmscontracts.ProposerManyChainMultisig,
		address,
		qualifier,
		label,
	)

	require.Equal(t, selector, got.ChainSelector)
	require.Equal(t, address, got.Address)
	require.Equal(
		t,
		datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		got.Type,
	)
	require.Equal(t, &semvers.V1_0_0, got.Version)
	require.Equal(t, qualifier, got.Qualifier)
	require.Equal(t, expectedLabels, got.Labels)
}

func TestNewAddressRef_EmptyLabel(t *testing.T) {
	t.Parallel()

	got := newAddressRef(
		123,
		mcmscontracts.ProposerManyChainMultisig,
		"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM",
		"",
		"",
	)

	require.Equal(t, datastore.NewLabelSet(), got.Labels)
}

func TestLoadAddressRef(t *testing.T) {
	t.Parallel()

	const (
		selector  uint64 = 123
		address          = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM"
		qualifier        = "qualified"
	)

	contractType := mcmscontracts.ProposerManyChainMultisig

	t.Run("nil datastore", func(t *testing.T) {
		t.Parallel()

		ref, exists, err := loadAddressRef(
			nil,
			selector,
			contractType,
			qualifier,
		)

		require.NoError(t, err)
		require.False(t, exists)
		require.Equal(t, datastore.AddressRef{}, ref)
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		ds := datastore.NewMemoryDataStore()

		ref, exists, err := loadAddressRef(
			ds.Seal(),
			selector,
			contractType,
			qualifier,
		)

		require.NoError(t, err)
		require.False(t, exists)
		require.Equal(t, datastore.AddressRef{}, ref)
	})

	t.Run("exact match", func(t *testing.T) {
		t.Parallel()

		want := newAddressRef(
			selector,
			contractType,
			address,
			qualifier,
			"label",
		)

		ds := dataStoreWithAddressRef(t, want)

		got, exists, err := loadAddressRef(
			ds,
			selector,
			contractType,
			qualifier,
		)

		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, want, got)
	})

	t.Run("qualifier isolation", func(t *testing.T) {
		t.Parallel()

		ds := dataStoreWithAddressRef(
			t,
			newAddressRef(
				selector,
				contractType,
				address,
				"other",
				"",
			),
		)

		_, exists, err := loadAddressRef(
			ds,
			selector,
			contractType,
			qualifier,
		)

		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("contract type isolation", func(t *testing.T) {
		t.Parallel()

		ds := dataStoreWithAddressRef(
			t,
			newAddressRef(
				selector,
				mcmscontracts.CancellerManyChainMultisig,
				address,
				qualifier,
				"",
			),
		)

		_, exists, err := loadAddressRef(
			ds,
			selector,
			contractType,
			qualifier,
		)

		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("selector isolation", func(t *testing.T) {
		t.Parallel()

		ds := dataStoreWithAddressRef(
			t,
			newAddressRef(
				selector+1,
				contractType,
				address,
				qualifier,
				"",
			),
		)

		_, exists, err := loadAddressRef(
			ds,
			selector,
			contractType,
			qualifier,
		)

		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("version isolation", func(t *testing.T) {
		t.Parallel()

		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			ChainSelector: selector,
			Address:       address,
			Type:          datastore.ContractType(contractType),
			Version:       semver.MustParse("2.0.0"),
			Qualifier:     qualifier,
		}))

		_, exists, err := loadAddressRef(
			ds.Seal(),
			selector,
			contractType,
			qualifier,
		)

		require.NoError(t, err)
		require.False(t, exists)
	})
}

func dataStoreWithAddressRef(
	t *testing.T,
	ref datastore.AddressRef,
) datastore.DataStore {
	t.Helper()

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(ref))

	return ds.Seal()
}
