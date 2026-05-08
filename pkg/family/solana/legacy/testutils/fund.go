package soltestutils

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	cldfsolana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"

	"github.com/smartcontractkit/cld-changesets/pkg/family/solana/legacy/solutils"

	pdasol "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
	solstate "github.com/smartcontractkit/cld-changesets/pkg/family/solana/legacy"
)

// FundSignerPDAs funds the timelock signer and MCMS signer PDAs with 1 SOL for testing
func FundSignerPDAs(
	t *testing.T, chain cldfsolana.Chain, mcmsState *solstate.MCMSWithTimelockState,
) {
	t.Helper()

	timelockSignerPDA := pdasol.GetTimelockSignerPDA(mcmsState.TimelockProgram, mcmsState.TimelockSeed)
	mcmSignerPDA := pdasol.GetMCMSignerPDA(mcmsState.McmProgram, mcmsState.ProposerMcmSeed)
	signerPDAs := []solana.PublicKey{timelockSignerPDA, mcmSignerPDA}
	err := solutils.FundAccounts(t.Context(), chain.Client, signerPDAs, 1)
	require.NoError(t, err)
}
