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

func TestUpdateContractMetadataChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	contractMetadata1 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value1"}
	contractMetadata2 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   UpdateContractMetadataChangesetInput
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
			input: UpdateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: UpdateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{{}},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no contract metadata given",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: UpdateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{},
			},
			wantErr: "missing contract metadata input",
		},
		{
			name: "failure: duplicate entries",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: UpdateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1, contractMetadata2},
			},
			wantErr: "duplicate contract metadata entries found in input",
		},
		{
			name: "failure: contract metadata does not exist",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: UpdateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1},
			},
			wantErr: "contract metadata for chain selector 1234 and address 0x01 does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := UpdateContractMetadataChangeset{}.VerifyPreconditions(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestUpdateContractMetadataChangeset_Apply(t *testing.T) {
	t.Parallel()

	contractMetadata1 := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "value1"}
	contractMetadata2 := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 1234, Metadata: "value2"}
	contractMetadata1Updated := cldfdatastore.ContractMetadata{Address: "0x01", ChainSelector: 1234, Metadata: "updated-value1"}
	contractMetadata2Updated := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 1234, Metadata: "updated-value2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   UpdateContractMetadataChangesetInput
		want    cldf.ChangesetOutput
		wantErr string
	}{
		{
			name: "success: updates two entries in contract metadata",
			env: cldf.Environment{
				DataStore:        testDataStoreWithContractMetadata(t, contractMetadata1, contractMetadata2).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: UpdateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1Updated, contractMetadata2Updated},
			},
			want: cldf.ChangesetOutput{
				DataStore: testDataStoreWithContractMetadata(t, contractMetadata1Updated, contractMetadata2Updated),
				Reports: []cldfoperations.Report[any, any]{{
					Def: cldfoperations.Definition{
						ID:          "catalog-update-contract-metadata",
						Version:     semver.MustParse("1.0.0"),
						Description: "Update contract metadata entries in the Catalog service",
					},
					Input: operations.UpdateContractMetadataInput{
						ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1Updated, contractMetadata2Updated},
					},
					Output: operations.UpdateContractMetadataOutput{
						DataStore: testDataStoreWithContractMetadata(t, contractMetadata1Updated, contractMetadata2Updated),
					},
				}},
			},
		},
		{
			name: "failure: fails to update entry that does not exist",
			env: cldf.Environment{
				DataStore:        testDataStoreWithContractMetadata(t, contractMetadata1).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: UpdateContractMetadataChangesetInput{
				ContractMetadata: []cldfdatastore.ContractMetadata{contractMetadata1Updated, contractMetadata2Updated},
			},
			wantErr: "failed to update contract metadata entry 1 in catalog store: " +
				"no contract metadata record can be found for the provided key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := UpdateContractMetadataChangeset{}.Apply(tt.env, tt.input)

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
