package changesets

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Masterminds/semver/v3"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfoperations "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/datastore/internal/keys"
)

func TestDeleteResourcesChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	contractMetadata := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 1234, Metadata: "value"}
	chainMetadata := cldfdatastore.ChainMetadata{ChainSelector: 5678, Metadata: "chain-value"}

	fullDS := func() cldfdatastore.DataStore {
		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(addressRef))
		require.NoError(t, ds.ContractMetadata().Add(contractMetadata))
		require.NoError(t, ds.ChainMetadata().Add(chainMetadata))

		return ds.Seal()
	}()

	tests := []struct {
		name    string
		env     cldf.Environment
		input   DeleteResourcesChangesetInput
		wantErr string
	}{
		{
			name: "success: all three resource types provided",
			env:  cldf.Environment{DataStore: fullDS},
			input: DeleteResourcesChangesetInput{
				AddressRefKeys:       []keys.AddressRefKey{testAddressRefKey(addressRef)},
				ContractMetadataKeys: []keys.ContractMetadataKey{testContractMetadataKey(contractMetadata)},
				ChainMetadataKeys:    []keys.ChainMetadataKey{testChainMetadataKey(chainMetadata)},
			},
		},
		{
			name: "success: only address ref keys",
			env:  cldf.Environment{DataStore: fullDS},
			input: DeleteResourcesChangesetInput{
				AddressRefKeys: []keys.AddressRefKey{testAddressRefKey(addressRef)},
			},
		},
		{
			name: "success: only contract metadata keys",
			env:  cldf.Environment{DataStore: fullDS},
			input: DeleteResourcesChangesetInput{
				ContractMetadataKeys: []keys.ContractMetadataKey{testContractMetadataKey(contractMetadata)},
			},
		},
		{
			name: "success: only chain metadata keys",
			env:  cldf.Environment{DataStore: fullDS},
			input: DeleteResourcesChangesetInput{
				ChainMetadataKeys: []keys.ChainMetadataKey{testChainMetadataKey(chainMetadata)},
			},
		},
		{
			name:    "failure: missing datastore",
			env:     cldf.Environment{},
			input:   DeleteResourcesChangesetInput{AddressRefKeys: []keys.AddressRefKey{testAddressRefKey(addressRef)}},
			wantErr: "missing datastore in environment",
		},
		{
			name:    "failure: all key slices empty",
			env:     cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input:   DeleteResourcesChangesetInput{},
			wantErr: "at least one resource key slice must be non-empty",
		},
		{
			name: "failure: address ref entry does not exist",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: DeleteResourcesChangesetInput{
				AddressRefKeys: []keys.AddressRefKey{testAddressRefKey(addressRef)},
			},
			wantErr: fmt.Sprintf("address ref entry for chain selector %v, type %v, version %v and qualifier %q does not exist",
				addressRef.ChainSelector, addressRef.Type, addressRef.Version, addressRef.Qualifier),
		},
		{
			name: "failure: contract metadata entry does not exist",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: DeleteResourcesChangesetInput{
				ContractMetadataKeys: []keys.ContractMetadataKey{testContractMetadataKey(contractMetadata)},
			},
			wantErr: fmt.Sprintf("contract metadata entry for chain selector %v and address %v does not exist",
				contractMetadata.ChainSelector, contractMetadata.Address),
		},
		{
			name: "failure: chain metadata entry does not exist",
			env:  cldf.Environment{DataStore: cldfdatastore.NewMemoryDataStore().Seal()},
			input: DeleteResourcesChangesetInput{
				ChainMetadataKeys: []keys.ChainMetadataKey{testChainMetadataKey(chainMetadata)},
			},
			wantErr: fmt.Sprintf("chain metadata entry for chain selector %v does not exist", chainMetadata.ChainSelector),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := DeleteResourcesChangeset{}.VerifyPreconditions(tt.env, tt.input)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDeleteResourcesChangeset_Apply(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	contractMetadata := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 1234, Metadata: "value"}
	chainMetadata := cldfdatastore.ChainMetadata{ChainSelector: 5678, Metadata: "chain-value"}

	fullDS := func() cldfdatastore.DataStore {
		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(addressRef))
		require.NoError(t, ds.ContractMetadata().Add(contractMetadata))
		require.NoError(t, ds.ChainMetadata().Add(chainMetadata))

		return ds.Seal()
	}()

	bundle := cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter())

	tests := []struct {
		name                    string
		input                   DeleteResourcesChangesetInput
		wantReportCount         int
		wantAddressRefDeleted   []string
		wantContractMetaDeleted []string
		wantChainMetaDeleted    []string
	}{
		{
			name: "success: deletes only address refs",
			input: DeleteResourcesChangesetInput{
				AddressRefKeys: []keys.AddressRefKey{testAddressRefKey(addressRef)},
			},
			wantReportCount:         4,
			wantAddressRefDeleted:   []string{addressRef.Key().String()},
			wantContractMetaDeleted: []string{},
			wantChainMetaDeleted:    []string{},
		},
		{
			name: "success: deletes mixed resources",
			input: DeleteResourcesChangesetInput{
				AddressRefKeys:       []keys.AddressRefKey{testAddressRefKey(addressRef)},
				ContractMetadataKeys: []keys.ContractMetadataKey{testContractMetadataKey(contractMetadata)},
				ChainMetadataKeys:    []keys.ChainMetadataKey{testChainMetadataKey(chainMetadata)},
			},
			wantReportCount:         4,
			wantAddressRefDeleted:   []string{addressRef.Key().String()},
			wantContractMetaDeleted: []string{contractMetadata.Key().String()},
			wantChainMetaDeleted:    []string{chainMetadata.Key().String()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := cldf.Environment{
				DataStore:        fullDS,
				OperationsBundle: bundle,
			}

			got, err := DeleteResourcesChangeset{}.Apply(env, tt.input)
			require.NoError(t, err)
			require.Len(t, got.Reports, tt.wantReportCount)

			memDS := got.DataStore.(*cldfdatastore.MemoryDataStore)
			require.ElementsMatch(t, tt.wantAddressRefDeleted, memDS.AddressRefStore.DeletedRemoteKeys)
			require.ElementsMatch(t, tt.wantContractMetaDeleted, memDS.ContractMetadataStore.DeletedRemoteKeys)
			require.ElementsMatch(t, tt.wantChainMetaDeleted, memDS.ChainMetadataStore.DeletedRemoteKeys)
		})
	}
}

func testAddressRefKey(addressRef cldfdatastore.AddressRef) keys.AddressRefKey {
	return keys.AddressRefKey{
		ChainSelector: addressRef.ChainSelector,
		Type:          addressRef.Type,
		Version:       addressRef.Version,
		Qualifier:     addressRef.Qualifier,
	}
}

func testContractMetadataKey(contractMetadata cldfdatastore.ContractMetadata) keys.ContractMetadataKey {
	return keys.ContractMetadataKey{
		ChainSelector: contractMetadata.ChainSelector,
		Address:       contractMetadata.Address,
	}
}

func testChainMetadataKey(chainMetadata cldfdatastore.ChainMetadata) keys.ChainMetadataKey {
	return keys.ChainMetadataKey{ChainSelector: chainMetadata.ChainSelector}
}
