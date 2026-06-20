package evmdeploytopology

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	deploycustomtopology "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy-custom-topology"
)

// init auto-registers the EVM family when this package is imported.
func init() {
	deploycustomtopology.Register(Registration())
}

// Registration returns the EVM chain-family deploy-custom-topology registration.
func Registration() deploycustomtopology.Registration {
	return deploycustomtopology.Registration{
		Family:   chainselectors.FamilyEVM,
		Sequence: seqDeployTopology,
		Verify:   verifyEVMChains,
	}
}

func verifyEVMChains(env cldf.Environment, chains []deploycustomtopology.ChainInput) error {
	for _, c := range chains {
		if _, ok := env.BlockChains.EVMChains()[c.ChainSelector]; !ok {
			return fmt.Errorf("EVM chain %d not found in environment", c.ChainSelector)
		}
	}

	return nil
}
