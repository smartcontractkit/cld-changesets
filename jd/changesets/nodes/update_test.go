package nodes

import (
	"testing"

	nodev1 "github.com/smartcontractkit/chainlink-protos/job-distributor/v1/node"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
)

func TestUpdateNodesChangeset(t *testing.T) {
	t.Parallel()

	registerNode := func(t *testing.T, rt *runtime.Runtime, name, csaKey string) string {
		t.Helper()
		_, err := runtime.ExecChangeset(rt, RegisterNodesChangeset{}, RegisterNodesInput{
			Domain: "keystone",
			Nodes:  []NodeToRegister{{Name: name, CSAKey: csaKey}},
		})
		require.NoError(t, err)

		return nodeIDByCSAKey(t, rt, csaKey)
	}

	t.Run("updates node name and labels", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		nodeID := registerNode(t, rt, "oracle-1", "csa-key-1")

		_, err = runtime.ExecChangeset(rt, UpdateNodesChangeset{}, UpdateNodesInput{
			Nodes: []NodeToUpdate{{
				ID:     nodeID,
				CSAKey: "csa-key-1",
				Name:   "oracle-1-updated",
				Labels: map[string]string{"env": "staging"},
			}},
		})
		require.NoError(t, err)

		resp, err := rt.Environment().Offchain.GetNode(t.Context(), &nodev1.GetNodeRequest{Id: nodeID})
		require.NoError(t, err)
		require.Equal(t, "oracle-1-updated", resp.GetNode().GetName())
	})

	t.Run("CSA key conflict — rejects reassignment to another node's key", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(environment.WithName("test")))
		require.NoError(t, err)

		node1ID := registerNode(t, rt, "oracle-1", "csa-key-1")
		registerNode(t, rt, "oracle-2", "csa-key-2")

		_, err = runtime.ExecChangeset(rt, UpdateNodesChangeset{}, UpdateNodesInput{
			Nodes: []NodeToUpdate{{ID: node1ID, CSAKey: "csa-key-2"}},
		})
		require.Error(t, err)
		require.ErrorContains(t, err, "CSA key")
	})

	t.Run("precondition — empty node list rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, UpdateNodesChangeset{}, UpdateNodesInput{})
		require.ErrorContains(t, err, "no nodes provided")
	})
}
