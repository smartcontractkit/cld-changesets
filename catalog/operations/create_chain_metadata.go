package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// CreateChainMetadataDeps holds non-serializable dependencies for the
// CreateChainMetadataOp operation.
type CreateChainMetadataDeps struct {
	DataStore cldfdatastore.DataStore
}

// CreateChainMetadataInput is the serializable input of a CreateChainMetadataOp invocation.
type CreateChainMetadataInput struct {
	ChainMetadata []cldfdatastore.ChainMetadata
}

// CreateChainMetadataOutput is the serializable output of a CreateChainMetadataOp invocation.
type CreateChainMetadataOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// CreateChainMetadataOp creates chain metadata entries in the Catalog service.
var CreateChainMetadataOp = cldfops.NewOperation(
	"catalog-create-chain-metadata",
	semver.MustParse("1.0.0"),
	"Add chain metadata entries to the Catalog service",
	func(b cldfops.Bundle, deps CreateChainMetadataDeps, input CreateChainMetadataInput) (CreateChainMetadataOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return CreateChainMetadataOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		for i, item := range input.ChainMetadata {
			err = dataStore.ChainMetadata().Add(item)
			if err != nil {
				return CreateChainMetadataOutput{}, fmt.Errorf("failed to create chain metadata entry %d in catalog store: %w", i, err)
			}
		}

		b.Logger.Infow("Catalog ChainMetadata created successfully")

		return CreateChainMetadataOutput{DataStore: dataStore}, nil
	},
)
