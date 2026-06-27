package grantrole

import (
	"testing"

	"github.com/stretchr/testify/require"

	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssdkmocks "github.com/smartcontractkit/mcms/sdk/mocks"
)

func TestAddressesForRole_unsupported(t *testing.T) {
	t.Parallel()

	inspector := mcmssdkmocks.NewTimelockInspector(t)

	_, err := AddressesForRole(t.Context(), inspector, "timelock", mcmssdk.TimelockRole(99))
	require.EqualError(t, err, "unsupported timelock role Unknown")
}

func TestAddressesForRole_admin(t *testing.T) {
	t.Parallel()

	inspector := mcmssdkmocks.NewTimelockInspector(t)

	got, err := AddressesForRole(t.Context(), inspector, "timelock", mcmssdk.TimelockRoleAdmin)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestAddressesNeedingGrant(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	timelock := "timelock"

	inspector := mcmssdkmocks.NewTimelockInspector(t)
	inspector.EXPECT().
		GetCancellers(ctx, timelock).
		Return([]string{"alice", "bob"}, nil)

	got, err := AddressesNeedingGrant(
		ctx,
		inspector,
		timelock,
		RoleGrant{
			Role:      mcmssdk.TimelockRoleCanceller,
			Addresses: []string{"alice", "carol"},
		},
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, []string{"carol"}, got)

	adminInspector := mcmssdkmocks.NewTimelockInspector(t)
	all, err := AddressesNeedingGrant(
		ctx,
		adminInspector,
		timelock,
		RoleGrant{
			Role:      mcmssdk.TimelockRoleAdmin,
			Addresses: []string{"alice"},
		},
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, []string{"alice"}, all)
}
