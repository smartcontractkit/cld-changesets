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

// DeleteContractMetadataChangeset deletes existing contract metadata entries from the Catalog service.
type DeleteContractMetadataChangeset struct{}

type DeleteContractMetadataChangesetInput struct {
	ContractMetadataKeys []cldfdatastore.ContractMetadataKey `json:"contractMetadataKeys"`
}

// VerifyPreconditions ensures the input is valid.
func (DeleteContractMetadataChangeset) VerifyPreconditions(e cldf.Environment, input DeleteContractMetadataChangesetInput) error {
	if len(input.ContractMetadataKeys) == 0 {
		return errors.New("missing contract metadata keys input")
	}
	if e.DataStore == nil {
		return errors.New("missing datastore in environment")
	}

	uniqKeys := lo.Uniq(input.ContractMetadataKeys)
	if len(uniqKeys) != len(input.ContractMetadataKeys) {
		return errors.New("duplicate contract metadata keys found in input")
	}

	for _, key := range input.ContractMetadataKeys {
		_, err := e.DataStore.ContractMetadata().Get(key)
		if errors.Is(err, cldfdatastore.ErrContractMetadataNotFound) {
			return fmt.Errorf("contract metadata for chain selector %v and address %v does not exist",
				key.ChainSelector(), key.Address())
		}
		if err != nil {
			return fmt.Errorf("failed to retrieve contract metadata for chain selector %v and address %v: %w",
				key.ChainSelector(), key.Address(), err)
		}
	}

	return nil
}

// Apply executes the changeset, deleting the contract metadata from the Catalog service.
func (DeleteContractMetadataChangeset) Apply(e cldf.Environment, input DeleteContractMetadataChangesetInput) (cldf.ChangesetOutput, error) {
	deps := operations.DeleteContractMetadataDeps{DataStore: e.DataStore}
	opInput := operations.DeleteContractMetadataInput{ContractMetadataKeys: input.ContractMetadataKeys}

	report, err := cldfops.ExecuteOperation(e.OperationsBundle, operations.DeleteContractMetadataOp, deps, opInput)
	out := cldf.ChangesetOutput{
		DataStore: report.Output.DataStore,
		Reports:   []cldfops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
