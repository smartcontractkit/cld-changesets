package solana

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

func TestMCMSPDA(t *testing.T) {
	t.Parallel()

	programID := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")

	tests := []struct {
		name   string
		prefix string
		fn     func(programID solana.PublicKey, seed PDASeed) solana.PublicKey
	}{
		{name: "GetMCMSignerPDA", prefix: pdaPrefixMultisigSigner, fn: GetMCMSignerPDA},
		{name: "GetMCMConfigPDA", prefix: pdaPrefixMultisigConfig, fn: GetMCMConfigPDA},
		{name: "GetMCMRootMetadataPDA", prefix: pdaPrefixRootMetadata, fn: GetMCMRootMetadataPDA},
		{name: "GetMCMExpiringRootAndOpCountPDA", prefix: pdaPrefixExpiringRootAndOpCount, fn: GetMCMExpiringRootAndOpCountPDA},
		{name: "GetTimelockConfigPDA", prefix: pdaPrefixTimelockConfig, fn: GetTimelockConfigPDA},
		{name: "GetTimelockSignerPDA", prefix: pdaPrefixTimelockSigner, fn: GetTimelockSignerPDA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			seed := testPDASeed(t)
			seeds := [][]byte{[]byte(tt.prefix), seed[:]}
			want := mustFindPDA(t, seeds, programID)
			got := tt.fn(programID, seed)
			require.Equal(t, want, got)
		})
	}
}

func TestPDAGeneratorsUseDistinctSeeds(t *testing.T) {
	t.Parallel()
	programID := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	id := testPDASeed(t)

	signer := GetMCMSignerPDA(programID, id)
	cfg := GetMCMConfigPDA(programID, id)
	meta := GetMCMRootMetadataPDA(programID, id)
	exp := GetMCMExpiringRootAndOpCountPDA(programID, id)
	tlCfg := GetTimelockConfigPDA(programID, id)
	tlSigner := GetTimelockSignerPDA(programID, id)

	keys := []solana.PublicKey{signer, cfg, meta, exp, tlCfg, tlSigner}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			require.NotEqualf(t, keys[i], keys[j], "PDA at %d equals PDA at %d", i, j)
		}
	}
}

func mustFindPDA(t *testing.T, seeds [][]byte, programID solana.PublicKey) solana.PublicKey {
	t.Helper()
	pda, _, err := solana.FindProgramAddress(seeds, programID)
	require.NoError(t, err)

	return pda
}

func testPDASeed(t *testing.T) PDASeed {
	t.Helper()
	var s PDASeed
	for i := range s {
		s[i] = byte(i + 1)
	}

	return s
}
