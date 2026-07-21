package changesets

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfoperations "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/datastore/keys"
)

func TestDeleteContractMetadataChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	contractMetadata1 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value1"}
	contractMetadata2 := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 1234, Metadata: "value2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   DeleteContractMetadataChangesetInput
		wantErr string
	}{
		{
			name: "success: valid preconditions",
			env: cldf.Environment{
				DataStore: testDataStoreWithContractMetadata(t, contractMetadata1, contractMetadata2).Seal(),
			},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []keys.ContractMetadataKey{{ChainSelector: contractMetadata1.ChainSelector, Address: contractMetadata1.Address}, {ChainSelector: contractMetadata2.ChainSelector, Address: contractMetadata2.Address}},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []keys.ContractMetadataKey{{ChainSelector: contractMetadata1.ChainSelector, Address: contractMetadata1.Address}},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no contract metadata keys given",
			env: cldf.Environment{
				DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
			},
			input:   DeleteContractMetadataChangesetInput{ContractMetadataKeys: []keys.ContractMetadataKey{}},
			wantErr: "missing contract metadata keys input",
		},
		{
			name: "failure: contract metadata entry does not exist",
			env: cldf.Environment{
				DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
			},
			input:   DeleteContractMetadataChangesetInput{ContractMetadataKeys: []keys.ContractMetadataKey{{ChainSelector: contractMetadata2.ChainSelector, Address: contractMetadata2.Address}}},
			wantErr: fmt.Sprintf("contract metadata entry for chain selector %v and address %v does not exist", contractMetadata2.ChainSelector, contractMetadata2.Address),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := DeleteContractMetadataChangeset{}.VerifyPreconditions(tt.env, tt.input)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDeleteContractMetadataChangeset_Apply(t *testing.T) {
	t.Parallel()

	contractMetadata1 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value1"}
	contractMetadata2 := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 5678, Metadata: "value2"}

	tests := []struct {
		name            string
		env             cldf.Environment
		input           DeleteContractMetadataChangesetInput
		wantDeletedKeys []string
		wantErr         string
	}{
		{
			name: "success: stages two contract metadata entries for deletion",
			env: cldf.Environment{
				DataStore:        testDataStoreWithContractMetadata(t, contractMetadata1, contractMetadata2).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []keys.ContractMetadataKey{{ChainSelector: contractMetadata1.ChainSelector, Address: contractMetadata1.Address}, {ChainSelector: contractMetadata2.ChainSelector, Address: contractMetadata2.Address}},
			},
			wantDeletedKeys: []string{contractMetadata1.Key().String(), contractMetadata2.Key().String()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DeleteContractMetadataChangeset{}.Apply(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Len(t, got.Reports, 1)
				memDS := got.DataStore.(*cldfdatastore.MemoryDataStore)
				require.ElementsMatch(t, tt.wantDeletedKeys, memDS.ContractMetadataStore.DeletedRemoteKeys)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDeleteContractMetadataChangeset_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	contractMetadata1 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value1"}
	contractMetadata2 := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 5678, Metadata: "value2"}

	raw := `{"contractMetadataKeys":[{"chainSelector":1234,"address":"0x01"},{"chainSelector":5678,"address":"0x02"}]}`

	var got DeleteContractMetadataChangesetInput
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Equal(t,
		[]keys.ContractMetadataKey{{ChainSelector: 1234, Address: "0x01"}, {ChainSelector: 5678, Address: "0x02"}},
		got.ContractMetadataKeys,
	)

	env := cldf.Environment{
		DataStore:        testDataStoreWithContractMetadata(t, contractMetadata1, contractMetadata2).Seal(),
		OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
	}

	require.NoError(t, DeleteContractMetadataChangeset{}.VerifyPreconditions(env, got))

	out, err := DeleteContractMetadataChangeset{}.Apply(env, got)
	require.NoError(t, err)
	memDS := out.DataStore.(*cldfdatastore.MemoryDataStore)
	require.ElementsMatch(t,
		[]string{contractMetadata1.Key().String(), contractMetadata2.Key().String()},
		memDS.ContractMetadataStore.DeletedRemoteKeys,
	)
}
