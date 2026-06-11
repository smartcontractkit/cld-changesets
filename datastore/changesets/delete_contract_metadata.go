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

// DeleteContractMetadataChangeset deletes contract metadata entries from the Datastore.
type DeleteContractMetadataChangeset struct{}

type DeleteContractMetadataChangesetInput struct {
	ContractMetadataKeys []keys.ContractMetadataKey `json:"contractMetadataKeys"`
}

// VerifyPreconditions ensures the input is valid.
func (DeleteContractMetadataChangeset) VerifyPreconditions(e cldf.Environment, input DeleteContractMetadataChangesetInput) error {
	if len(input.ContractMetadataKeys) == 0 {
		return errors.New("missing contract metadata keys input")
	}
	if e.DataStore == nil {
		return errors.New("missing datastore in environment")
	}

	for _, key := range input.ContractMetadataKeys {
		fwKey := key.ToFrameworkKey()
		_, err := e.DataStore.ContractMetadata().Get(fwKey)
		if err != nil {
			if errors.Is(err, cldfdatastore.ErrContractMetadataNotFound) {
				return fmt.Errorf("contract metadata entry for chain selector %v and address %v does not exist",
					fwKey.ChainSelector(), fwKey.Address())
			}

			return fmt.Errorf("failed to retrieve contract metadata entry for chain selector %v and address %v: %w",
				fwKey.ChainSelector(), fwKey.Address(), err)
		}
	}

	return nil
}

// Apply executes the changeset, staging the contract metadata entries to be deleted from the Datastore.
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
