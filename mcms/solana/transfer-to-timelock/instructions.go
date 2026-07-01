package soltransfertotimelock

import (
	"fmt"

	solanago "github.com/gagliardetto/solana-go"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	accessControllerBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/access_controller"
	mcmBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/mcm"

	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
)

func transferOwnershipInstruction(
	programID solanago.PublicKey,
	seed legacysolana.PDASeed,
	proposedOwner, ownerPDA, auth solanago.PublicKey,
) (solanago.Instruction, error) {
	if (seed == legacysolana.PDASeed{}) {
		return newSeedlessTransferOwnershipInstruction(programID, proposedOwner, ownerPDA, auth)
	}

	return newSeededTransferOwnershipInstruction(programID, seed, proposedOwner, ownerPDA, auth)
}

func acceptMCMSTransaction(contract OwnableContract, authority solanago.PublicKey) (mcmstypes.Transaction, error) {
	acceptInstruction, err := acceptOwnershipInstruction(contract.ProgramID, contract.Seed, contract.OwnerPDA, authority)
	if err != nil {
		return mcmstypes.Transaction{}, fmt.Errorf("build accept ownership instruction: %w", err)
	}

	acceptMCMSTx, err := mcmssolanasdk.NewTransactionFromInstruction(acceptInstruction, string(contract.Type), []string{})
	if err != nil {
		return mcmstypes.Transaction{}, fmt.Errorf("build mcms transaction from accept ownership instruction: %w", err)
	}

	return acceptMCMSTx, nil
}

func acceptOwnershipInstruction(
	programID solanago.PublicKey,
	seed legacysolana.PDASeed,
	ownerPDA, auth solanago.PublicKey,
) (solanago.Instruction, error) {
	if (seed == legacysolana.PDASeed{}) {
		return newSeedlessAcceptOwnershipInstruction(programID, ownerPDA, auth)
	}

	return newSeededAcceptOwnershipInstruction(programID, seed, ownerPDA, auth)
}

func newSeededTransferOwnershipInstruction(
	programID solanago.PublicKey,
	seed legacysolana.PDASeed,
	proposedOwner, config, authority solanago.PublicKey,
) (solanago.Instruction, error) {
	ix, err := mcmBindings.NewTransferOwnershipInstruction(seed, proposedOwner, config, authority).ValidateAndBuild()

	return &seededInstruction{Instruction: ix, programID: programID}, err
}

func newSeededAcceptOwnershipInstruction(
	programID solanago.PublicKey,
	seed legacysolana.PDASeed,
	config, authority solanago.PublicKey,
) (solanago.Instruction, error) {
	ix, err := mcmBindings.NewAcceptOwnershipInstruction(seed, config, authority).ValidateAndBuild()

	return &seededInstruction{Instruction: ix, programID: programID}, err
}

func newSeedlessTransferOwnershipInstruction(
	programID, proposedOwner, config, authority solanago.PublicKey,
) (solanago.Instruction, error) {
	ix, err := accessControllerBindings.NewTransferOwnershipInstruction(proposedOwner, config, authority).ValidateAndBuild()

	return &seedlessInstruction{Instruction: ix, programID: programID}, err
}

func newSeedlessAcceptOwnershipInstruction(
	programID, config, authority solanago.PublicKey,
) (solanago.Instruction, error) {
	ix, err := accessControllerBindings.NewAcceptOwnershipInstruction(config, authority).ValidateAndBuild()

	return &seedlessInstruction{Instruction: ix, programID: programID}, err
}

type seedlessInstruction struct {
	*accessControllerBindings.Instruction
	programID solanago.PublicKey
}

func (s *seedlessInstruction) ProgramID() solanago.PublicKey {
	return s.programID
}

type seededInstruction struct {
	*mcmBindings.Instruction
	programID solanago.PublicKey
}

func (s *seededInstruction) ProgramID() solanago.PublicKey {
	return s.programID
}
