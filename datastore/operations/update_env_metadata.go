package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// UpdateEnvMetadataDeps holds non-serializable dependencies for the
// UpdateEnvMetadataOp operation.
type UpdateEnvMetadataDeps struct {
	DataStore cldfdatastore.DataStore
}

// UpdateEnvMetadataInput is the serializable input of an UpdateEnvMetadataOp invocation.
type UpdateEnvMetadataInput struct {
	EnvMetadata cldfdatastore.EnvMetadata
}

// UpdateEnvMetadataOutput is the serializable output of an UpdateEnvMetadataOp invocation.
type UpdateEnvMetadataOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// UpdateEnvMetadataOp updates existing env metadata entries in the Datastore.
var UpdateEnvMetadataOp = cldfops.NewOperation(
	"datastore-update-env-metadata",
	semver.MustParse("1.0.0"),
	"Update env metadata entries in the Datastore",
	func(b cldfops.Bundle, deps UpdateEnvMetadataDeps, input UpdateEnvMetadataInput) (UpdateEnvMetadataOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return UpdateEnvMetadataOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		err = dataStore.EnvMetadata().Set(input.EnvMetadata)
		if err != nil {
			return UpdateEnvMetadataOutput{}, fmt.Errorf("failed to update env metadata in datastore: %w", err)
		}

		b.Logger.Infow("Datastore EnvMetadata updated successfully")

		return UpdateEnvMetadataOutput{DataStore: dataStore}, nil
	},
)
