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
				ChainMetadataKeys: []keys.ChainMetadataKey{{ChainSelector: chainMetadata1.ChainSelector}, {ChainSelector: chainMetadata2.ChainSelector}},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: DeleteChainMetadataChangesetInput{
				ChainMetadataKeys: []keys.ChainMetadataKey{{ChainSelector: chainMetadata1.ChainSelector}},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no chain metadata keys given",
			env: cldf.Environment{
				DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
			},
			input:   DeleteChainMetadataChangesetInput{ChainMetadataKeys: []keys.ChainMetadataKey{}},
			wantErr: "missing chain metadata keys input",
		},
		{
			name: "failure: chain metadata entry does not exist",
			env: cldf.Environment{
				DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
			},
			input:   DeleteChainMetadataChangesetInput{ChainMetadataKeys: []keys.ChainMetadataKey{{ChainSelector: chainMetadata2.ChainSelector}}},
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
				ChainMetadataKeys: []keys.ChainMetadataKey{{ChainSelector: chainMetadata1.ChainSelector}, {ChainSelector: chainMetadata2.ChainSelector}},
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

func TestDeleteChainMetadataChangeset_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	chainMetadata1 := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "value1"}
	chainMetadata2 := cldfdatastore.ChainMetadata{ChainSelector: 5678, Metadata: "value2"}

	raw := `{"chainMetadataKeys":[{"chainSelector":1234},{"chainSelector":5678}]}`

	var got DeleteChainMetadataChangesetInput
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Equal(t,
		[]keys.ChainMetadataKey{{ChainSelector: 1234}, {ChainSelector: 5678}},
		got.ChainMetadataKeys,
	)

	env := cldf.Environment{
		DataStore:        testDataStoreWithChainMetadata(t, chainMetadata1, chainMetadata2).Seal(),
		OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
	}

	require.NoError(t, DeleteChainMetadataChangeset{}.VerifyPreconditions(env, got))

	out, err := DeleteChainMetadataChangeset{}.Apply(env, got)
	require.NoError(t, err)
	memDS := out.DataStore.(*cldfdatastore.MemoryDataStore)
	require.ElementsMatch(t,
		[]string{chainMetadata1.Key().String(), chainMetadata2.Key().String()},
		memDS.ChainMetadataStore.DeletedRemoteKeys,
	)
}
