package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// CreateAddressRefDeps holds non-serializable dependencies for the
// CreateAddressRefOp operation.
type CreateAddressRefDeps struct {
	DataStore cldfdatastore.DataStore
}

// CreateAddressRefInput is the serializable input of a CreateAddressRefOp invocation.
type CreateAddressRefInput struct {
	AddressRefs []cldfdatastore.AddressRef
}

// CreateAddressRefOutput is the serializable output of a CreateAddressRefOp invocation.
type CreateAddressRefOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// CreateAddressRefOp creates address ref entries in the Catalog service.
var CreateAddressRefOp = cldfops.NewOperation(
	"catalog-create-address-ref",
	semver.MustParse("1.0.0"),
	"Add address ref entries to the Catalog service",
	func(b cldfops.Bundle, deps CreateAddressRefDeps, input CreateAddressRefInput) (CreateAddressRefOutput, error) {
		dataStore := cldfdatastore.NewMemoryDataStore()
		err := dataStore.Merge(deps.DataStore)
		if err != nil {
			return CreateAddressRefOutput{}, fmt.Errorf("failed to create memory data store: %w", err)
		}

		for i, item := range input.AddressRefs {
			err = dataStore.Addresses().Add(item)
			if err != nil {
				return CreateAddressRefOutput{}, fmt.Errorf("failed to create address ref entry %d in catalog store: %w", i, err)
			}
		}

		b.Logger.Infow("Catalog AddressRef created successfully")

		return CreateAddressRefOutput{DataStore: dataStore}, nil
	},
)
