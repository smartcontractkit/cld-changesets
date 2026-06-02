package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// UpdateContractMetadataDeps holds non-serializable dependencies for the
// UpdateContractMetadataOp operation.
type UpdateContractMetadataDeps struct {
	DataStore cldfdatastore.DataStore
}

// UpdateContractMetadataInput is the serializable input of an UpdateContractMetadataOp invocation.
type UpdateContractMetadataInput struct {
	ContractMetadata []cldfdatastore.ContractMetadata
}

// UpdateContractMetadataOutput is the serializable output of an UpdateContractMetadataOp invocation.
type UpdateContractMetadataOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// UpdateContractMetadataOp updates existing contract metadata entries in the Datastore.
var UpdateContractMetadataOp = cldfops.NewOperation(
	"datastore-update-contract-metadata",
	semver.MustParse("1.0.0"),
	"Update contract metadata entries in the Datastore",
	func(b cldfops.Bundle, deps UpdateContractMetadataDeps, input UpdateContractMetadataInput) (UpdateContractMetadataOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return UpdateContractMetadataOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		for i, item := range input.ContractMetadata {
			err = dataStore.ContractMetadata().Update(item)
			if err != nil {
				return UpdateContractMetadataOutput{}, fmt.Errorf("failed to update contract metadata entry %d in datastore: %w", i, err)
			}
		}

		b.Logger.Infow("Datastore ContractMetadata updated successfully")

		return UpdateContractMetadataOutput{DataStore: dataStore}, nil
	},
)
