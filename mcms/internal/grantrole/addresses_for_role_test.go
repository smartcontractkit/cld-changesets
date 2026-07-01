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
			name: "proposer returns proposers",
			role: mcmssdk.TimelockRoleProposer,
			setup: func(t *testing.T, ctx context.Context, inspector *mcmssdkmocks.TimelockInspector, timelock string) {
				t.Helper()
				inspector.EXPECT().
					GetProposers(ctx, timelock).
					Return([]string{"proposer-1"}, nil)
			},
			want: []string{"proposer-1"},
		},
		{
			name: "canceller returns cancellers",
			role: mcmssdk.TimelockRoleCanceller,
			setup: func(t *testing.T, ctx context.Context, inspector *mcmssdkmocks.TimelockInspector, timelock string) {
				t.Helper()
				inspector.EXPECT().
					GetCancellers(ctx, timelock).
					Return([]string{"canceller-1"}, nil)
			},
			want: []string{"canceller-1"},
		},
		{
			name: "bypasser returns bypassers",
			role: mcmssdk.TimelockRoleBypasser,
			setup: func(t *testing.T, ctx context.Context, inspector *mcmssdkmocks.TimelockInspector, timelock string) {
				t.Helper()
				inspector.EXPECT().
					GetBypassers(ctx, timelock).
					Return([]string{"bypasser-1"}, nil)
			},
			want: []string{"bypasser-1"},
		},
		{
			name: "executor returns executors",
			role: mcmssdk.TimelockRoleExecutor,
			setup: func(t *testing.T, ctx context.Context, inspector *mcmssdkmocks.TimelockInspector, timelock string) {
				t.Helper()
				inspector.EXPECT().
					GetExecutors(ctx, timelock).
					Return([]string{"executor-1"}, nil)
			},
			want: []string{"executor-1"},
		},
		{
			name: "admin returns nil without query",
			role: mcmssdk.TimelockRoleAdmin,
			want: nil,
		},
		{
			name:    "unsupported role",
			role:    mcmssdk.TimelockRole(99),
			wantErr: "unsupported timelock role Unknown",
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
