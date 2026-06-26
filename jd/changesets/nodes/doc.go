// Package nodes provides changesets for managing Job Distributor nodes.
//
// # Usage
//
//	import "github.com/smartcontractkit/cld-changesets/jd/changesets/nodes"
//
//	// Register nodes (idempotent — already-registered nodes are skipped):
//	_, err := runtime.ExecChangeset(rt, nodes.RegisterNodesChangeset{}, nodes.RegisterNodesInput{
//		Domain: "keystone",
//		Nodes: []nodes.NodeToRegister{{
//			Name:   "oracle-1",
//			CSAKey: "csa-key-1",
//		}},
//	})
//
//	// Update node name and/or labels:
//	_, err = runtime.ExecChangeset(rt, nodes.UpdateNodesChangeset{}, nodes.UpdateNodesInput{
//		Nodes: []nodes.NodeToUpdate{{
//			ID:     "node-id-1",
//			CSAKey: "csa-key-1",
//			Name:   "oracle-1-updated",
//		}},
//	})
//
//	// Disable nodes by CSA key (idempotent — not-found and already-disabled are skipped):
//	_, err = runtime.ExecChangeset(rt, nodes.DisableNodesChangeset{}, nodes.DisableNodesInput{
//		CSAKeys: []string{"csa-key-1"},
//	})
package nodes
