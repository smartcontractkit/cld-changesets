package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// UpdateChainMetadataDeps holds non-serializable dependencies for the
// UpdateChainMetadataOp operation.
type UpdateChainMetadataDeps struct {
	DataStore cldfdatastore.DataStore
}

// UpdateChainMetadataInput is the serializable input of an UpdateChainMetadataOp invocation.
type UpdateChainMetadataInput struct {
	ChainMetadata []cldfdatastore.ChainMetadata
}

// UpdateChainMetadataOutput is the serializable output of an UpdateChainMetadataOp invocation.
type UpdateChainMetadataOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// UpdateChainMetadataOp updates existing chain metadata entries in the Catalog service.
var UpdateChainMetadataOp = cldfops.NewOperation(
	"catalog-update-chain-metadata",
	semver.MustParse("1.0.0"),
	"Update chain metadata entries in the Catalog service",
	func(b cldfops.Bundle, deps UpdateChainMetadataDeps, input UpdateChainMetadataInput) (UpdateChainMetadataOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return UpdateChainMetadataOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		for i, item := range input.ChainMetadata {
			err = dataStore.ChainMetadata().Update(item)
			if err != nil {
				return UpdateChainMetadataOutput{}, fmt.Errorf("failed to update chain metadata entry %d in catalog store: %w", i, err)
			}
		}

		b.Logger.Infow("Catalog ChainMetadata updated successfully")

		return UpdateChainMetadataOutput{DataStore: dataStore}, nil
	},
)
