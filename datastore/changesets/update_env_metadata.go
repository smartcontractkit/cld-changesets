package changesets

import (
	"errors"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/datastore/operations"
)

// UpdateEnvMetadataChangeset updates existing env metadata entries in the Datastore.
type UpdateEnvMetadataChangeset struct{}

type UpdateEnvMetadataChangesetInput struct {
	EnvMetadata cldfdatastore.EnvMetadata `json:"envMetadata"`
}

// VerifyPreconditions ensures the input is valid.
func (UpdateEnvMetadataChangeset) VerifyPreconditions(e cldf.Environment, input UpdateEnvMetadataChangesetInput) error {
	if input.EnvMetadata.Metadata == nil {
		return errors.New("missing env metadata input")
	}
	if e.DataStore == nil {
		return errors.New("missing datastore in environment")
	}

	return nil
}

// Apply executes the changeset, updating the env metadata in the Datastore.
func (UpdateEnvMetadataChangeset) Apply(
	e cldf.Environment, input UpdateEnvMetadataChangesetInput,
) (cldf.ChangesetOutput, error) {
	deps := operations.UpdateEnvMetadataDeps{DataStore: e.DataStore}
	opInput := operations.UpdateEnvMetadataInput{EnvMetadata: input.EnvMetadata}

	report, err := cldfops.ExecuteOperation(e.OperationsBundle, operations.UpdateEnvMetadataOp, deps, opInput)
	out := cldf.ChangesetOutput{
		DataStore: report.Output.DataStore,
		Reports:   []cldfops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
