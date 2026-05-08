package changesets

import (
	"errors"
	"fmt"

	"github.com/samber/lo"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/catalog/operations"
)

// CreateAddressRefChangeset creates address ref entries in the Catalog service.
type CreateAddressRefChangeset struct{}

type CreateAddressRefChangesetInput struct {
	AddressRefs []cldfdatastore.AddressRef `json:"addressRefs"`
}

// VerifyPreconditions ensures the input is valid.
func (CreateAddressRefChangeset) VerifyPreconditions(e cldf.Environment, input CreateAddressRefChangesetInput) error {
	if len(input.AddressRefs) == 0 {
		return errors.New("missing address refs input")
	}
	if e.DataStore == nil {
		return errors.New("missing datastore in environment")
	}

	uniqAddressRefs := lo.UniqBy(input.AddressRefs, func(ar cldfdatastore.AddressRef) cldfdatastore.AddressRefKey {
		return ar.Key()
	})
	if len(uniqAddressRefs) != len(input.AddressRefs) {
		return errors.New("duplicate address ref entries found in input")
	}

	for _, addressRef := range input.AddressRefs {
		_, err := e.DataStore.Addresses().Get(addressRef.Key())
		if err == nil {
			return fmt.Errorf("address ref for chain selector %v, type %v, version %v and qualifier %q already exists",
				addressRef.ChainSelector, addressRef.Type, addressRef.Version, addressRef.Qualifier)
		}
		if !errors.Is(err, cldfdatastore.ErrAddressRefNotFound) {
			return fmt.Errorf("failed to retrieve address ref for chain selector %v, type %v, version %v and qualifier %q: %w",
				addressRef.ChainSelector, addressRef.Type, addressRef.Version, addressRef.Qualifier, err)
		}
	}

	return nil
}

// Apply executes the changeset, adding the address refs to the Catalog service.
func (CreateAddressRefChangeset) Apply(e cldf.Environment, input CreateAddressRefChangesetInput) (cldf.ChangesetOutput, error) {
	deps := operations.CreateAddressRefDeps{DataStore: e.DataStore}
	opInput := operations.CreateAddressRefInput{AddressRefs: input.AddressRefs}

	report, err := cldfops.ExecuteOperation(e.OperationsBundle, operations.CreateAddressRefOp, deps, opInput)
	out := cldf.ChangesetOutput{
		DataStore: report.Output.DataStore,
		Reports:   []cldfops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
