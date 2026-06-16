package sequences

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/datastore/internal/keys"
)

func TestDeleteResourcesSeq(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	contractMetadata := cldfdatastore.ContractMetadata{Address: "0x02", ChainSelector: 1234, Metadata: "value"}
	chainMetadata := cldfdatastore.ChainMetadata{ChainSelector: 5678, Metadata: "chain-value"}

	bundle := func(t *testing.T) cldfops.Bundle {
		t.Helper()
		return cldfops.NewBundle(t.Context, cldflogger.Test(t), cldfops.NewMemoryReporter())
	}

	tests := []struct {
		name                    string
		input                   DeleteResourcesSeqInput
		setup                   func(t *testing.T) cldfdatastore.DataStore
		wantAddressRefDeleted   []string
		wantContractMetaDeleted []string
		wantChainMetaDeleted    []string
		wantErr                 string
	}{
		{
			name: "success: deletes only address refs when other slices are empty",
			setup: func(t *testing.T) cldfdatastore.DataStore {
				t.Helper()
				return testDataStore(t, addressRef, contractMetadata, chainMetadata).Seal()
			},
			input: DeleteResourcesSeqInput{
				AddressRefKeys: []keys.AddressRefKey{testAddressRefKey(addressRef)},
			},
			wantAddressRefDeleted:   []string{addressRef.Key().String()},
			wantContractMetaDeleted: []string{},
			wantChainMetaDeleted:    []string{},
		},
		{
			name: "success: deletes only contract metadata when other slices are empty",
			setup: func(t *testing.T) cldfdatastore.DataStore {
				t.Helper()
				return testDataStore(t, addressRef, contractMetadata, chainMetadata).Seal()
			},
			input: DeleteResourcesSeqInput{
				ContractMetadataKeys: []keys.ContractMetadataKey{testContractMetadataKey(contractMetadata)},
			},
			wantAddressRefDeleted:   []string{},
			wantContractMetaDeleted: []string{contractMetadata.Key().String()},
			wantChainMetaDeleted:    []string{},
		},
		{
			name: "success: deletes only chain metadata when other slices are empty",
			setup: func(t *testing.T) cldfdatastore.DataStore {
				t.Helper()
				return testDataStore(t, addressRef, contractMetadata, chainMetadata).Seal()
			},
			input: DeleteResourcesSeqInput{
				ChainMetadataKeys: []keys.ChainMetadataKey{testChainMetadataKey(chainMetadata)},
			},
			wantAddressRefDeleted:   []string{},
			wantContractMetaDeleted: []string{},
			wantChainMetaDeleted:    []string{chainMetadata.Key().String()},
		},
		{
			name: "success: deletes mixed resources in a single invocation",
			setup: func(t *testing.T) cldfdatastore.DataStore {
				t.Helper()
				return testDataStore(t, addressRef, contractMetadata, chainMetadata).Seal()
			},
			input: DeleteResourcesSeqInput{
				AddressRefKeys:       []keys.AddressRefKey{testAddressRefKey(addressRef)},
				ContractMetadataKeys: []keys.ContractMetadataKey{testContractMetadataKey(contractMetadata)},
				ChainMetadataKeys:    []keys.ChainMetadataKey{testChainMetadataKey(chainMetadata)},
			},
			wantAddressRefDeleted:   []string{addressRef.Key().String()},
			wantContractMetaDeleted: []string{contractMetadata.Key().String()},
			wantChainMetaDeleted:    []string{chainMetadata.Key().String()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := DeleteResourcesSeqDeps{DataStore: tt.setup(t)}
			report, err := cldfops.ExecuteSequence(bundle(t), DeleteResourcesSeq, deps, tt.input)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			memDS := report.Output.DataStore.(*cldfdatastore.MemoryDataStore)
			require.ElementsMatch(t, tt.wantAddressRefDeleted, memDS.AddressRefStore.DeletedRemoteKeys)
			require.ElementsMatch(t, tt.wantContractMetaDeleted, memDS.ContractMetadataStore.DeletedRemoteKeys)
			require.ElementsMatch(t, tt.wantChainMetaDeleted, memDS.ChainMetadataStore.DeletedRemoteKeys)
		})
	}
}

// ----- helpers -----

func testDataStore(t *testing.T, addressRefs ...any) cldfdatastore.MutableDataStore {
	t.Helper()

	ds := cldfdatastore.NewMemoryDataStore()
	for _, r := range addressRefs {
		switch v := r.(type) {
		case cldfdatastore.AddressRef:
			require.NoError(t, ds.Addresses().Add(v))
		case cldfdatastore.ContractMetadata:
			require.NoError(t, ds.ContractMetadata().Add(v))
		case cldfdatastore.ChainMetadata:
			require.NoError(t, ds.ChainMetadata().Add(v))
		}
	}

	return ds
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
