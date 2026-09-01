package stellardeploy

import (
	"fmt"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
)

func verifyStellarChains(env cldf.Environment, inputs []deploy.ChainInput) error {
	chains := env.BlockChains.StellarChains()

	for _, input := range inputs {
		if _, ok := chains[input.ChainSelector]; !ok {
			return fmt.Errorf("stellar chain %d not found in environment", input.ChainSelector)
		}

		if err := input.Config.Proposer.Validate(); err != nil {
			return fmt.Errorf("stellar chain %d: invalid proposer config: %w", input.ChainSelector, err)
		}
		if err := input.Config.Canceller.Validate(); err != nil {
			return fmt.Errorf("stellar chain %d: invalid canceller config: %w", input.ChainSelector, err)
		}
		if err := input.Config.Bypasser.Validate(); err != nil {
			return fmt.Errorf("stellar chain %d: invalid bypasser config: %w", input.ChainSelector, err)
		}

		if input.Config.TimelockMinDelay != nil && !input.Config.TimelockMinDelay.IsUint64() {
			return fmt.Errorf("stellar chain %d: timelock minimum delay %s does not fit uint64", input.ChainSelector, input.Config.TimelockMinDelay)
		}
	}

	return nil
}
