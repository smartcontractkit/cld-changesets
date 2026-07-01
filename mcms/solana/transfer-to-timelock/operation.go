package soltransfertotimelock

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	solanago "github.com/gagliardetto/solana-go"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
)

// OpTransferToTimelockInput is the input for transferring one Solana contract to the timelock.
type OpTransferToTimelockInput struct {
	Contract            OwnableContract
	TimelockSignerPDA   solanago.PublicKey
	OnlyAcceptOwnership bool
}

// OpTransferToTimelockOutput is the output of a single contract transfer operation.
type OpTransferToTimelockOutput struct {
	BatchOps []mcmstypes.BatchOperation
}

// OpTransferToTimelock transfers one ownable Solana contract to the timelock signer PDA.
var OpTransferToTimelock = operations.NewOperation(
	"solana-transfer-to-timelock",
	semver.MustParse("1.0.0"),
	"Transfer ownable Solana contract ownership to the MCMS timelock",
	func(b operations.Bundle, chain cldfsol.Chain, in OpTransferToTimelockInput) (OpTransferToTimelockOutput, error) {
		var out OpTransferToTimelockOutput
		chainSelector := chain.Selector

		if !in.OnlyAcceptOwnership {
			if chain.DeployerKey == nil {
				return OpTransferToTimelockOutput{}, fmt.Errorf("missing deployer key for chain %d", chain.Selector)
			}

			transferInstruction, err := transferOwnershipInstruction(
				in.Contract.ProgramID,
				in.Contract.Seed,
				in.TimelockSignerPDA,
				in.Contract.OwnerPDA,
				chain.DeployerKey.PublicKey(),
			)
			if err != nil {
				return out, fmt.Errorf("create transfer ownership instruction: %w", err)
			}

			b.Logger.Infow(
				"confirming solana transfer ownership instruction",
				"program", in.Contract.ProgramID.String(),
				"contractType", in.Contract.Type,
			)
			if err = chain.Confirm([]solanago.Instruction{transferInstruction}); err != nil {
				return out, fmt.Errorf("confirm transfer ownership instruction: %w", err)
			}

			owner, err := contractOwner(b.GetContext(), chain, in.Contract)
			if err != nil {
				return out, fmt.Errorf("read contract owner after transfer: %w", err)
			}
			if owner == in.TimelockSignerPDA {
				b.Logger.Infof("contract %s already owned by timelock after transfer", in.Contract.Type)
				return out, nil
			}
		}

		acceptMCMSTx, err := acceptMCMSTransaction(in.Contract, in.TimelockSignerPDA)
		if err != nil {
			return out, fmt.Errorf("create accept ownership mcms transaction: %w", err)
		}

		out.BatchOps = append(out.BatchOps, mcmstypes.BatchOperation{
			ChainSelector: mcmstypes.ChainSelector(chainSelector),
			Transactions:  []mcmstypes.Transaction{acceptMCMSTx},
		})

		return out, nil
	},
)

func transferContractToTimelock(
	b operations.Bundle,
	chain cldfsol.Chain,
	timelockSignerPDA solanago.PublicKey,
	contract OwnableContract,
	in transfertotimelock.ChainInput,
) ([]mcmstypes.BatchOperation, error) {
	owner, err := contractOwner(b.GetContext(), chain, contract)
	if err != nil {
		return nil, fmt.Errorf("read contract owner: %w", err)
	}
	if owner == timelockSignerPDA {
		b.Logger.Infof("contract %s already owned by timelock", contract.Type)
		return nil, nil
	}

	report, err := operations.ExecuteOperation(
		b,
		OpTransferToTimelock,
		chain,
		OpTransferToTimelockInput{
			Contract:            contract,
			TimelockSignerPDA:   timelockSignerPDA,
			OnlyAcceptOwnership: in.OnlyAcceptOwnership,
		},
	)
	if err != nil {
		return nil, err
	}

	return report.Output.BatchOps, nil
}
