package sequences

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldfops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/datastore/keys"
	datastoreops "github.com/smartcontractkit/cld-changesets/datastore/operations"
)

// DeleteResourcesSeqDeps holds non-serializable dependencies for the DeleteResourcesSeq sequence.
type DeleteResourcesSeqDeps struct {
	DataStore cldfdatastore.DataStore
}

// DeleteResourcesSeqInput is the serializable input of a DeleteResourcesSeq invocation.
type DeleteResourcesSeqInput struct {
	AddressRefKeys       []keys.AddressRefKey       `json:"addressRefKeys"`
	ContractMetadataKeys []keys.ContractMetadataKey `json:"contractMetadataKeys"`
	ChainMetadataKeys    []keys.ChainMetadataKey    `json:"chainMetadataKeys"`
}

// DeleteResourcesSeqOutput is the serializable output of a DeleteResourcesSeq invocation.
type DeleteResourcesSeqOutput struct {
	DataStore cldfdatastore.MutableDataStore
}

// DeleteResourcesSeq stages deletions across address refs, contract metadata, and chain metadata
// by composing the corresponding per-resource delete operations. Empty slices skip the corresponding
// resource type within each operation. Staged deletions are not applied immediately; they are recorded in the
// Datastore and executed during the post-changeset merge phase.
var DeleteResourcesSeq = cldfops.NewSequence(
	"datastore-delete-resources",
	semver.MustParse("1.0.0"),
	"Stage deletions of address refs, contract metadata, and chain metadata from the Datastore",
	func(b cldfops.Bundle, deps DeleteResourcesSeqDeps, input DeleteResourcesSeqInput) (DeleteResourcesSeqOutput, error) {
		addressRefReport, err := cldfops.ExecuteOperation(
			b,
			datastoreops.DeleteAddressRefOp,
			datastoreops.DeleteAddressRefDeps{DataStore: deps.DataStore},
			datastoreops.DeleteAddressRefInput{AddressRefKeys: input.AddressRefKeys},
		)
		if err != nil {
			return DeleteResourcesSeqOutput{}, fmt.Errorf("failed to delete address refs: %w", err)
		}

		contractMetadataReport, err := cldfops.ExecuteOperation(
			b,
			datastoreops.DeleteContractMetadataOp,
			datastoreops.DeleteContractMetadataDeps{DataStore: addressRefReport.Output.DataStore.Seal()},
			datastoreops.DeleteContractMetadataInput{ContractMetadataKeys: input.ContractMetadataKeys},
		)
		if err != nil {
			return DeleteResourcesSeqOutput{}, fmt.Errorf("failed to delete contract metadata: %w", err)
		}

		chainMetadataReport, err := cldfops.ExecuteOperation(
			b,
			datastoreops.DeleteChainMetadataOp,
			datastoreops.DeleteChainMetadataDeps{DataStore: contractMetadataReport.Output.DataStore.Seal()},
			datastoreops.DeleteChainMetadataInput{ChainMetadataKeys: input.ChainMetadataKeys},
		)
		if err != nil {
			return DeleteResourcesSeqOutput{}, fmt.Errorf("failed to delete chain metadata: %w", err)
		}

		b.Logger.Infow("Datastore resources successfully staged for deletion")

		return DeleteResourcesSeqOutput{DataStore: chainMetadataReport.Output.DataStore}, nil
	},
)
