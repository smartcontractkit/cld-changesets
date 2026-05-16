package cldfutil

import (
	"fmt"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

func ValidateSelectorsInEnvironment(e cldf.Environment, chains []uint64) error {
	for _, chain := range chains {
		if !e.BlockChains.Exists(chain) {
			return fmt.Errorf("chain %d not found in environment", chain)
		}
	}

	return nil
}
