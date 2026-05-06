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

func TestUpdateAddressRefChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef1 := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	addressRef2 := cldfdatastore.AddressRef{Address: "0x02", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   UpdateAddressRefChangesetInput
		wantErr string
	}{
		{
			name: "success: valid preconditions",
			env: cldf.Environment{DataStore: func() cldfdatastore.DataStore {
				ds := cldfdatastore.NewMemoryDataStore()
				err := ds.Addresses().Add(addressRef1)
				require.NoError(t, err)

				return ds.Seal()
			}()},
			input: UpdateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: UpdateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no address refs given",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: UpdateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{},
			},
			wantErr: "missing address refs input",
		},
		{
			name: "failure: duplicate entries",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: UpdateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1, addressRef2},
			},
			wantErr: "duplicate address ref entries found in input",
		},
		{
			name: "failure: address ref does not exist",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: UpdateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1},
			},
			wantErr: "address ref for chain selector 1234, type MyContract, version 1.0.0 and qualifier \"q1\" does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := UpdateAddressRefChangeset{}.VerifyPreconditions(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestUpdateAddressRefChangeset_Apply(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef1 := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	addressRef2 := cldfdatastore.AddressRef{Address: "0x02", ChainSelector: 5678, Type: "OtherContract", Version: version, Qualifier: "q2"}
	addressRef1Updated := cldfdatastore.AddressRef{Address: "0x99", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	addressRef2Updated := cldfdatastore.AddressRef{Address: "0x88", ChainSelector: 5678, Type: "OtherContract", Version: version, Qualifier: "q2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   UpdateAddressRefChangesetInput
		want    cldf.ChangesetOutput
		wantErr string
	}{
		{
			name: "success: updates two entries in address refs",
			env: cldf.Environment{
				DataStore:        testDataStoreWithAddressRefs(t, addressRef1, addressRef2).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: UpdateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1Updated, addressRef2Updated},
			},
			want: cldf.ChangesetOutput{
				DataStore: testDataStoreWithAddressRefs(t, addressRef1Updated, addressRef2Updated),
				Reports: []cldfoperations.Report[any, any]{{
					Def: cldfoperations.Definition{
						ID:          "catalog-update-address-ref",
						Version:     semver.MustParse("1.0.0"),
						Description: "Update address ref entries in the Catalog service",
					},
					Input: operations.UpdateAddressRefInput{
						AddressRefs: []cldfdatastore.AddressRef{addressRef1Updated, addressRef2Updated},
					},
					Output: operations.UpdateAddressRefOutput{
						DataStore: testDataStoreWithAddressRefs(t, addressRef1Updated, addressRef2Updated),
					},
				}},
			},
		},
		{
			name: "failure: fails to update entry that does not exist",
			env: cldf.Environment{
				DataStore:        testDataStoreWithAddressRefs(t).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: UpdateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1Updated},
			},
			wantErr: "failed to update address ref entry 0 in catalog store: " +
				"no such address ref can be found for the provided key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := UpdateAddressRefChangeset{}.Apply(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Empty(t,
					cmp.Diff(tt.want, got,
						cmpopts.IgnoreFields(cldfoperations.Report[any, any]{}, "ID", "Timestamp"),
						cmpopts.IgnoreUnexported(cldfdatastore.MemoryAddressRefStore{}, cldfdatastore.MemoryChainMetadataStore{},
							cldfdatastore.MemoryContractMetadataStore{}, cldfdatastore.MemoryEnvMetadataStore{},
							cldfdatastore.LabelSet{})))
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
