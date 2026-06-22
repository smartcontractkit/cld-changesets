package solfiredrill

import (
	"fmt"
	"math/big"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	firedrill "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill"
)

var seqFireDrill = operations.NewSequence(
	"seq-solana-firedrill",
	&semvers.V1_0_0,
	"Build noop MCMS fire-drill batch operation for a Solana chain",
	runSolanaFireDrill,
)

func runSolanaFireDrill(
	_ operations.Bundle,
	deps firedrill.Deps,
	in firedrill.ChainInput,
) (sequenceutils.OnChainOutput, error) {
	if _, ok := deps.BlockChains.SolanaChains()[in.ChainSelector]; !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("solana chain %d not found in environment", in.ChainSelector)
	}

	tx, err := buildNoOPSolana()
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}

	return sequenceutils.OnChainOutput{
		BatchOps: []mcmstypes.BatchOperation{{
			ChainSelector: mcmstypes.ChainSelector(in.ChainSelector),
			Transactions:  []mcmstypes.Transaction{tx},
		}},
	}, nil
}

func buildNoOPSolana() (mcmstypes.Transaction, error) {
	memo := []byte("noop")

	tx, err := mcmssolanasdk.NewTransaction(
		solanago.MemoProgramID.String(),
		memo,
		big.NewInt(0),
		[]*solanago.AccountMeta{},
		"Memo",
		[]string{},
	)
	if err != nil {
		return mcmstypes.Transaction{}, err
	}

	return tx, nil
}
