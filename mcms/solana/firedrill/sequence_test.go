package solfiredrill

import (
	"testing"
	"time"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	firedrill "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill"
)

func testSolanaChainInput(selector uint64) firedrill.ChainInput {
	return firedrill.ChainInput{
		ChainSelector: selector,
		MCMS: cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			TimelockDelay:  mcmstypes.NewDuration(time.Second),
		},
	}
}

func TestRunSolanaFireDrill_errors(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	input := testSolanaChainInput(selector)

	_, err := runSolanaFireDrill(
		optest.NewBundle(t),
		firedrill.Deps{BlockChains: chain.NewBlockChains(nil)},
		input,
	)
	require.ErrorContains(t, err, "solana chain")
}

func TestRunSolanaFireDrill_success(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	input := testSolanaChainInput(selector)

	out, err := runSolanaFireDrill(
		optest.NewBundle(t),
		firedrill.Deps{
			BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
				selector: cldf_solana.Chain{Selector: selector},
			}),
		},
		input,
	)
	require.NoError(t, err)
	require.Len(t, out.BatchOps, 1)
	require.Equal(t, mcmstypes.ChainSelector(selector), out.BatchOps[0].ChainSelector)
	require.Len(t, out.BatchOps[0].Transactions, 1)
	require.Equal(t, "Memo", out.BatchOps[0].Transactions[0].ContractType)
	require.Equal(t, solanago.MemoProgramID.String(), out.BatchOps[0].Transactions[0].To)
}

func TestBuildNoOPSolana(t *testing.T) {
	t.Parallel()

	tx, err := buildNoOPSolana()
	require.NoError(t, err)
	require.Equal(t, "Memo", tx.ContractType)
	require.Equal(t, solanago.MemoProgramID.String(), tx.To)
}
