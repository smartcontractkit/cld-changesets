package solgrantrole

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
)

func init() {
	grantrole.Registry.Register(Registration())
}

func Registration() grantrole.Registration {
	return grantrole.Registration{
		Family:   chainselectors.FamilySolana,
		Sequence: seqGrantRole,
		Verify:   verifySolanaChains,
	}
}

func verifySolanaChains(env cldf.Environment, chains []grantrole.SeqInput) error {
	for _, in := range chains {
		if _, ok := env.BlockChains.SolanaChains()[in.ChainSelector]; !ok {
			return fmt.Errorf("solana chain %d not found in environment", in.ChainSelector)
		}
		if err := validateMCMSIfPresent(env, in); err != nil {
			return err
		}
		if err := validateRoles(in); err != nil {
			return fmt.Errorf("chain %d: %w", in.ChainSelector, err)
		}
		if err := validateGrantAddresses(env, in); err != nil {
			return fmt.Errorf("chain %d: %w", in.ChainSelector, err)
		}
	}

	return nil
}
