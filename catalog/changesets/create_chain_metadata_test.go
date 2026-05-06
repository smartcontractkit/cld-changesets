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

func TestCreateChainMetadataChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	chainMetadata1 := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "value1"}
	chainMetadata2 := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "value2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   CreateChainMetadataChangesetInput
		wantErr string
	}{
		{
			name: "success: valid preconditions",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: CreateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: CreateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{{}},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no chain metadata given",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: CreateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{},
			},
			wantErr: "missing chain metadata input",
		},
		{
			name: "failure: duplicate entries",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: CreateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1, chainMetadata2},
			},
			wantErr: "duplicate chain metadata entries found in input",
		},
		{
			name: "failure: chain metadata already exists",
			env: cldf.Environment{DataStore: func() cldfdatastore.DataStore {
				ds := cldfdatastore.NewMemoryDataStore()
				err := ds.ChainMetadata().Add(chainMetadata1)
				require.NoError(t, err)

				return ds.Seal()
			}()},
			input: CreateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1},
			},
			wantErr: "chain metadata for chain selector 1234 already exists",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := CreateChainMetadataChangeset{}.VerifyPreconditions(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestCreateChainMetadataChangeset_Apply(t *testing.T) {
	t.Parallel()

	chainMetadata1 := cldfdatastore.ChainMetadata{ChainSelector: 1234, Metadata: "value1"}
	chainMetadata2 := cldfdatastore.ChainMetadata{ChainSelector: 5678, Metadata: "value2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   CreateChainMetadataChangesetInput
		want    cldf.ChangesetOutput
		wantErr string
	}{
		{
			name: "success: adds two entries to chain metadata",
			env: cldf.Environment{
				DataStore:        testDataStoreWithChainMetadata(t).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: CreateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1, chainMetadata2},
			},
			want: cldf.ChangesetOutput{
				DataStore: testDataStoreWithChainMetadata(t, chainMetadata1, chainMetadata2),
				Reports: []cldfoperations.Report[any, any]{{
					Def: cldfoperations.Definition{
						ID:          "catalog-create-chain-metadata",
						Version:     semver.MustParse("1.0.0"),
						Description: "Add chain metadata entries to the Catalog service",
					},
					Input: operations.CreateChainMetadataInput{
						ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1, chainMetadata2},
					},
					Output: operations.CreateChainMetadataOutput{
						DataStore: testDataStoreWithChainMetadata(t, chainMetadata1, chainMetadata2),
					},
				}},
			},
		},
		{
			name: "failure: fails to add second entry",
			env: cldf.Environment{
				DataStore:        testDataStoreWithChainMetadata(t, chainMetadata2).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: CreateChainMetadataChangesetInput{
				ChainMetadata: []cldfdatastore.ChainMetadata{chainMetadata1, chainMetadata2},
			},
			wantErr: "failed to create chain metadata entry 1 in catalog store: " +
				"a chain metadata record with the supplied key already exists",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CreateChainMetadataChangeset{}.Apply(tt.env, tt.input)

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

// ----- helpers -----

func testDataStoreWithChainMetadata(
	t *testing.T, metadata ...cldfdatastore.ChainMetadata,
) cldfdatastore.MutableDataStore {
	t.Helper()

	ds := cldfdatastore.NewMemoryDataStore()
	for _, m := range metadata {
		err := ds.ChainMetadata().Add(m)
		require.NoError(t, err)
	}

	return ds
}
