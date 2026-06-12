package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/datastore/internal/keys"
)

// DeleteAddressRefDeps holds non-serializable dependencies for the DeleteAddressRefOp operation.
type DeleteAddressRefDeps struct {
	DataStore cldfdatastore.DataStore
}

// DeleteAddressRefInput is the serializable input of a DeleteAddressRefOp invocation.
type DeleteAddressRefInput struct {
	AddressRefKeys []keys.AddressRefKey `json:"addressRefKeys"`
}

// DeleteAddressRefOutput is the serializable output of a DeleteAddressRefOp invocation.
type DeleteAddressRefOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// DeleteAddressRefOp deletes address ref entries from the Datastore.
var DeleteAddressRefOp = cldfops.NewOperation(
	"datastore-delete-address-ref",
	semver.MustParse("1.0.0"),
	"Delete address ref entries from the Datastore",
	func(b cldfops.Bundle, deps DeleteAddressRefDeps, input DeleteAddressRefInput) (DeleteAddressRefOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return DeleteAddressRefOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		for i, key := range input.AddressRefKeys {
			fwKey, err := key.ToFrameworkKey()
			if err != nil {
				return DeleteAddressRefOutput{}, fmt.Errorf("addressRefKeys[%d]: %w", i, err)
			}

			err = dataStore.Addresses().RemoteDelete(fwKey)
			if err != nil {
				return DeleteAddressRefOutput{}, fmt.Errorf("failed to delete address ref entry %d in datastore: %w", i, err)
			}
		}

		b.Logger.Infow("Datastore AddressRef successfully staged for deletion")

		return DeleteAddressRefOutput{DataStore: dataStore}, nil
	},
)
