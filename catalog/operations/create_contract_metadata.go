package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// CreateContractMetadataDeps holds non-serializable dependencies for the
// CreateContractMetadataOp operation.
type CreateContractMetadataDeps struct {
	DataStore cldfdatastore.DataStore
}

// CreateContractMetadataInput is the serializable input of a CreateContractMetadataOp invocation.
type CreateContractMetadataInput struct {
	ContractMetadata []cldfdatastore.ContractMetadata
}

// CreateContractMetadataOutput is the serializable output of a CreateContractMetadataOp invocation.
type CreateContractMetadataOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// CreateContractMetadataOp creates contract metadata entries in the Catalog service.
var CreateContractMetadataOp = cldfops.NewOperation(
	"catalog-create-contract-metadata",
	semver.MustParse("1.0.0"),
	"Add contract metadata entries to the Catalog service",
	func(b cldfops.Bundle, deps CreateContractMetadataDeps, input CreateContractMetadataInput) (CreateContractMetadataOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return CreateContractMetadataOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		for i, item := range input.ContractMetadata {
			err = dataStore.ContractMetadata().Add(item)
			if err != nil {
				return CreateContractMetadataOutput{}, fmt.Errorf("failed to create contract metadata entry %d in catalog store: %w", i, err)
			}
		}

		b.Logger.Infow("Catalog ContractMetadata created successfully")

		return CreateContractMetadataOutput{DataStore: dataStore}, nil
	},
)
