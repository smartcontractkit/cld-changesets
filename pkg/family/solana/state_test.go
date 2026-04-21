package solana

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	"github.com/stretchr/testify/require"
)

func TestMaybeLoadMCMSWithTimelockChainState_NoMatchingRefs(t *testing.T) {
	t.Parallel()

	t.Run("nil refs", func(t *testing.T) {
		t.Parallel()
		got, err := maybeLoadMCMSWithTimelockChainState(nil)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.NotNil(t, got.MCMSWithTimelockPrograms)
		require.Equal(t, solana.PublicKey{}, got.McmProgram)
	})

	t.Run("empty refs", func(t *testing.T) {
		t.Parallel()
		got, err := maybeLoadMCMSWithTimelockChainState([]datastore.AddressRef{})
		require.NoError(t, err)
		require.NotNil(t, got)
		require.NotNil(t, got.MCMSWithTimelockPrograms)
		require.Equal(t, solana.PublicKey{}, got.McmProgram)
	})
}
