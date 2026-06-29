package nodes

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
)

func TestDisableNodesChangeset(t *testing.T) {
	t.Parallel()

	t.Run("disables a registered node", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, RegisterNodesChangeset{}, RegisterNodesInput{
			Domain: "keystone",
			Nodes:  []NodeToRegister{{Name: "oracle-1", CSAKey: "csa-key-1"}},
		})
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, DisableNodesChangeset{}, DisableNodesInput{
			CSAKeys: []string{"csa-key-1"},
		})
		require.NoError(t, err)
	})

	t.Run("skips nodes not found — Skipped=true, no error", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, DisableNodesChangeset{}, DisableNodesInput{
			CSAKeys: []string{"nonexistent-key"},
		})
		require.NoError(t, err)
	})

	t.Run("precondition — empty CSA keys rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, DisableNodesChangeset{}, DisableNodesInput{})
		require.ErrorContains(t, err, "no csa_keys provided")
	})

	t.Run("precondition — duplicate CSA key rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, DisableNodesChangeset{}, DisableNodesInput{
			CSAKeys: []string{"csa-key-1", "csa-key-1"},
		})
		require.ErrorContains(t, err, "duplicate csa_key")
	})
}
