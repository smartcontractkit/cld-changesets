package nodes

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
)

func TestRegisterNodesChangeset(t *testing.T) {
	t.Parallel()

	t.Run("registers new nodes", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, RegisterNodesChangeset{}, RegisterNodesInput{
			Domain: "keystone",
			Nodes: []NodeToRegister{
				{Name: "oracle-1", CSAKey: "csa-key-1"},
				{Name: "oracle-2", CSAKey: "csa-key-2", IsBootstrap: true},
			},
		})
		require.NoError(t, err)

		for _, csaKey := range []string{"csa-key-1", "csa-key-2"} {
			require.NotEmpty(t, nodeIDByCSAKey(t, rt, csaKey))
		}
	})

	t.Run("idempotent — skips already registered nodes", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		input := RegisterNodesInput{
			Domain: "keystone",
			Nodes:  []NodeToRegister{{Name: "oracle-1", CSAKey: "csa-key-1"}},
		}
		_, err = runtime.ExecChangeset(rt, RegisterNodesChangeset{}, input)
		require.NoError(t, err)
		_, err = runtime.ExecChangeset(rt, RegisterNodesChangeset{}, input)
		require.NoError(t, err)
		require.NotEmpty(t, nodeIDByCSAKey(t, rt, "csa-key-1"))
	})

	t.Run("precondition — empty domain rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, RegisterNodesChangeset{}, RegisterNodesInput{
			Nodes: []NodeToRegister{{Name: "oracle-1", CSAKey: "csa-key-1"}},
		})
		require.ErrorContains(t, err, "domain is required")
	})

	t.Run("precondition — duplicate node name rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, RegisterNodesChangeset{}, RegisterNodesInput{
			Domain: "keystone",
			Nodes: []NodeToRegister{
				{Name: "oracle-1", CSAKey: "csa-key-1"},
				{Name: "oracle-1", CSAKey: "csa-key-2"},
			},
		})
		require.ErrorContains(t, err, "duplicate node name")
	})

	t.Run("precondition — duplicate CSA key rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, RegisterNodesChangeset{}, RegisterNodesInput{
			Domain: "keystone",
			Nodes: []NodeToRegister{
				{Name: "oracle-1", CSAKey: "csa-key-1"},
				{Name: "oracle-2", CSAKey: "csa-key-1"},
			},
		})
		require.ErrorContains(t, err, "duplicate csa_key")
	})
}
