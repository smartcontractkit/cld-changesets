package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// DeleteChainMetadataDeps holds non-serializable dependencies for the DeleteChainMetadataOp operation.
type DeleteChainMetadataDeps struct {
	DataStore cldfdatastore.DataStore
}

// DeleteChainMetadataInput is the serializable input of a DeleteChainMetadataOp invocation.
type DeleteChainMetadataInput struct {
	ChainMetadataKeys []cldfdatastore.ChainMetadataKey
}

// DeleteChainMetadataOutput is the serializable output of a DeleteChainMetadataOp invocation.
type DeleteChainMetadataOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// DeleteChainMetadataOp deletes chain metadata entries from the Datastore.
var DeleteChainMetadataOp = cldfops.NewOperation(
	"datastore-delete-chain-metadata",
	semver.MustParse("1.0.0"),
	"Delete chain metadata entries from the Datastore",
	func(b cldfops.Bundle, deps DeleteChainMetadataDeps, input DeleteChainMetadataInput) (DeleteChainMetadataOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return DeleteChainMetadataOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		for i, key := range input.ChainMetadataKeys {
			err = dataStore.ChainMetadata().RemoteDelete(key)
			if err != nil {
				return DeleteChainMetadataOutput{}, fmt.Errorf("failed to delete chain metadata entry %d in datastore: %w", i, err)
			}
		}

		b.Logger.Infow("Datastore ChainMetadata successfully staged for deletion")

		return DeleteChainMetadataOutput{DataStore: dataStore}, nil
	},
)
