package evmsetconfig

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
)

// init auto-registers the EVM family when this package is imported.
func init() {
	setconfig.Register(Registration())
}

// Registration returns the EVM chain-family set-config registration.
func Registration() setconfig.Registration {
	return setconfig.Registration{
		Family:   chainselectors.FamilyEVM,
		Sequence: seqSetConfig,
		Verify:   verifyEVMChains,
	}
}

func verifyEVMChains(env cldf.Environment, chains []setconfig.ChainInput) error {
	for _, in := range chains {
		if _, ok := env.BlockChains.EVMChains()[in.ChainSelector]; !ok {
			return fmt.Errorf("EVM chain %d not found in environment", in.ChainSelector)
		}
		if err := validateMCMSIfPresent(env, in); err != nil {
			return err
		}
	}

	return nil
}
