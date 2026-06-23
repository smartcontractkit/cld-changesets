package soldeploy

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
)

// init auto-registers the Solana family when this package is imported.
func init() {
	deploy.Register(Registration())
}

// Registration returns the Solana chain-family deploy registration for MCMS with
// timelock.
func Registration() deploy.Registration {
	return deploy.Registration{
		Family:   chainselectors.FamilySolana,
		Sequence: seqDeployMCMSWithTimelock,
		Verify:   verifySolanaChains,
	}
}

func verifySolanaChains(env cldf.Environment, chains []deploy.ChainInput) error {
	for _, c := range chains {
		if _, ok := env.BlockChains.SolanaChains()[c.ChainSelector]; !ok {
			return fmt.Errorf("solana chain %d not found in environment", c.ChainSelector)
		}
	}

	return nil
}
