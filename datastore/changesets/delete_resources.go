package changesets

import (
	"errors"
	"fmt"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/datastore/keys"
	datastoreseqs "github.com/smartcontractkit/cld-changesets/datastore/sequences"
)

// DeleteResourcesChangeset stages deletions of address refs, contract metadata, and chain metadata
// in a single invocation. Staged deletions are not applied immediately; they are recorded in the
// Datastore and executed during the post-changeset merge phase.
type DeleteResourcesChangeset struct{}

type DeleteResourcesChangesetInput struct {
	AddressRefKeys       []keys.AddressRefKey       `json:"addressRefKeys"`
	ContractMetadataKeys []keys.ContractMetadataKey `json:"contractMetadataKeys"`
	ChainMetadataKeys    []keys.ChainMetadataKey    `json:"chainMetadataKeys"`
}

// VerifyPreconditions ensures the input is valid.
func (DeleteResourcesChangeset) VerifyPreconditions(e cldf.Environment, input DeleteResourcesChangesetInput) error {
	if e.DataStore == nil {
		return errors.New("missing datastore in environment")
	}

	if len(input.AddressRefKeys) == 0 && len(input.ContractMetadataKeys) == 0 && len(input.ChainMetadataKeys) == 0 {
		return errors.New("at least one resource key slice must be non-empty")
	}

	for i, key := range input.AddressRefKeys {
		fwKey, err := key.ToFrameworkKey()
		if err != nil {
			return fmt.Errorf("addressRefKeys[%d]: %w", i, err)
		}

		_, err = e.DataStore.Addresses().Get(fwKey)
		if err != nil {
			if errors.Is(err, cldfdatastore.ErrAddressRefNotFound) {
				return fmt.Errorf("address ref entry for chain selector %v, type %v, version %v and qualifier %q does not exist",
					fwKey.ChainSelector(), fwKey.Type(), fwKey.Version(), fwKey.Qualifier())
			}

			return fmt.Errorf("failed to retrieve address ref entry for chain selector %v, type %v, version %v and qualifier %q: %w",
				fwKey.ChainSelector(), fwKey.Type(), fwKey.Version(), fwKey.Qualifier(), err)
		}
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

	for _, key := range input.ChainMetadataKeys {
		fwKey := key.ToFrameworkKey()
		_, err := e.DataStore.ChainMetadata().Get(fwKey)
		if err != nil {
			if errors.Is(err, cldfdatastore.ErrChainMetadataNotFound) {
				return fmt.Errorf("chain metadata entry for chain selector %v does not exist", fwKey.ChainSelector())
			}

			return fmt.Errorf("failed to retrieve chain metadata entry for chain selector %v: %w", fwKey.ChainSelector(), err)
		}
	}

	return nil
}

// Apply executes the changeset, staging the resources to be deleted from the Datastore.
func (DeleteResourcesChangeset) Apply(e cldf.Environment, input DeleteResourcesChangesetInput) (cldf.ChangesetOutput, error) {
	deps := datastoreseqs.DeleteResourcesSeqDeps{DataStore: e.DataStore}
	seqInput := datastoreseqs.DeleteResourcesSeqInput{
		AddressRefKeys:       input.AddressRefKeys,
		ContractMetadataKeys: input.ContractMetadataKeys,
		ChainMetadataKeys:    input.ChainMetadataKeys,
	}

	report, err := cldfops.ExecuteSequence(
		e.OperationsBundle,
		datastoreseqs.DeleteResourcesSeq,
		deps,
		seqInput,
	)
	out := cldf.ChangesetOutput{
		DataStore: report.Output.DataStore,
		Reports:   report.ExecutionReports,
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
