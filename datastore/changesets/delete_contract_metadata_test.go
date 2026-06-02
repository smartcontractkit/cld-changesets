package changesets

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfoperations "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
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
				ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{contractMetadata1.Key(), contractMetadata2.Key()},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{contractMetadata1.Key()},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no contract metadata keys given",
			env: cldf.Environment{
				DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
			},
			input:   DeleteContractMetadataChangesetInput{ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{}},
			wantErr: "missing contract metadata keys input",
		},
		{
			name: "failure: contract metadata entry does not exist",
			env: cldf.Environment{
				DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
			},
			input:   DeleteContractMetadataChangesetInput{ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{contractMetadata2.Key()}},
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
				ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{contractMetadata1.Key(), contractMetadata2.Key()},
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
