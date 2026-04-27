package solana

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"

	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
)

// FundFromAddressIxs builds system program transfer instructions that send lamports from
// the given sender to each recipient. It does not submit or confirm a transaction.
func FundFromAddressIxs(from solana.PublicKey, recipients []solana.PublicKey, amount uint64) ([]solana.Instruction, error) {
	if len(recipients) == 0 {
		return nil, nil
	}
	ixs := make([]solana.Instruction, len(recipients))
	for i, recipient := range recipients {
		ix, err := system.NewTransferInstruction(
			amount,
			from,
			recipient,
		).ValidateAndBuild()
		if err != nil {
			return nil, fmt.Errorf("transfer instruction recipient[%d] %s: %w", i, recipient.String(), err)
		}
		ixs[i] = ix
	}

	return ixs, nil
}

// FundFromDeployerKey transfers SOL from the deployer to each recipient and waits for confirmations.
func FundFromDeployerKey(solChain cldf_solana.Chain, recipients []solana.PublicKey, amount uint64) error {
	ixs, err := FundFromAddressIxs(solChain.DeployerKey.PublicKey(), recipients, amount)
	if err != nil {
		return fmt.Errorf("failed to create transfer instructions: %w", err)
	}
	if len(ixs) == 0 {
		return nil
	}
	err = solChain.Confirm(ixs)
	if err != nil {
		return fmt.Errorf("failed to confirm transaction: %w", err)
	}

	return nil
}
