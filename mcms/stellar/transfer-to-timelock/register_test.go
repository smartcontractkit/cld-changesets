package stellartransfertotimelock

import (
	"testing"

	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
)

func TestRegistration(t *testing.T) {
	t.Parallel()

	reg := Registration()

	require.Equal(t, chainselectors.FamilyStellar, reg.Family)
	require.Equal(t, seqTransferToTimelock, reg.Sequence)
	require.NotNil(t, reg.Verify)
}

func TestInitRegistersStellar(t *testing.T) {
	t.Parallel()

	require.Contains(
		t,
		transfertotimelock.Registry.RegisteredFamilies(),
		chainselectors.FamilyStellar,
	)
}
