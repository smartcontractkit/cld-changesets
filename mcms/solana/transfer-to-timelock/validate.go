package soltransfertotimelock

import (
	"errors"
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
)

func validateMCMS(env cldf.Environment, in transfertotimelock.ChainInput) error {
	if in.MCMS == nil {
		return errors.New("MCMS timelock proposal input is required")
	}

	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilySolana)
	if !ok {
		return fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilySolana)
	}

	timelockRef, err := reader.GetTimelockRef(env, in.ChainSelector, *in.MCMS)
	if err != nil {
		return fmt.Errorf("timelock not present on chain %d: %w", in.ChainSelector, err)
	}
	if _, _, err = mcmssolanasdk.ParseContractAddress(timelockRef.Address); err != nil {
		return fmt.Errorf("invalid timelock ref on chain %d: %w", in.ChainSelector, err)
	}

	mcmsRef, err := reader.GetMCMSRef(env, in.ChainSelector, *in.MCMS)
	if err != nil {
		return fmt.Errorf("mcms not present on chain %d: %w", in.ChainSelector, err)
	}
	if _, _, err = mcmssolanasdk.ParseContractAddress(mcmsRef.Address); err != nil {
		return fmt.Errorf("invalid mcms ref on chain %d: %w", in.ChainSelector, err)
	}

	return nil
}

func validateContracts(env cldf.Environment, in transfertotimelock.ChainInput) error {
	if len(in.Contracts) == 0 {
		return errors.New("no contracts provided")
	}

	chain, err := requireSolanaChain(env, in.ChainSelector)
	if err != nil {
		return err
	}
	if chain.DeployerKey == nil {
		return fmt.Errorf("missing deployer key for chain %d", in.ChainSelector)
	}

	seen := make(map[string]struct{}, len(in.Contracts))
	for i, ref := range in.Contracts {
		key, keyErr := ref.Key()
		if keyErr != nil {
			return fmt.Errorf("contracts[%d]: %w", i, keyErr)
		}
		keyStr := fmt.Sprintf("%v", key)
		if _, dup := seen[keyStr]; dup {
			return fmt.Errorf("duplicate contract ref %v", key)
		}
		seen[keyStr] = struct{}{}
	}

	timelockSignerPDA, err := timelockSignerPDA(env, in)
	if err != nil {
		return err
	}

	deployer := chain.DeployerKey.PublicKey()
	ctx := env.GetContext()

	contracts := make([]OwnableContract, len(in.Contracts))
	seenContracts := make(map[string]struct{}, len(in.Contracts))
	for i, ref := range in.Contracts {
		contract, err := resolveOwnableContract(env, in.ChainSelector, ref)
		if err != nil {
			return fmt.Errorf("contracts[%d]: %w", i, err)
		}
		contractID := contract.OwnerPDA.String()
		if _, dup := seenContracts[contractID]; dup {
			return fmt.Errorf("duplicate contract %s", contractID)
		}
		seenContracts[contractID] = struct{}{}
		contracts[i] = contract
	}

	for _, contract := range contracts {
		owner, err := contractOwner(ctx, chain, contract)
		if err != nil {
			return fmt.Errorf("contract %s: %w", contract.Type, err)
		}
		if err := validateContractOwner(contract, owner, deployer, timelockSignerPDA, in.OnlyAcceptOwnership); err != nil {
			return err
		}
	}

	return nil
}
