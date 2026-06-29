package jobs

import (
	"testing"

	jobv1 "github.com/smartcontractkit/chainlink-protos/job-distributor/v1/job"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"

	jdnodes "github.com/smartcontractkit/cld-changesets/jd/changesets/nodes"
	jdops "github.com/smartcontractkit/cld-changesets/jd/operations"
)

const testJobTOML = `
type = "offchainreporting2"
name = "test-job"
contractID = "0x0000000000000000000000000000000000000001"
externalJobID = "00000000-0000-0000-0000-000000000001"
`

func registerTestNode(t *testing.T, rt *runtime.Runtime) string {
	t.Helper()
	_, err := runtime.ExecChangeset(rt, jdnodes.RegisterNodesChangeset{}, jdnodes.RegisterNodesInput{
		Domain: "keystone",
		Nodes: []jdnodes.NodeToRegister{{
			Name:   "oracle-1",
			CSAKey: "csa-key-1",
			Labels: map[string]string{"target": "oracle-1"},
		}},
	})
	require.NoError(t, err)
	node, err := jdops.ListNodeByPublicKey(t.Context(), rt.Environment().Offchain, "csa-key-1")
	require.NoError(t, err)
	require.NotNil(t, node, "node not found after registration")

	return node.GetId()
}

func proposeJob(t *testing.T, rt *runtime.Runtime, nodeID string) string {
	t.Helper()
	resp, err := rt.Environment().Offchain.ProposeJob(t.Context(), &jobv1.ProposeJobRequest{
		NodeId: nodeID,
		Spec:   testJobTOML,
	})
	require.NoError(t, err)

	return resp.GetProposal().GetJobId()
}
