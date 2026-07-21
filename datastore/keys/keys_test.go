package keys_test

import (
	"encoding/json"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"

	"github.com/smartcontractkit/cld-changesets/datastore/keys"
)

func TestChainMetadataKey_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := keys.ChainMetadataKey{ChainSelector: 1234}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got keys.ChainMetadataKey
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, original, got)
}

func TestChainMetadataKey_ToFrameworkKey(t *testing.T) {
	t.Parallel()

	k := keys.ChainMetadataKey{ChainSelector: 9999}
	fwKey := k.ToFrameworkKey()

	require.Equal(t, uint64(9999), fwKey.ChainSelector())
}

func TestContractMetadataKey_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := keys.ContractMetadataKey{ChainSelector: 1234, Address: "0xDEAD"}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got keys.ContractMetadataKey
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, original, got)
}

func TestContractMetadataKey_ToFrameworkKey(t *testing.T) {
	t.Parallel()

	k := keys.ContractMetadataKey{ChainSelector: 42, Address: "0xBEEF"}
	fwKey := k.ToFrameworkKey()

	require.Equal(t, uint64(42), fwKey.ChainSelector())
	require.Equal(t, "0xBEEF", fwKey.Address())
}

func TestAddressRefKey_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := keys.AddressRefKey{
		ChainSelector: 1234,
		Type:          cldfdatastore.ContractType("MyContract"),
		Version:       semver.MustParse("1.0.0"),
		Qualifier:     "",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var got keys.AddressRefKey
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, original.ChainSelector, got.ChainSelector)
	require.Equal(t, original.Type, got.Type)
	require.True(t, original.Version.Equal(got.Version))
	require.Equal(t, original.Qualifier, got.Qualifier)
}

func TestAddressRefKey_ToFrameworkKey(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("2.3.4")
	k := keys.AddressRefKey{
		ChainSelector: 7,
		Type:          cldfdatastore.ContractType("SomeContract"),
		Version:       version,
		Qualifier:     "primary",
	}

	fwKey, err := k.ToFrameworkKey()
	require.NoError(t, err)

	require.Equal(t, uint64(7), fwKey.ChainSelector())
	require.Equal(t, cldfdatastore.ContractType("SomeContract"), fwKey.Type())
	require.True(t, version.Equal(fwKey.Version()))
	require.Equal(t, "primary", fwKey.Qualifier())
}

func TestAddressRefKey_ToFrameworkKey_NilVersionError(t *testing.T) {
	t.Parallel()

	k := keys.AddressRefKey{
		ChainSelector: 1,
		Type:          "AnyContract",
		Version:       nil,
		Qualifier:     "",
	}

	_, err := k.ToFrameworkKey()
	require.ErrorIs(t, err, cldfdatastore.ErrAddressRefVersionRequired)
}
