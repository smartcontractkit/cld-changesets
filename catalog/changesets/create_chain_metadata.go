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

// CreateChainMetadataChangeset creates chain metadata entries in the Catalog service.
type CreateChainMetadataChangeset struct{}

type CreateChainMetadataChangesetInput struct {
	ChainMetadata []cldfdatastore.ChainMetadata `json:"chainMetadata"`
}

// VerifyPreconditions ensures the input is valid.
func (CreateChainMetadataChangeset) VerifyPreconditions(e cldf.Environment, input CreateChainMetadataChangesetInput) error {
	if len(input.ChainMetadata) == 0 {
		return errors.New("missing chain metadata input")
	}
	if e.DataStore == nil {
		return errors.New("missing datastore in environment")
	}

	uniqChainMetadata := lo.UniqBy(input.ChainMetadata, func(cm cldfdatastore.ChainMetadata) cldfdatastore.ChainMetadataKey {
		return cm.Key()
	})
	if len(uniqChainMetadata) != len(input.ChainMetadata) {
		return errors.New("duplicate chain metadata entries found in input")
	}

	for _, chainMetadata := range input.ChainMetadata {
		_, err := e.DataStore.ChainMetadata().Get(chainMetadata.Key())
		if err == nil {
			return fmt.Errorf("chain metadata for chain selector %v already exists",
				chainMetadata.ChainSelector)
		}
		if !errors.Is(err, cldfdatastore.ErrChainMetadataNotFound) {
			return fmt.Errorf("failed to retrieve chain metadata for chain selector %v: %w",
				chainMetadata.ChainSelector, err)
		}
	}

	return nil
}

// Apply executes the changeset, adding the chain metadata to the Catalog service.
func (CreateChainMetadataChangeset) Apply(e cldf.Environment, input CreateChainMetadataChangesetInput) (cldf.ChangesetOutput, error) {
	deps := operations.CreateChainMetadataDeps{DataStore: e.DataStore}
	opInput := operations.CreateChainMetadataInput{ChainMetadata: input.ChainMetadata}

	report, err := cldfops.ExecuteOperation(e.OperationsBundle, operations.CreateChainMetadataOp, deps, opInput)
	out := cldf.ChangesetOutput{
		DataStore: report.Output.DataStore,
		Reports:   []cldfops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
