package operations

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	nodev1 "github.com/smartcontractkit/chainlink-protos/job-distributor/v1/node"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/domain"
	cldf_engine_offchain "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/offchain"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// NodeToRegister is the per-node input for RegisterNodesInput.
type NodeToRegister struct {
	Name        string            `json:"name"         yaml:"name"`
	CSAKey      string            `json:"csa_key"      yaml:"csa_key"`
	IsBootstrap bool              `json:"is_bootstrap" yaml:"is_bootstrap"`
	Labels      map[string]string `json:"labels"       yaml:"labels"`
}

// RegisterNodesInput is the serializable input of SeqJDRegisterNodes.
type RegisterNodesInput struct {
	Domain string           `json:"domain" yaml:"domain"`
	Nodes  []NodeToRegister `json:"nodes"  yaml:"nodes"`
}

// NodeToUpdate is the per-node input for UpdateNodesInput.
type NodeToUpdate struct {
	ID     string            `json:"id"               yaml:"id"`
	CSAKey string            `json:"csa_key"          yaml:"csa_key"`
	Name   string            `json:"name,omitempty"   yaml:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// UpdateNodesInput is the serializable input of SeqJDUpdateNodes.
type UpdateNodesInput struct {
	Nodes []NodeToUpdate `json:"nodes" yaml:"nodes"`
}

// DisableNodesInput is the serializable input of SeqJDDisableNodes.
type DisableNodesInput struct {
	CSAKeys []string `json:"csa_keys" yaml:"csa_keys"`
}

// RegisterNodeInput is the serializable input of OpJDRegisterNode.
type RegisterNodeInput struct {
	Domain      string            `json:"domain"`
	Name        string            `json:"name"`
	CSAKey      string            `json:"csa_key"`
	IsBootstrap bool              `json:"is_bootstrap"`
	Labels      map[string]string `json:"labels"`
}

// RegisterNodeOutput is the serializable output of OpJDRegisterNode.
type RegisterNodeOutput struct {
	NodeID  string `json:"node_id"`
	Skipped bool   `json:"skipped"`
}

// UpdateNodeInput is the serializable input of OpJDUpdateNode.
type UpdateNodeInput struct {
	ID     string            `json:"id"`
	CSAKey string            `json:"csa_key"`
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// UpdateNodeOutput is the serializable output of OpJDUpdateNode.
type UpdateNodeOutput struct {
	NodeID string `json:"node_id"`
}

// DisableNodeInput is the serializable input of OpJDDisableNode.
type DisableNodeInput struct {
	CSAKey string `json:"csa_key"`
}

// DisableNodeOutput is the serializable output of OpJDDisableNode.
type DisableNodeOutput struct {
	NodeID  string `json:"node_id"`
	Skipped bool   `json:"skipped"`
}

// RegisterNodesOutput is the serializable output of SeqJDRegisterNodes.
type RegisterNodesOutput struct {
	RegisteredNodeIDs []string `json:"registered_node_ids"`
	SkippedNodeIDs    []string `json:"skipped_node_ids"`
}

// UpdateNodesOutput is the serializable output of SeqJDUpdateNodes.
type UpdateNodesOutput struct {
	UpdatedNodeIDs []string `json:"updated_node_ids"`
}

// DisableNodesOutput is the serializable output of SeqJDDisableNodes.
type DisableNodesOutput struct {
	DisabledNodeIDs []string `json:"disabled_node_ids"`
	SkippedCSAKeys  []string `json:"skipped_csa_keys"`
}

// OpJDRegisterNode registers a single node.
var OpJDRegisterNode = fwops.NewOperation(
	"jd-register-node",
	semver.MustParse("1.0.0"),
	"Register a node",
	func(b fwops.Bundle, deps JDOpDeps, in RegisterNodeInput) (RegisterNodeOutput, error) {
		ctx := b.GetContext()

		existing, err := ListNodeByPublicKey(ctx, deps.Offchain, in.CSAKey)
		if err != nil {
			return RegisterNodeOutput{}, fmt.Errorf("failed to check if node is already registered: %w", err)
		}
		if existing != nil {
			b.Logger.Infow("node already registered, skipping", "name", in.Name, "csa_key", in.CSAKey)

			return RegisterNodeOutput{NodeID: existing.GetId(), Skipped: true}, nil
		}
		labels := in.Labels
		if labels == nil {
			labels = make(map[string]string)
		}
		d := domain.NewDomain("", in.Domain)
		nodeID, err := cldf_engine_offchain.RegisterNode(ctx, deps.Offchain, in.Name, in.CSAKey, in.IsBootstrap, d, deps.EnvName, labels)
		if err != nil {
			return RegisterNodeOutput{}, fmt.Errorf("failed to register node %q (csa_key: %s): %w", in.Name, in.CSAKey, err)
		}
		b.Logger.Infow("registered node", "name", in.Name, "id", nodeID)

		return RegisterNodeOutput{NodeID: nodeID}, nil
	},
)

// OpJDUpdateNode updates a node's name and/or labels.
var OpJDUpdateNode = fwops.NewOperation(
	"jd-update-node",
	semver.MustParse("1.0.0"),
	"Update a node's name and/or labels",
	func(b fwops.Bundle, deps JDOpDeps, in UpdateNodeInput) (UpdateNodeOutput, error) {
		ctx := b.GetContext()
		resp, err := deps.Offchain.GetNode(ctx, &nodev1.GetNodeRequest{Id: in.ID})
		if err != nil {
			return UpdateNodeOutput{}, fmt.Errorf("failed to get node %q: %w", in.ID, err)
		}
		nodeInfo := resp.GetNode()

		existing, lookupErr := ListNodeByPublicKey(ctx, deps.Offchain, in.CSAKey)
		if lookupErr != nil {
			b.Logger.Warnw("could not verify CSA key conflict, skipping check", "csa_key", in.CSAKey, "error", lookupErr)
		} else if existing != nil && existing.GetId() != in.ID {
			return UpdateNodeOutput{}, fmt.Errorf(
				"CSA key %s is already registered to node %q (%s), cannot reassign to %q",
				in.CSAKey, existing.GetId(), existing.GetName(), in.ID,
			)
		}

		name := nodeInfo.GetName()
		if in.Name != "" {
			name = in.Name
		}
		labels := nodeInfo.GetLabels()
		if in.Labels != nil {
			labels = labelsFromMap(in.Labels)
		}

		if _, err = deps.Offchain.UpdateNode(ctx, &nodev1.UpdateNodeRequest{
			Id:        in.ID,
			Name:      name,
			PublicKey: in.CSAKey,
			Labels:    labels,
		}); err != nil {
			return UpdateNodeOutput{}, fmt.Errorf("failed to update node %q: %w", in.ID, err)
		}
		b.Logger.Infow("updated node", "id", in.ID, "name", name,
			"old_csa_key", nodeInfo.GetPublicKey(), "new_csa_key", in.CSAKey)

		return UpdateNodeOutput{NodeID: in.ID}, nil
	},
)

// OpJDDisableNode disables a node by CSA key.
var OpJDDisableNode = fwops.NewOperation(
	"jd-disable-node",
	semver.MustParse("1.0.0"),
	"Disable a node by CSA key",
	func(b fwops.Bundle, deps JDOpDeps, in DisableNodeInput) (DisableNodeOutput, error) {
		ctx := b.GetContext()
		nodeInfo, err := ListNodeByPublicKey(ctx, deps.Offchain, in.CSAKey)
		if err != nil {
			return DisableNodeOutput{}, fmt.Errorf("failed to look up node with csa_key %s: %w", in.CSAKey, err)
		}
		if nodeInfo == nil {
			b.Logger.Infow("node not found, skipping disable", "csa_key", in.CSAKey)

			return DisableNodeOutput{Skipped: true}, nil
		}
		if !nodeInfo.GetIsEnabled() {
			b.Logger.Infow("node already disabled, skipping", "node_id", nodeInfo.GetId())

			return DisableNodeOutput{NodeID: nodeInfo.GetId(), Skipped: true}, nil
		}
		if _, err = deps.Offchain.DisableNode(ctx, &nodev1.DisableNodeRequest{Id: nodeInfo.GetId()}); err != nil {
			return DisableNodeOutput{}, fmt.Errorf("failed to disable node %q (%s): %w", nodeInfo.GetId(), nodeInfo.GetName(), err)
		}
		b.Logger.Infow("disabled node", "node_id", nodeInfo.GetId(), "name", nodeInfo.GetName())

		return DisableNodeOutput{NodeID: nodeInfo.GetId()}, nil
	},
)

// SeqJDRegisterNodes registers multiple nodes.
var SeqJDRegisterNodes = fwops.NewSequence(
	"seq-jd-register-nodes",
	semver.MustParse("1.0.0"),
	"Register multiple nodes",
	func(b fwops.Bundle, deps JDOpDeps, in RegisterNodesInput) (RegisterNodesOutput, error) {
		var registeredIDs, skippedIDs []string
		var failed int
		var merr error
		for _, n := range in.Nodes {
			report, err := fwops.ExecuteOperation(b, OpJDRegisterNode, deps, RegisterNodeInput{
				Domain:      in.Domain,
				Name:        n.Name,
				CSAKey:      n.CSAKey,
				IsBootstrap: n.IsBootstrap,
				Labels:      n.Labels,
			})
			if err != nil {
				failed++
				merr = errors.Join(merr, err)
				b.Logger.Errorw("failed to register node", "name", n.Name, "error", err)

				continue
			}
			if report.Output.Skipped {
				skippedIDs = append(skippedIDs, report.Output.NodeID)
			} else {
				registeredIDs = append(registeredIDs, report.Output.NodeID)
			}
		}
		b.Logger.Infow("register_nodes complete",
			"total", len(in.Nodes),
			"registered", len(registeredIDs),
			"skipped", len(skippedIDs),
			"failed", failed)

		return RegisterNodesOutput{RegisteredNodeIDs: registeredIDs, SkippedNodeIDs: skippedIDs}, merr
	},
)

// SeqJDUpdateNodes updates multiple nodes.
var SeqJDUpdateNodes = fwops.NewSequence(
	"seq-jd-update-nodes",
	semver.MustParse("1.0.0"),
	"Update multiple nodes",
	func(b fwops.Bundle, deps JDOpDeps, in UpdateNodesInput) (UpdateNodesOutput, error) {
		var updatedIDs []string
		var failed int
		var merr error
		for _, n := range in.Nodes {
			_, err := fwops.ExecuteOperation(b, OpJDUpdateNode, deps, UpdateNodeInput(n))
			if err != nil {
				failed++
				merr = errors.Join(merr, err)
				b.Logger.Errorw("failed to update node", "id", n.ID, "error", err)

				continue
			}
			updatedIDs = append(updatedIDs, n.ID)
		}
		b.Logger.Infow("update_nodes complete",
			"total", len(in.Nodes),
			"updated", len(updatedIDs),
			"failed", failed)

		return UpdateNodesOutput{UpdatedNodeIDs: updatedIDs}, merr
	},
)

// SeqJDDisableNodes disables multiple nodes by CSA key.
var SeqJDDisableNodes = fwops.NewSequence(
	"seq-jd-disable-nodes",
	semver.MustParse("1.0.0"),
	"Disable multiple nodes by CSA key",
	func(b fwops.Bundle, deps JDOpDeps, in DisableNodesInput) (DisableNodesOutput, error) {
		var disabledIDs, skippedCSAKeys []string
		var failed int
		var merr error
		for _, csaKey := range in.CSAKeys {
			report, err := fwops.ExecuteOperation(b, OpJDDisableNode, deps, DisableNodeInput{CSAKey: csaKey})
			if err != nil {
				failed++
				merr = errors.Join(merr, err)
				b.Logger.Errorw("failed to disable node", "csa_key", csaKey, "error", err)

				continue
			}
			if report.Output.Skipped {
				skippedCSAKeys = append(skippedCSAKeys, csaKey)
			} else {
				disabledIDs = append(disabledIDs, report.Output.NodeID)
			}
		}
		b.Logger.Infow("disable_nodes complete",
			"total", len(in.CSAKeys),
			"disabled", len(disabledIDs),
			"skipped", len(skippedCSAKeys),
			"failed", failed)

		return DisableNodesOutput{DisabledNodeIDs: disabledIDs, SkippedCSAKeys: skippedCSAKeys}, merr
	},
)
