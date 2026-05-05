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

func TestDeleteContractMetadataChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	key1 := cldfdatastore.NewContractMetadataKey(1234, "0x01")
	contractMetadata1 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value1"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   DeleteContractMetadataChangesetInput
		wantErr string
	}{
		{
			name: "success: valid preconditions",
			env: cldf.Environment{DataStore: func() cldfdatastore.DataStore {
				ds := cldfdatastore.NewMemoryDataStore()
				err := ds.ContractMetadata().Add(contractMetadata1)
				require.NoError(t, err)

				return ds.Seal()
			}()},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{key1},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{key1},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no contract metadata keys given",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{},
			},
			wantErr: "missing contract metadata keys input",
		},
		{
			name: "failure: duplicate keys",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{key1, key1},
			},
			wantErr: "duplicate contract metadata keys found in input",
		},
		{
			name: "failure: contract metadata does not exist",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{key1},
			},
			wantErr: "contract metadata for chain selector 1234 and address 0x01 does not exist",
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
	contractMetadata2 := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 1234, Metadata: "value2"}

	key1 := cldfdatastore.NewContractMetadataKey(1234, "0x01")
	key2 := cldfdatastore.NewContractMetadataKey(1234, "0x02")

	tests := []struct {
		name    string
		env     cldf.Environment
		input   DeleteContractMetadataChangesetInput
		want    cldf.ChangesetOutput
		wantErr string
	}{
		{
			name: "success: deletes two entries from contract metadata",
			env: cldf.Environment{
				DataStore:        testDataStoreWithContractMetadata(t, contractMetadata1, contractMetadata2).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{key1, key2},
			},
			want: cldf.ChangesetOutput{
				DataStore: testDataStoreWithContractMetadata(t),
				Reports: []cldfoperations.Report[any, any]{{
					Def: cldfoperations.Definition{
						ID:          "catalog-delete-contract-metadata",
						Version:     semver.MustParse("1.0.0"),
						Description: "Delete contract metadata entries from the Catalog service",
					},
					Input: operations.DeleteContractMetadataInput{
						ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{key1, key2},
					},
					Output: operations.DeleteContractMetadataOutput{
						DataStore: testDataStoreWithContractMetadata(t),
					},
				}},
			},
		},
		{
			name: "failure: fails to delete entry that does not exist",
			env: cldf.Environment{
				DataStore:        testDataStoreWithContractMetadata(t, contractMetadata1).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: DeleteContractMetadataChangesetInput{
				ContractMetadataKeys: []cldfdatastore.ContractMetadataKey{key1, key2},
			},
			wantErr: "failed to delete contract metadata entry 1 from catalog store: " +
				"no contract metadata record can be found for the provided key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DeleteContractMetadataChangeset{}.Apply(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Empty(t,
					cmp.Diff(tt.want, got,
						contractMetadataKeyComparer,
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

var contractMetadataKeyComparer = cmp.Comparer(func(x, y cldfdatastore.ContractMetadataKey) bool {
	return x.Equals(x)
})
