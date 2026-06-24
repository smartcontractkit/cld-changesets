package evmtransfertotimelock

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestParseEVMAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		addr    string
		label   string
		want    common.Address
		wantErr string
	}{
		{
			name:  "valid address",
			addr:  "0x0000000000000000000000000000000000000abc",
			label: "timelock",
			want:  common.HexToAddress("0xabc"),
		},
		{
			name:    "invalid hex",
			addr:    "not-an-address",
			label:   "timelock",
			wantErr: `invalid timelock address "not-an-address"`,
		},
		{
			name:    "zero address",
			addr:    "0x0000000000000000000000000000000000000000",
			label:   "mcms",
			wantErr: "mcms address must not be zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseEVMAddress(tt.addr, tt.label)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)

				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
