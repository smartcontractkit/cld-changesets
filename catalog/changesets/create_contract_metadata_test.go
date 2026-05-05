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

func TestCreateContractMetadataChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	contractMetadata1 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value1"}
	contractMetadata2 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   CreateContractMetadataChangesetInput
		wantErr string
	}{
		{
			name: "success: valid preconditions",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: CreateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: CreateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{{}},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no contract metadata given",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: CreateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{},
			},
			wantErr: "missing contract metadata input",
		},
		{
			name: "failure: duplicate entries",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: CreateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1, contractMetadata2},
			},
			wantErr: "duplicate contract metadata entries found in input",
		},
		{
			name: "failure: contract metadata already exists",
			env: cldf.Environment{DataStore: func() cldfdatastore.DataStore {
				ds := cldfdatastore.NewMemoryDataStore()
				err := ds.ContractMetadata().Add(contractMetadata1)
				require.NoError(t, err)

				return ds.Seal()
			}()},
			input: CreateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1},
			},
			wantErr: "contract metadata for chain selector 1234 and address 0x01 already exists",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := CreateContractMetadataChangeset{}.VerifyPreconditions(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestCreateContractMetadataChangeset_Apply(t *testing.T) {
	t.Parallel()

	contractMetadata1 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value1"}
	contractMetadata2 := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 1234, Metadata: "value2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   CreateContractMetadataChangesetInput
		want    cldf.ChangesetOutput
		wantErr string
	}{
		{
			name: "success: adds two entries to contract metadata",
			env: cldf.Environment{
				DataStore:        testDataStoreWithContractMetadata(t).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: CreateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1, contractMetadata2},
			},
			want: cldf.ChangesetOutput{
				DataStore: testDataStoreWithContractMetadata(t, contractMetadata1, contractMetadata2),
				Reports: []cldfoperations.Report[any, any]{{
					Def: cldfoperations.Definition{
						ID:          "catalog-create-contract-metadata",
						Version:     semver.MustParse("1.0.0"),
						Description: "Add contract metadata entries to the Catalog service",
					},
					Input: operations.CreateContractMetadataInput{
						ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1, contractMetadata2},
					},
					Output: operations.CreateContractMetadataOutput{
						DataStore: testDataStoreWithContractMetadata(t, contractMetadata1, contractMetadata2),
					},
				}},
			},
		},
		{
			name: "failure: fails to add second entry",
			env: cldf.Environment{
				DataStore:        testDataStoreWithContractMetadata(t, contractMetadata2).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: CreateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1, contractMetadata2},
			},
			wantErr: "failed to create contract metadata entry 1 in catalog store: " +
				"a contract metadata record with the supplied key already exists",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CreateContractMetadataChangeset{}.Apply(tt.env, tt.input)

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

func testDataStoreWithContractMetadata(
	t *testing.T, metadata ...cldfdatastore.ContractMetadata,
) cldfdatastore.MutableDataStore {
	t.Helper()

	ds := cldfdatastore.NewMemoryDataStore()
	for _, m := range metadata {
		err := ds.ContractMetadata().Add(m)
		require.NoError(t, err)
	}

	return ds
}
