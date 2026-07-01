package soltransfertotimelock

import (
	"fmt"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

// OwnableContract identifies a Solana ownable program account to transfer.
type OwnableContract struct {
	ProgramID solanago.PublicKey
	Seed      legacysolana.PDASeed
	OwnerPDA  solanago.PublicKey
	Type      cldf.ContractType
}

func resolveOwnableContract(env cldf.Environment, chainSelector uint64, ref refkey.RefKey) (OwnableContract, error) {
	if ref.ChainSelector != 0 && ref.ChainSelector != chainSelector {
		return OwnableContract{}, fmt.Errorf(
			"ref chain selector %d does not match chain %d",
			ref.ChainSelector,
			chainSelector,
		)
	}
	if ref.ChainSelector == 0 {
		ref.ChainSelector = chainSelector
	}

	resolved, err := ref.Resolve(env)
	if err != nil {
		return OwnableContract{}, err
	}

	contractType := cldf.ContractType(resolved.Type)
	programID, seed, err := legacysolana.DecodeAddressWithSeed(resolved.Address)
	if err != nil {
		account, parseErr := solanago.PublicKeyFromBase58(resolved.Address)
		if parseErr != nil {
			return OwnableContract{}, fmt.Errorf("parse contract address %q: %w", resolved.Address, parseErr)
		}

		acProgram, acErr := accessControllerProgramFromDatastore(env, chainSelector, ref.Qualifier)
		if acErr != nil {
			return OwnableContract{}, acErr
		}

		return OwnableContract{
			ProgramID: acProgram,
			OwnerPDA:  account,
			Type:      contractType,
		}, nil
	}

	ownerPDA, err := ownerPDAForSeededContract(programID, seed, contractType)
	if err != nil {
		return OwnableContract{}, err
	}

	return OwnableContract{
		ProgramID: programID,
		Seed:      seed,
		OwnerPDA:  ownerPDA,
		Type:      contractType,
	}, nil
}

func ownerPDAForSeededContract(
	programID solanago.PublicKey,
	seed legacysolana.PDASeed,
	contractType cldf.ContractType,
) (solanago.PublicKey, error) {
	switch contractType {
	case mcmscontracts.ProposerManyChainMultisig,
		mcmscontracts.CancellerManyChainMultisig,
		mcmscontracts.BypasserManyChainMultisig:
		return familysolana.GetMCMConfigPDA(programID, seed), nil
	case mcmscontracts.RBACTimelock:
		return familysolana.GetTimelockConfigPDA(programID, seed), nil
	default:
		return solanago.PublicKey{}, fmt.Errorf("unsupported seeded contract type %q for transfer to timelock", contractType)
	}
}

func accessControllerProgramFromDatastore(env cldf.Environment, chainSelector uint64, qualifier string) (solanago.PublicKey, error) {
	if env.DataStore == nil {
		return solanago.PublicKey{}, fmt.Errorf("datastore not available for chain %d", chainSelector)
	}

	ref, err := datastore.FindUniqueRef(env.DataStore.Addresses(), datastore.AddressRef{
		ChainSelector: chainSelector,
		Type:          datastore.ContractType(mcmscontracts.AccessControllerProgram),
		Qualifier:     qualifier,
	})
	if err != nil {
		return solanago.PublicKey{}, fmt.Errorf("resolve access controller program for chain %d: %w", chainSelector, err)
	}

	programID, err := solanago.PublicKeyFromBase58(ref.Address)
	if err != nil {
		return solanago.PublicKey{}, fmt.Errorf("parse access controller program for chain %d: %w", chainSelector, err)
	}

	return programID, nil
}
