package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// UpdateAddressRefDeps holds non-serializable dependencies for the
// UpdateAddressRefOp operation.
type UpdateAddressRefDeps struct {
	DataStore cldfdatastore.DataStore
}

// UpdateAddressRefInput is the serializable input of an UpdateAddressRefOp invocation.
type UpdateAddressRefInput struct {
	AddressRefs []cldfdatastore.AddressRef
}

// UpdateAddressRefOutput is the serializable output of an UpdateAddressRefOp invocation.
type UpdateAddressRefOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// UpdateAddressRefOp updates existing address ref entries in the Catalog service.
var UpdateAddressRefOp = cldfops.NewOperation(
	"catalog-update-address-ref",
	semver.MustParse("1.0.0"),
	"Update address ref entries in the Catalog service",
	func(b cldfops.Bundle, deps UpdateAddressRefDeps, input UpdateAddressRefInput) (UpdateAddressRefOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return UpdateAddressRefOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		for i, item := range input.AddressRefs {
			err = dataStore.Addresses().Update(item)
			if err != nil {
				return UpdateAddressRefOutput{}, fmt.Errorf("failed to update address ref entry %d in catalog store: %w", i, err)
			}
		}

		b.Logger.Infow("Catalog AddressRef updated successfully")

		return UpdateAddressRefOutput{DataStore: dataStore}, nil
	},
)
