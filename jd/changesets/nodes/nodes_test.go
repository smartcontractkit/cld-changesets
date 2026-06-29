package nodes

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"

	jdops "github.com/smartcontractkit/cld-changesets/jd/operations"
)

func nodeIDByCSAKey(t *testing.T, rt *runtime.Runtime, csaKey string) string {
	t.Helper()
	node, err := jdops.ListNodeByPublicKey(t.Context(), rt.Environment().Offchain, csaKey)
	require.NoError(t, err)
	require.NotNil(t, node, "no node found with csa_key %q", csaKey)

	return node.GetId()
}
