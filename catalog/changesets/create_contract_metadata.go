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

// CreateContractMetadataChangeset creates contract metadata entries in the Catalog service.
type CreateContractMetadataChangeset struct{}

type CreateContractMetadataChangesetInput struct {
	ContractMetadata []cldfdatastore.ContractMetadata `json:"contractMetadata"`
}

// VerifyPreconditions ensures the input is valid.
func (CreateContractMetadataChangeset) VerifyPreconditions(e cldf.Environment, input CreateContractMetadataChangesetInput) error {
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
		_, err := e.DataStore.ContractMetadata().Get(cldfdatastore.NewContractMetadataKey(contractMetadata.ChainSelector, contractMetadata.Address))
		if err == nil {
			return fmt.Errorf("contract metadata for chain selector %v and address %v already exists",
				contractMetadata.ChainSelector, contractMetadata.Address)
		}
		if !errors.Is(err, cldfdatastore.ErrContractMetadataNotFound) {
			return fmt.Errorf("failed to retrieve contract metadata for chain selector %v and address %v: %w",
				contractMetadata.ChainSelector, contractMetadata.Address, err)
		}
	}

	return nil
}

// Apply executes the changeset, adding the contract metadata to the Catalog service.
func (CreateContractMetadataChangeset) Apply(e cldf.Environment, input CreateContractMetadataChangesetInput) (cldf.ChangesetOutput, error) {
	deps := operations.CreateContractMetadataDeps{DataStore: e.DataStore}
	opInput := operations.CreateContractMetadataInput{ContractMetadata: input.ContractMetadata}

	report, err := cldfops.ExecuteOperation(e.OperationsBundle, operations.CreateContractMetadataOp, deps, opInput)
	out := cldf.ChangesetOutput{
		DataStore: report.Output.DataStore,
		Reports:   []cldfops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
