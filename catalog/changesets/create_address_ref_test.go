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

func TestCreateAddressRefChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef1 := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	addressRef2 := cldfdatastore.AddressRef{Address: "0x02", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   CreateAddressRefChangesetInput
		wantErr string
	}{
		{
			name: "success: valid preconditions",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: CreateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: CreateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no address refs given",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: CreateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{},
			},
			wantErr: "missing address refs input",
		},
		{
			name: "failure: duplicate entries",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: CreateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1, addressRef2},
			},
			wantErr: "duplicate address ref entries found in input",
		},
		{
			name: "failure: address ref already exists",
			env: cldf.Environment{DataStore: func() cldfdatastore.DataStore {
				ds := cldfdatastore.NewMemoryDataStore()
				err := ds.Addresses().Add(addressRef1)
				require.NoError(t, err)

				return ds.Seal()
			}()},
			input: CreateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1},
			},
			wantErr: "address ref for chain selector 1234, type MyContract, version 1.0.0 and qualifier \"q1\" already exists",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := CreateAddressRefChangeset{}.VerifyPreconditions(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestCreateAddressRefChangeset_Apply(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef1 := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	addressRef2 := cldfdatastore.AddressRef{Address: "0x02", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q2"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   CreateAddressRefChangesetInput
		want    cldf.ChangesetOutput
		wantErr string
	}{
		{
			name: "success: adds two entries to address refs",
			env: cldf.Environment{
				DataStore:        testDataStoreWithAddressRefs(t).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: CreateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1, addressRef2},
			},
			want: cldf.ChangesetOutput{
				DataStore: testDataStoreWithAddressRefs(t, addressRef1, addressRef2),
				Reports: []cldfoperations.Report[any, any]{{
					Def: cldfoperations.Definition{
						ID:          "catalog-create-address-ref",
						Version:     semver.MustParse("1.0.0"),
						Description: "Add address ref entries to the Catalog service",
					},
					Input: operations.CreateAddressRefInput{
						AddressRefs: []cldfdatastore.AddressRef{addressRef1, addressRef2},
					},
					Output: operations.CreateAddressRefOutput{
						DataStore: testDataStoreWithAddressRefs(t, addressRef1, addressRef2),
					},
				}},
			},
		},
		{
			name: "failure: fails to add second entry",
			env: cldf.Environment{
				DataStore:        testDataStoreWithAddressRefs(t, addressRef2).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: CreateAddressRefChangesetInput{
				AddressRefs: []cldfdatastore.AddressRef{addressRef1, addressRef2},
			},
			wantErr: "failed to create address ref entry 1 in catalog store: " +
				"an address ref with the supplied key already exists",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CreateAddressRefChangeset{}.Apply(tt.env, tt.input)

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

// ----- helpers -----

func testDataStoreWithAddressRefs(
	t *testing.T, addressRefs ...cldfdatastore.AddressRef,
) cldfdatastore.MutableDataStore {
	t.Helper()

	ds := cldfdatastore.NewMemoryDataStore()
	for _, ar := range addressRefs {
		err := ds.Addresses().Add(ar)
		require.NoError(t, err)
	}

	return ds
}
