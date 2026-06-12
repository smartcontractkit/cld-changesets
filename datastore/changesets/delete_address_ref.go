package changesets

import (
	"errors"
	"fmt"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/datastore/internal/keys"
	"github.com/smartcontractkit/cld-changesets/datastore/operations"
)

// DeleteAddressRefChangeset deletes address ref entries from the Datastore.
type DeleteAddressRefChangeset struct{}

type DeleteAddressRefChangesetInput struct {
	AddressRefKeys []keys.AddressRefKey `json:"addressRefKeys"`
}

func (i DeleteAddressRefChangesetInput) addressRefKeys() ([]cldfdatastore.AddressRefKey, error) {
	fwKeys := make([]cldfdatastore.AddressRefKey, 0, len(i.AddressRefKeys))
	for idx, inputKey := range i.AddressRefKeys {
		key, err := inputKey.ToFrameworkKey()
		if err != nil {
			return nil, fmt.Errorf("addressRefKeys[%d]: %w", idx, err)
		}

		fwKeys = append(fwKeys, key)
	}

	return fwKeys, nil
}

// VerifyPreconditions ensures the input is valid.
func (DeleteAddressRefChangeset) VerifyPreconditions(e cldf.Environment, input DeleteAddressRefChangesetInput) error {
	if len(input.AddressRefKeys) == 0 {
		return errors.New("missing address ref keys input")
	}
	if e.DataStore == nil {
		return errors.New("missing datastore in environment")
	}

	fwKeys, err := input.addressRefKeys()
	if err != nil {
		return fmt.Errorf("invalid address ref keys input: %w", err)
	}

	for _, key := range fwKeys {
		_, err := e.DataStore.Addresses().Get(key)
		if err != nil {
			if errors.Is(err, cldfdatastore.ErrAddressRefNotFound) {
				return fmt.Errorf("address ref entry for chain selector %v, type %v, version %v and qualifier %q does not exist",
					key.ChainSelector(), key.Type(), key.Version(), key.Qualifier())
			}

			return fmt.Errorf("failed to retrieve address ref entry for chain selector %v, type %v, version %v and qualifier %q: %w",
				key.ChainSelector(), key.Type(), key.Version(), key.Qualifier(), err)
		}
	}

	return nil
}

// Apply executes the changeset, staging the address refs to be deleted from the Datastore.
func (DeleteAddressRefChangeset) Apply(e cldf.Environment, input DeleteAddressRefChangesetInput) (cldf.ChangesetOutput, error) {
	if _, err := input.addressRefKeys(); err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("invalid address ref keys input: %w", err)
	}

	deps := operations.DeleteAddressRefDeps{DataStore: e.DataStore}
	opInput := operations.DeleteAddressRefInput{AddressRefKeys: input.AddressRefKeys}

	report, err := cldfops.ExecuteOperation(e.OperationsBundle, operations.DeleteAddressRefOp, deps, opInput)
	out := cldf.ChangesetOutput{
		DataStore: report.Output.DataStore,
		Reports:   []cldfops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
