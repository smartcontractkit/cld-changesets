package operations_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/datastore/keys"
	"github.com/smartcontractkit/cld-changesets/datastore/operations"
)

func TestDeleteChainMetadataOp_ReportJSONRoundTrip(t *testing.T) {
	t.Parallel()

	chainMetadata1 := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "value1"}
	chainMetadata2 := cldfdatastore.ChainMetadata{ChainSelector: 5678, Metadata: "value2"}

	ds := cldfdatastore.NewMemoryDataStore()
	require.NoError(t, ds.ChainMetadata().Add(chainMetadata1))
	require.NoError(t, ds.ChainMetadata().Add(chainMetadata2))

	deps := operations.DeleteChainMetadataDeps{DataStore: ds.Seal()}
	opInput := operations.DeleteChainMetadataInput{
		ChainMetadataKeys: []keys.ChainMetadataKey{
			{ChainSelector: chainMetadata1.ChainSelector},
			{ChainSelector: chainMetadata2.ChainSelector},
		},
	}

	bundle := cldfops.NewBundle(t.Context, cldflogger.Test(t), cldfops.NewMemoryReporter())
	report, err := cldfops.ExecuteOperation(bundle, operations.DeleteChainMetadataOp, deps, opInput)
	require.NoError(t, err)

	// Marshal the report Input to JSON, then unmarshal back into DeleteChainMetadataInput.
	inputData, err := json.Marshal(report.Input)
	require.NoError(t, err)

	var recovered operations.DeleteChainMetadataInput
	require.NoError(t, json.Unmarshal(inputData, &recovered))
	require.Equal(t, opInput, recovered)
}
