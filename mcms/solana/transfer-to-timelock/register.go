package soltransfertotimelock

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
)

func init() {
	transfertotimelock.Registry.Register(Registration())
}

// Registration returns the Solana chain-family transfer-to-timelock registration.
func Registration() transfertotimelock.Registration {
	return transfertotimelock.Registration{
		Family:   chainselectors.FamilySolana,
		Sequence: seqTransferToTimelock,
		Verify:   verifySolanaChains,
	}
}

func verifySolanaChains(env cldf.Environment, chains []transfertotimelock.ChainInput) error {
	for _, in := range chains {
		if err := validateMCMS(env, in); err != nil {
			return err
		}
		if err := validateContracts(env, in); err != nil {
			return fmt.Errorf("chain %d: %w", in.ChainSelector, err)
		}
	}

	return nil
}
