package nodes

import (
	"errors"
	"fmt"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	jdops "github.com/smartcontractkit/cld-changesets/jd/operations"
)

var _ cldf.ChangeSetV2[RegisterNodesInput] = RegisterNodesChangeset{}

// NodeToRegister is the per-node input for RegisterNodesInput.
type NodeToRegister = jdops.NodeToRegister

// RegisterNodesInput is the serializable input of RegisterNodesChangeset.
type RegisterNodesInput = jdops.RegisterNodesInput

// RegisterNodesChangeset upserts nodes (already-registered nodes are skipped).
type RegisterNodesChangeset struct{}

func (RegisterNodesChangeset) VerifyPreconditions(_ cldf.Environment, cfg RegisterNodesInput) error {
	if cfg.Domain == "" {
		return errors.New("domain is required")
	}
	if len(cfg.Nodes) == 0 {
		return errors.New("no nodes provided")
	}
	seenNames := make(map[string]struct{}, len(cfg.Nodes))
	seenCSAKeys := make(map[string]struct{}, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		if n.Name == "" {
			return errors.New("node name cannot be empty")
		}
		if n.CSAKey == "" {
			return fmt.Errorf("node CSA key cannot be empty for node %q", n.Name)
		}
		if _, ok := seenNames[n.Name]; ok {
			return fmt.Errorf("duplicate node name %q", n.Name)
		}
		seenNames[n.Name] = struct{}{}
		if _, ok := seenCSAKeys[n.CSAKey]; ok {
			return fmt.Errorf("duplicate csa_key for node %q", n.Name)
		}
		seenCSAKeys[n.CSAKey] = struct{}{}
	}

	return nil
}

func (RegisterNodesChangeset) Apply(e cldf.Environment, input RegisterNodesInput) (cldf.ChangesetOutput, error) {
	deps := jdops.JDOpDeps{Offchain: e.Offchain, EnvName: e.Name}
	_, err := fwops.ExecuteSequence(e.OperationsBundle, jdops.SeqJDRegisterNodes, deps, input)

	return cldf.ChangesetOutput{}, err
}
