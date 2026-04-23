package solana

import (
	"github.com/gagliardetto/solana-go"
)

const (
	pdaPrefixMultisigSigner         = "multisig_signer"
	pdaPrefixMultisigConfig         = "multisig_config"
	pdaPrefixRootMetadata           = "root_metadata"
	pdaPrefixExpiringRootAndOpCount = "expiring_root_and_op_count"
	pdaPrefixTimelockConfig         = "timelock_config"
	pdaPrefixTimelockSigner         = "timelock_signer"
)

// GetMCMSignerPDA returns the PDA for the MCMS signer
func GetMCMSignerPDA(programID solana.PublicKey, seed PDASeed) solana.PublicKey {
	seeds := [][]byte{[]byte(pdaPrefixMultisigSigner), seed[:]}
	return getPDA(programID, seeds)
}

// GetMCMConfigPDA returns the PDA for the MCMS config
func GetMCMConfigPDA(programID solana.PublicKey, seed PDASeed) solana.PublicKey {
	seeds := [][]byte{[]byte(pdaPrefixMultisigConfig), seed[:]}
	return getPDA(programID, seeds)
}

// GetMCMRootMetadataPDA returns the PDA for the MCMS root metadata
func GetMCMRootMetadataPDA(programID solana.PublicKey, seed PDASeed) solana.PublicKey {
	seeds := [][]byte{[]byte(pdaPrefixRootMetadata), seed[:]}
	return getPDA(programID, seeds)
}

// GetMCMExpiringRootAndOpCountPDA returns the PDA for the MCMS expiring root and op count
func GetMCMExpiringRootAndOpCountPDA(programID solana.PublicKey, seed PDASeed) solana.PublicKey {
	seeds := [][]byte{[]byte(pdaPrefixExpiringRootAndOpCount), seed[:]}
	return getPDA(programID, seeds)
}

// GetTimelockConfigPDA returns the PDA for the Timelock config
func GetTimelockConfigPDA(programID solana.PublicKey, seed PDASeed) solana.PublicKey {
	seeds := [][]byte{[]byte(pdaPrefixTimelockConfig), seed[:]}
	return getPDA(programID, seeds)
}

// GetTimelockSignerPDA returns the PDA for the Timelock signer
func GetTimelockSignerPDA(programID solana.PublicKey, seed PDASeed) solana.PublicKey {
	seeds := [][]byte{[]byte(pdaPrefixTimelockSigner), seed[:]}
	return getPDA(programID, seeds)
}

// getPDA returns the PDA for the given program ID and seeds
func getPDA(programID solana.PublicKey, seeds [][]byte) solana.PublicKey {
	// todo(ggoh): add error handling
	pda, _, _ := solana.FindProgramAddress(seeds, programID)
	return pda
}
