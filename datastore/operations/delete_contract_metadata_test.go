package operations_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/datastore/internal/keys"
	"github.com/smartcontractkit/cld-changesets/datastore/operations"
)

func TestDeleteContractMetadataOp_ReportJSONRoundTrip(t *testing.T) {
	t.Parallel()

	contractMetadata1 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value1"}
	contractMetadata2 := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 5678, Metadata: "value2"}

	ds := cldfdatastore.NewMemoryDataStore()
	require.NoError(t, ds.ContractMetadata().Add(contractMetadata1))
	require.NoError(t, ds.ContractMetadata().Add(contractMetadata2))

	deps := operations.DeleteContractMetadataDeps{DataStore: ds.Seal()}
	opInput := operations.DeleteContractMetadataInput{
		ContractMetadataKeys: []keys.ContractMetadataKey{
			{ChainSelector: contractMetadata1.ChainSelector, Address: contractMetadata1.Address},
			{ChainSelector: contractMetadata2.ChainSelector, Address: contractMetadata2.Address},
		},
	}

	bundle := cldfops.NewBundle(t.Context, cldflogger.Test(t), cldfops.NewMemoryReporter())
	report, err := cldfops.ExecuteOperation(bundle, operations.DeleteContractMetadataOp, deps, opInput)
	require.NoError(t, err)

	// Marshal the report Input to JSON, then unmarshal back into DeleteContractMetadataInput.
	inputData, err := json.Marshal(report.Input)
	require.NoError(t, err)

	var recovered operations.DeleteContractMetadataInput
	require.NoError(t, json.Unmarshal(inputData, &recovered))
	require.Equal(t, opInput, recovered)
}
