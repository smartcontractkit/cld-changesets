package changesets

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/datastore/operations"
)

// DeleteAddressRefChangeset deletes address ref entries from the Datastore.
type DeleteAddressRefChangeset struct{}

type DeleteAddressRefChangesetInput struct {
	AddressRefKeys []DeleteAddressRefKey `json:"addressRefKeys"`
}

type DeleteAddressRefKey struct {
	ChainSelector uint64                     `json:"chainSelector"`
	Type          cldfdatastore.ContractType `json:"type"`
	Version       *semver.Version            `json:"version"`
	Qualifier     string                     `json:"qualifier"`
}

func (k DeleteAddressRefKey) addressRefKey() (cldfdatastore.AddressRefKey, error) {
	if k.Version == nil {
		return nil, cldfdatastore.ErrAddressRefVersionRequired
	}

	return cldfdatastore.NewAddressRefKey(k.ChainSelector, k.Type, k.Version, k.Qualifier), nil
}

func (i DeleteAddressRefChangesetInput) addressRefKeys() ([]cldfdatastore.AddressRefKey, error) {
	keys := make([]cldfdatastore.AddressRefKey, 0, len(i.AddressRefKeys))
	for _, inputKey := range i.AddressRefKeys {
		key, err := inputKey.addressRefKey()
		if err != nil {
			return nil, err
		}

		keys = append(keys, key)
	}

	return keys, nil
}

// VerifyPreconditions ensures the input is valid.
func (DeleteAddressRefChangeset) VerifyPreconditions(e cldf.Environment, input DeleteAddressRefChangesetInput) error {
	if len(input.AddressRefKeys) == 0 {
		return errors.New("missing address ref keys input")
	}
	if e.DataStore == nil {
		return errors.New("missing datastore in environment")
	}

	keys, err := input.addressRefKeys()
	if err != nil {
		return fmt.Errorf("invalid address ref keys input: %w", err)
	}

	for _, key := range keys {
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
	deps := operations.DeleteAddressRefDeps{DataStore: e.DataStore}
	addressRefKeys, err := input.addressRefKeys()
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("invalid address ref keys input: %w", err)
	}
	opInput := operations.DeleteAddressRefInput{AddressRefKeys: addressRefKeys}

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
