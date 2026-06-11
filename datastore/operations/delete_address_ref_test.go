package operations_test

import (
	"encoding/json"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/datastore/internal/keys"
	"github.com/smartcontractkit/cld-changesets/datastore/operations"
)

func TestDeleteAddressRefOp_ReportJSONRoundTrip(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef1 := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	addressRef2 := cldfdatastore.AddressRef{Address: "0x02", ChainSelector: 5678, Type: "OtherContract", Version: version, Qualifier: ""}

	ds := cldfdatastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(addressRef1))
	require.NoError(t, ds.Addresses().Add(addressRef2))

	deps := operations.DeleteAddressRefDeps{DataStore: ds.Seal()}
	opInput := operations.DeleteAddressRefInput{
		AddressRefKeys: []keys.AddressRefKey{
			{ChainSelector: addressRef1.ChainSelector, Type: addressRef1.Type, Version: addressRef1.Version, Qualifier: addressRef1.Qualifier},
			{ChainSelector: addressRef2.ChainSelector, Type: addressRef2.Type, Version: addressRef2.Version, Qualifier: addressRef2.Qualifier},
		},
	}

	bundle := cldfops.NewBundle(t.Context, cldflogger.Test(t), cldfops.NewMemoryReporter())
	report, err := cldfops.ExecuteOperation(bundle, operations.DeleteAddressRefOp, deps, opInput)
	require.NoError(t, err)

	// Marshal the report Input to JSON, then unmarshal back into DeleteAddressRefInput.
	inputData, err := json.Marshal(report.Input)
	require.NoError(t, err)

	var recovered operations.DeleteAddressRefInput
	require.NoError(t, json.Unmarshal(inputData, &recovered))

	require.Len(t, recovered.AddressRefKeys, len(opInput.AddressRefKeys))
	for i, k := range opInput.AddressRefKeys {
		require.Equal(t, k.ChainSelector, recovered.AddressRefKeys[i].ChainSelector)
		require.Equal(t, k.Type, recovered.AddressRefKeys[i].Type)
		require.True(t, k.Version.Equal(recovered.AddressRefKeys[i].Version))
		require.Equal(t, k.Qualifier, recovered.AddressRefKeys[i].Qualifier)
	}
}
