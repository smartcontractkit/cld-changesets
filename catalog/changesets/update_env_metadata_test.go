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

func TestUpdateEnvMetadataChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	envMetadata := cldfdatastore.EnvMetadata{Metadata: "value"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   UpdateEnvMetadataChangesetInput
		wantErr string
	}{
		{
			name: "success: valid preconditions",
			env: cldf.Environment{DataStore: func() cldfdatastore.DataStore {
				ds := cldfdatastore.NewMemoryDataStore()
				err := ds.EnvMetadata().Set(envMetadata)
				require.NoError(t, err)

				return ds.Seal()
			}()},
			input: UpdateEnvMetadataChangesetInput{
				EnvMetadata: envMetadata,
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: UpdateEnvMetadataChangesetInput{
				EnvMetadata: envMetadata,
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no env metadata given",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: UpdateEnvMetadataChangesetInput{
				EnvMetadata: cldfdatastore.EnvMetadata{},
			},
			wantErr: "missing env metadata input",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := UpdateEnvMetadataChangeset{}.VerifyPreconditions(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestUpdateEnvMetadataChangeset_Apply(t *testing.T) {
	t.Parallel()

	envMetadata := cldfdatastore.EnvMetadata{Metadata: "value"}
	envMetadataUpdated := cldfdatastore.EnvMetadata{Metadata: "updated-value"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   UpdateEnvMetadataChangesetInput
		want    cldf.ChangesetOutput
		wantErr string
	}{
		{
			name: "success: updates env metadata",
			env: cldf.Environment{
				DataStore:        testDataStoreWithEnvMetadata(t, envMetadata).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: UpdateEnvMetadataChangesetInput{
				EnvMetadata: envMetadataUpdated,
			},
			want: cldf.ChangesetOutput{
				DataStore: testDataStoreWithEnvMetadata(t, envMetadataUpdated),
				Reports: []cldfoperations.Report[any, any]{{
					Def: cldfoperations.Definition{
						ID:          "catalog-update-env-metadata",
						Version:     semver.MustParse("1.0.0"),
						Description: "Update env metadata entries in the Catalog service",
					},
					Input: operations.UpdateEnvMetadataInput{
						EnvMetadata: envMetadataUpdated,
					},
					Output: operations.UpdateEnvMetadataOutput{
						DataStore: testDataStoreWithEnvMetadata(t, envMetadataUpdated),
					},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := UpdateEnvMetadataChangeset{}.Apply(tt.env, tt.input)

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

func testDataStoreWithEnvMetadata(
	t *testing.T, metadata cldfdatastore.EnvMetadata,
) cldfdatastore.MutableDataStore {
	t.Helper()

	ds := cldfdatastore.NewMemoryDataStore()
	err := ds.EnvMetadata().Set(metadata)
	require.NoError(t, err)

	return ds
}
