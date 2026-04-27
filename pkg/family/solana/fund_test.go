package solana

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/stretchr/testify/require"
)

func TestFundFromAddressIxs(t *testing.T) {
	t.Parallel()

	from := solana.MustPublicKeyFromBase58("Vote111111111111111111111111111111111111111")
	r1 := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	r2 := solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	recipients := []solana.PublicKey{r1, r2}
	const amount uint64 = 42_000_000

	ixs, err := FundFromAddressIxs(from, recipients, amount)
	require.NoError(t, err)
	require.Len(t, ixs, len(recipients))
	for i, ix := range ixs {
		require.True(t, ix.ProgramID().Equals(system.ProgramID), "instruction %d program id", i)
		accts := ix.Accounts()
		require.Len(t, accts, 2)
		require.True(t, accts[0].PublicKey.Equals(from), "instruction %d sender", i)
		require.True(t, accts[1].PublicKey.Equals(recipients[i]), "instruction %d recipient", i)

		data, err := ix.Data()
		require.NoError(t, err)
		decoded, err := system.DecodeInstruction(accts, data)
		require.NoError(t, err)
		tr, ok := decoded.Impl.(*system.Transfer)
		require.True(t, ok, "instruction %d should decode as Transfer", i)
		require.NotNil(t, tr.Lamports)
		require.Equal(t, amount, *tr.Lamports)
	}
}

func TestFundFromAddressIxs_noRecipients(t *testing.T) {
	t.Parallel()

	from := solana.MustPublicKeyFromBase58("Vote111111111111111111111111111111111111111")

	ixs, err := FundFromAddressIxs(from, nil, 42)
	require.NoError(t, err)
	require.Nil(t, ixs)

	ixs, err = FundFromAddressIxs(from, []solana.PublicKey{}, 42)
	require.NoError(t, err)
	require.Nil(t, ixs)
}
