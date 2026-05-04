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

// UpdateContractMetadataChangeset updates existing contract metadata entries in the Catalog service.
type UpdateContractMetadataChangeset struct{}

type UpdateContractMetadataChangesetInput struct {
	ContractMetadata []cldfdatastore.ContractMetadata `json:"contractMetadata"`
}

// VerifyPreconditions ensures the input is valid.
func (UpdateContractMetadataChangeset) VerifyPreconditions(e cldf.Environment, input UpdateContractMetadataChangesetInput) error {
	if len(input.ContractMetadata) == 0 {
		return errors.New("missing contract metadata input")
	}
	if e.DataStore == nil {
		return errors.New("missing datastore in environment")
	}

	uniqContractMetadata := lo.UniqBy(input.ContractMetadata, func(cm cldfdatastore.ContractMetadata) cldfdatastore.ContractMetadataKey {
		return cm.Key()
	})
	if len(uniqContractMetadata) != len(input.ContractMetadata) {
		return errors.New("duplicate contract metadata entries found in input")
	}

	for _, contractMetadata := range input.ContractMetadata {
		_, err := e.DataStore.ContractMetadata().Get(contractMetadata.Key())
		if errors.Is(err, cldfdatastore.ErrContractMetadataNotFound) {
			return fmt.Errorf("contract metadata for chain selector %v and address %v does not exist",
				contractMetadata.ChainSelector, contractMetadata.Address)
		}
		if err != nil {
			return fmt.Errorf("failed to retrieve contract metadata for chain selector %v and address %v: %w",
				contractMetadata.ChainSelector, contractMetadata.Address, err)
		}
	}

	return nil
}

// Apply executes the changeset, updating the contract metadata in the Catalog service.
func (UpdateContractMetadataChangeset) Apply(e cldf.Environment, input UpdateContractMetadataChangesetInput) (cldf.ChangesetOutput, error) {
	deps := operations.UpdateContractMetadataDeps{DataStore: e.DataStore}
	opInput := operations.UpdateContractMetadataInput{ContractMetadata: input.ContractMetadata}

	report, err := cldfops.ExecuteOperation(e.OperationsBundle, operations.UpdateContractMetadataOp, deps, opInput)
	out := cldf.ChangesetOutput{
		DataStore: report.Output.DataStore,
		Reports:   []cldfops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
