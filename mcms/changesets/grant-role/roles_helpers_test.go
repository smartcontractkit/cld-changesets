package grantrole

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssdkmocks "github.com/smartcontractkit/mcms/sdk/mocks"
)

func TestAddressesForRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		role    mcmssdk.TimelockRole
		setup   func(t *testing.T, ctx context.Context, inspector *mcmssdkmocks.TimelockInspector, timelock string)
		want    []string
		wantErr string
	}{
		{
			name:    "unsupported role",
			role:    mcmssdk.TimelockRole(99),
			wantErr: "unsupported timelock role Unknown",
		},
		{
			name: "admin returns nil without query",
			role: mcmssdk.TimelockRoleAdmin,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			timelock := "timelock"
			inspector := mcmssdkmocks.NewTimelockInspector(t)
			if tt.setup != nil {
				tt.setup(t, ctx, inspector, timelock)
			}

			got, err := AddressesForRole(ctx, inspector, timelock, tt.role)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAddressesNeedingGrant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, ctx context.Context, inspector *mcmssdkmocks.TimelockInspector, timelock string)
		grant   RoleGrant
		want    []string
		wantErr string
	}{
		{
			name: "filters addresses that already hold role",
			setup: func(t *testing.T, ctx context.Context, inspector *mcmssdkmocks.TimelockInspector, timelock string) {
				t.Helper()
				inspector.EXPECT().
					GetCancellers(ctx, timelock).
					Return([]string{"alice", "bob"}, nil)
			},
			grant: RoleGrant{
				Role:      mcmssdk.TimelockRoleCanceller,
				Addresses: []string{"alice", "carol"},
			},
			want: []string{"carol"},
		},
		{
			name: "admin role returns all grant addresses without query",
			grant: RoleGrant{
				Role:      mcmssdk.TimelockRoleAdmin,
				Addresses: []string{"alice"},
			},
			want: []string{"alice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			timelock := "timelock"
			inspector := mcmssdkmocks.NewTimelockInspector(t)
			if tt.setup != nil {
				tt.setup(t, ctx, inspector, timelock)
			}

			got, err := AddressesNeedingGrant(ctx, inspector, timelock, tt.grant)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
