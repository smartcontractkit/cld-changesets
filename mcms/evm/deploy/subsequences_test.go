package evmdeploy

import (
	"testing"

	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/stretchr/testify/require"

	mcmops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/many_chain_multi_sig"
)

func TestMcmTypeVersion(t *testing.T) {
	t.Parallel()

	t.Run("proposer", func(t *testing.T) {
		t.Parallel()
		got, err := mcmTypeVersion(mcmscontracts.ProposerManyChainMultisig)
		require.NoError(t, err)
		require.Equal(t, mcmops.ProposerManyChainMultiSigTypeAndVersion, got)
	})

	t.Run("bypasser", func(t *testing.T) {
		t.Parallel()
		got, err := mcmTypeVersion(mcmscontracts.BypasserManyChainMultisig)
		require.NoError(t, err)
		require.Equal(t, mcmops.BypasserManyChainMultiSigTypeAndVersion, got)
	})

	t.Run("canceller", func(t *testing.T) {
		t.Parallel()
		got, err := mcmTypeVersion(mcmscontracts.CancellerManyChainMultisig)
		require.NoError(t, err)
		require.Equal(t, mcmops.CancellerManyChainMultiSigTypeAndVersion, got)
	})

	t.Run("unsupported", func(t *testing.T) {
		t.Parallel()
		_, err := mcmTypeVersion(mcmscontracts.RBACTimelock)
		require.ErrorContains(t, err, "unsupported contract type")
	})
}
