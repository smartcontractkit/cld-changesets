package solsetconfig

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
)

func init() {
	setconfig.Register(Registration())
}

func Registration() setconfig.Registration {
	return setconfig.Registration{
		Family:   chainselectors.FamilySolana,
		Sequence: seqSetConfig,
		Verify:   verifySolanaChains,
	}
}

func verifySolanaChains(env cldf.Environment, chains []setconfig.ChainInput) error {
	for _, in := range chains {
		if _, ok := env.BlockChains.SolanaChains()[in.ChainSelector]; !ok {
			return fmt.Errorf("solana chain %d not found in environment", in.ChainSelector)
		}
		if err := validateMCMSIfPresent(env, in); err != nil {
			return err
		}
	}

	return nil
}
