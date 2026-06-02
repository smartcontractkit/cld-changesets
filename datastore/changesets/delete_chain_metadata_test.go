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

func TestDeleteChainMetadataChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	chainMetadata1 := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "value1"}
	chainMetadata2 := cldfdatastore.ChainMetadata{ChainSelector: 5678, Metadata: "value2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   DeleteChainMetadataChangesetInput
		wantErr string
	}{
		{
			name: "success: valid preconditions",
			env: cldf.Environment{
				DataStore: testDataStoreWithChainMetadata(t, chainMetadata1, chainMetadata2).Seal(),
			},
			input: DeleteChainMetadataChangesetInput{
				ChainMetadataKeys: []cldfdatastore.ChainMetadataKey{chainMetadata1.Key(), chainMetadata2.Key()},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: DeleteChainMetadataChangesetInput{
				ChainMetadataKeys: []cldfdatastore.ChainMetadataKey{chainMetadata1.Key()},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no chain metadata keys given",
			env: cldf.Environment{
				DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
			},
			input:   DeleteChainMetadataChangesetInput{ChainMetadataKeys: []cldfdatastore.ChainMetadataKey{}},
			wantErr: "missing chain metadata keys input",
		},
		{
			name: "failure: chain metadata entry does not exist",
			env: cldf.Environment{
				DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
			},
			input:   DeleteChainMetadataChangesetInput{ChainMetadataKeys: []cldfdatastore.ChainMetadataKey{chainMetadata2.Key()}},
			wantErr: fmt.Sprintf("chain metadata entry for chain selector %v does not exist", chainMetadata2.ChainSelector),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := DeleteChainMetadataChangeset{}.VerifyPreconditions(tt.env, tt.input)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDeleteChainMetadataChangeset_Apply(t *testing.T) {
	t.Parallel()

	chainMetadata1 := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "value1"}
	chainMetadata2 := cldfdatastore.ChainMetadata{ChainSelector: 5678, Metadata: "value2"}

	tests := []struct {
		name            string
		env             cldf.Environment
		input           DeleteChainMetadataChangesetInput
		wantDeletedKeys []string
		wantErr         string
	}{
		{
			name: "success: stages two chain metadata entries for deletion",
			env: cldf.Environment{
				DataStore:        testDataStoreWithChainMetadata(t, chainMetadata1, chainMetadata2).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: DeleteChainMetadataChangesetInput{
				ChainMetadataKeys: []cldfdatastore.ChainMetadataKey{chainMetadata1.Key(), chainMetadata2.Key()},
			},
			wantDeletedKeys: []string{chainMetadata1.Key().String(), chainMetadata2.Key().String()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DeleteChainMetadataChangeset{}.Apply(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Len(t, got.Reports, 1)
				memDS := got.DataStore.(*cldfdatastore.MemoryDataStore)
				require.ElementsMatch(t, tt.wantDeletedKeys, memDS.ChainMetadataStore.DeletedRemoteKeys)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
