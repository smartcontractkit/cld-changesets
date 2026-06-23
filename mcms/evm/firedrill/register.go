package evmfiredrill

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	firedrill "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill"
)

func init() {
	firedrill.Register(Registration())
}

// Registration returns the EVM chain-family fire-drill registration.
func Registration() firedrill.Registration {
	return firedrill.Registration{
		Family:   chainselectors.FamilyEVM,
		Sequence: seqFireDrill,
		Verify:   verifyEVMChains,
	}
}

func verifyEVMChains(env cldf.Environment, chains []firedrill.ChainInput) error {
	for _, in := range chains {
		if _, ok := env.BlockChains.EVMChains()[in.ChainSelector]; !ok {
			return fmt.Errorf("EVM chain %d not found in environment", in.ChainSelector)
		}
		if err := validateMCMS(env, in); err != nil {
			return err
		}
	}

	return nil
}
