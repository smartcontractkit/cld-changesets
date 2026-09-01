package stellartransfertotimelock

import (
	"fmt"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
	stellarinternal "github.com/smartcontractkit/cld-changesets/mcms/stellar/internal"
)

func verifyStellarChains(env cldf.Environment, inputs []transfertotimelock.ChainInput) error {
	chains := env.BlockChains.StellarChains()
	for _, input := range inputs {
		if _, ok := chains[input.ChainSelector]; !ok {
			return fmt.Errorf("stellar chain %d not found in environment", input.ChainSelector)
		}
		if input.MCMS != nil {
			if err := stellarinternal.ValidateMCMSRefs(env, input.ChainSelector, *input.MCMS); err != nil {
				return err
			}
		} else {
			continue
		}
	}

	return nil
}
