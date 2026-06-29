package solgrantrole

import (
	"testing"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssdkmocks "github.com/smartcontractkit/mcms/sdk/mocks"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
)

func TestAddressesNeedingGrant(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	timelock := "timelock"
	grantee := solanago.NewWallet().PublicKey()
	pending := solanago.NewWallet().PublicKey()

	inspector := mcmssdkmocks.NewTimelockInspector(t)
	inspector.EXPECT().
		GetCancellers(ctx, timelock).
		Return([]string{grantee.String()}, nil)

	got, err := AddressesNeedingGrant(
		ctx,
		inspector,
		timelock,
		grantrole.RoleGrant{
			Role:      mcmssdk.TimelockRoleCanceller,
			Addresses: []string{grantee.String(), pending.String()},
		},
	)
	require.NoError(t, err)
	require.Equal(t, []solanago.PublicKey{pending}, got)
}
