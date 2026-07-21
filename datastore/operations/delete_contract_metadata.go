package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/datastore/keys"
)

// DeleteContractMetadataDeps holds non-serializable dependencies for the DeleteContractMetadataOp operation.
type DeleteContractMetadataDeps struct {
	DataStore cldfdatastore.DataStore
}

// DeleteContractMetadataInput is the serializable input of a DeleteContractMetadataOp invocation.
type DeleteContractMetadataInput struct {
	ContractMetadataKeys []keys.ContractMetadataKey `json:"contractMetadataKeys"`
}

// DeleteContractMetadataOutput is the serializable output of a DeleteContractMetadataOp invocation.
type DeleteContractMetadataOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// DeleteContractMetadataOp deletes contract metadata entries from the Datastore.
var DeleteContractMetadataOp = cldfops.NewOperation(
	"datastore-delete-contract-metadata",
	semver.MustParse("1.0.0"),
	"Delete contract metadata entries from the Datastore",
	func(b cldfops.Bundle, deps DeleteContractMetadataDeps, input DeleteContractMetadataInput) (DeleteContractMetadataOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return DeleteContractMetadataOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		for i, key := range input.ContractMetadataKeys {
			err = dataStore.ContractMetadata().RemoteDelete(key.ToFrameworkKey())
			if err != nil {
				return DeleteContractMetadataOutput{}, fmt.Errorf("failed to delete contract metadata entry %d in datastore: %w", i, err)
			}
		}

		b.Logger.Infow("Datastore ContractMetadata successfully staged for deletion")

		return DeleteContractMetadataOutput{DataStore: dataStore}, nil
	},
)
