package mcmsrole

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestNewRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		roleName string
		wantID   common.Hash
	}{
		{
			name:     "admin",
			roleName: "ADMIN_ROLE",
			wantID:   common.HexToHash("0xa49807205ce4d355092ef5a8a18f56e8913cf4a201fbe287825b095693c21775"),
		},
		{
			name:     "proposer",
			roleName: "PROPOSER_ROLE",
			wantID:   common.HexToHash("0xb09aa5aeb3702cfd50b6b62bc4532604938f21248a27a1d5ca736082b6819cc1"),
		},
		{
			name:     "executor",
			roleName: "EXECUTOR_ROLE",
			wantID:   common.HexToHash("0xd8aa0f3194971a2a116679f7c2090f6939c8d4e01a2a8d7e41d55e5351469e63"),
		},
		{
			name:     "bypasser",
			roleName: "BYPASSER_ROLE",
			wantID:   common.HexToHash("0xa1b2b8005de234c4b8ce8cd0be058239056e0d54f6097825b5117101469d5a8d"),
		},
		{
			name:     "canceller",
			roleName: "CANCELLER_ROLE",
			wantID:   common.HexToHash("0xfd643c72710c63c0180259aba6b2d05451e3591a24e58b62239378085726f783"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewRole(tt.roleName)
			require.Equal(t, tt.roleName, got.Name)
			require.Equal(t, tt.wantID, got.ID)
		})
	}
}

func TestPredefinedRoles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role Role
	}{
		{name: "admin", role: AdminRole},
		{name: "proposer", role: ProposerRole},
		{name: "executor", role: ExecutorRole},
		{name: "bypasser", role: BypasserRole},
		{name: "canceller", role: CancellerRole},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, NewRole(tt.role.Name), tt.role)
		})
	}
}
