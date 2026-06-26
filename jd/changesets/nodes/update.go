package nodes

import (
	"errors"
	"fmt"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	jdops "github.com/smartcontractkit/cld-changesets/jd/operations"
)

var _ cldf.ChangeSetV2[UpdateNodesInput] = UpdateNodesChangeset{}

// NodeToUpdate is the per-node input for UpdateNodesInput.
type NodeToUpdate = jdops.NodeToUpdate

// UpdateNodesInput is the serializable input of UpdateNodesChangeset.
type UpdateNodesInput = jdops.UpdateNodesInput

// UpdateNodesChangeset updates node name and/or labels.
type UpdateNodesChangeset struct{}

func (UpdateNodesChangeset) VerifyPreconditions(_ cldf.Environment, cfg UpdateNodesInput) error {
	if len(cfg.Nodes) == 0 {
		return errors.New("no nodes provided")
	}
	seenIDs := make(map[string]struct{}, len(cfg.Nodes))
	seenCSAKeys := make(map[string]struct{}, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		if n.ID == "" {
			return errors.New("node id cannot be empty")
		}
		if n.CSAKey == "" {
			return fmt.Errorf("node CSA key cannot be empty for node id %q", n.ID)
		}
		if _, ok := seenIDs[n.ID]; ok {
			return fmt.Errorf("duplicate node id %q", n.ID)
		}
		seenIDs[n.ID] = struct{}{}
		if _, ok := seenCSAKeys[n.CSAKey]; ok {
			return fmt.Errorf("duplicate csa_key for node id %q", n.ID)
		}
		seenCSAKeys[n.CSAKey] = struct{}{}
	}

	return nil
}

func (UpdateNodesChangeset) Apply(e cldf.Environment, input UpdateNodesInput) (cldf.ChangesetOutput, error) {
	deps := jdops.JDOpDeps{Offchain: e.Offchain, EnvName: e.Name}
	_, err := fwops.ExecuteSequence(e.OperationsBundle, jdops.SeqJDUpdateNodes, deps, input)

	return cldf.ChangesetOutput{}, err
}
