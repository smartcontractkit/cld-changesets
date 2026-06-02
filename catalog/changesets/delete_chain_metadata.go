package changesets

import (
	"errors"
	"fmt"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/catalog/operations"
)

// DeleteChainMetadataChangeset deletes chain metadata entries from the Catalog service.
type DeleteChainMetadataChangeset struct{}

type DeleteChainMetadataChangesetInput struct {
	ChainMetadataKeys []cldfdatastore.ChainMetadataKey `json:"chainMetadataKeys"`
}

// VerifyPreconditions ensures the input is valid.
func (DeleteChainMetadataChangeset) VerifyPreconditions(e cldf.Environment, input DeleteChainMetadataChangesetInput) error {
	if len(input.ChainMetadataKeys) == 0 {
		return errors.New("missing chain metadata keys input")
	}
	if e.DataStore == nil {
		return errors.New("missing datastore in environment")
	}

	for _, key := range input.ChainMetadataKeys {
		_, err := e.DataStore.ChainMetadata().Get(key)
		if err != nil {
			if errors.Is(err, cldfdatastore.ErrChainMetadataNotFound) {
				return fmt.Errorf("chain metadata entry for chain selector %v does not exist", key.ChainSelector())
			}

			return fmt.Errorf("failed to retrieve chain metadata entry for chain selector %v: %w", key.ChainSelector(), err)
		}
	}

	return nil
}

// Apply executes the changeset, staging the chain metadata entries to be deleted from the Catalog service or local datastore files.
func (DeleteChainMetadataChangeset) Apply(e cldf.Environment, input DeleteChainMetadataChangesetInput) (cldf.ChangesetOutput, error) {
	deps := operations.DeleteChainMetadataDeps{DataStore: e.DataStore}
	opInput := operations.DeleteChainMetadataInput{ChainMetadataKeys: input.ChainMetadataKeys}

	report, err := cldfops.ExecuteOperation(e.OperationsBundle, operations.DeleteChainMetadataOp, deps, opInput)
	out := cldf.ChangesetOutput{
		DataStore: report.Output.DataStore,
		Reports:   []cldfops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
