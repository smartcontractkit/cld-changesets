package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// DeleteContractMetadataDeps holds non-serializable dependencies for the
// DeleteContractMetadataOp operation.
type DeleteContractMetadataDeps struct {
	DataStore cldfdatastore.DataStore
}

// DeleteContractMetadataInput is the serializable input of a DeleteContractMetadataOp invocation.
type DeleteContractMetadataInput struct {
	ContractMetadataKeys []cldfdatastore.ContractMetadataKey
}

// DeleteContractMetadataOutput is the serializable output of a DeleteContractMetadataOp invocation.
type DeleteContractMetadataOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// DeleteContractMetadataOp deletes existing contract metadata entries from the Catalog service.
var DeleteContractMetadataOp = cldfops.NewOperation(
	"catalog-delete-contract-metadata",
	semver.MustParse("1.0.0"),
	"Delete contract metadata entries from the Catalog service",
	func(b cldfops.Bundle, deps DeleteContractMetadataDeps, input DeleteContractMetadataInput) (DeleteContractMetadataOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return DeleteContractMetadataOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		for i, key := range input.ContractMetadataKeys {
			err = dataStore.ContractMetadata().Delete(key)
			if err != nil {
				return DeleteContractMetadataOutput{}, fmt.Errorf("failed to delete contract metadata entry %d from catalog store: %w", i, err)
			}
		}

		b.Logger.Infow("Catalog ContractMetadata deleted successfully")

		return DeleteContractMetadataOutput{DataStore: dataStore}, nil
	},
)
