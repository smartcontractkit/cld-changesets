package changesets

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfoperations "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/catalog/operations"
)

func TestUpdateChainMetadataChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	chainMetadata1 := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "value1"}
	chainMetadata2 := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "value2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   UpdateChainMetadataChangesetInput
		wantErr string
	}{
		{
			name: "success: valid preconditions",
			env: cldf.Environment{DataStore: func() cldfdatastore.DataStore {
				ds := cldfdatastore.NewMemoryDataStore()
				err := ds.ChainMetadata().Add(chainMetadata1)
				require.NoError(t, err)

				return ds.Seal()
			}()},
			input: UpdateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: UpdateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{{}},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no chain metadata given",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: UpdateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{},
			},
			wantErr: "missing chain metadata input",
		},
		{
			name: "failure: duplicate entries",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: UpdateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1, chainMetadata2},
			},
			wantErr: "duplicate chain metadata entries found in input",
		},
		{
			name: "failure: chain metadata does not exist",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: UpdateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1},
			},
			wantErr: "chain metadata for chain selector 1234 does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := UpdateChainMetadataChangeset{}.VerifyPreconditions(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestUpdateChainMetadataChangeset_Apply(t *testing.T) {
	t.Parallel()

	chainMetadata1 := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "value1"}
	chainMetadata2 := cldfdatastore.ChainMetadata{ChainSelector: 5678, Metadata: "value2"}
	chainMetadata1Updated := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "updated-value1"}
	chainMetadata2Updated := cldfdatastore.ChainMetadata{ChainSelector: 5678, Metadata: "updated-value2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   UpdateChainMetadataChangesetInput
		want    cldf.ChangesetOutput
		wantErr string
	}{
		{
			name: "success: updates two entries in chain metadata",
			env: cldf.Environment{
				DataStore:        testDataStoreWithChainMetadata(t, chainMetadata1, chainMetadata2).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: UpdateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1Updated, chainMetadata2Updated},
			},
			want: cldf.ChangesetOutput{
				DataStore: testDataStoreWithChainMetadata(t, chainMetadata1Updated, chainMetadata2Updated),
				Reports: []cldfoperations.Report[any, any]{{
					Def: cldfoperations.Definition{
						ID:          "catalog-update-chain-metadata",
						Version:     semver.MustParse("1.0.0"),
						Description: "Update chain metadata entries in the Catalog service",
					},
					Input: operations.UpdateChainMetadataInput{
						ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1Updated, chainMetadata2Updated},
					},
					Output: operations.UpdateChainMetadataOutput{
						DataStore: testDataStoreWithChainMetadata(t, chainMetadata1Updated, chainMetadata2Updated),
					},
				}},
			},
		},
		{
			name: "failure: fails to update entry that does not exist",
			env: cldf.Environment{
				DataStore:        testDataStoreWithChainMetadata(t, chainMetadata1).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: UpdateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1Updated, chainMetadata2Updated},
			},
			wantErr: "failed to update chain metadata entry 1 in catalog store: " +
				"no chain metadata record can be found for the provided key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := UpdateChainMetadataChangeset{}.Apply(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Empty(t,
					cmp.Diff(tt.want, got,
						cmpopts.IgnoreFields(cldfoperations.Report[any, any]{}, "ID", "Timestamp"),
						cmpopts.IgnoreUnexported(cldfdatastore.MemoryAddressRefStore{}, cldfdatastore.MemoryChainMetadataStore{},
							cldfdatastore.MemoryContractMetadataStore{}, cldfdatastore.MemoryEnvMetadataStore{})))
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
