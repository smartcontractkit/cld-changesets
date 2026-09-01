package stellartransfertotimelock

import (
	"fmt"

	mcmstypes "github.com/smartcontractkit/mcms/types"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
	stellarinternal "github.com/smartcontractkit/cld-changesets/mcms/stellar/internal"
)

func verifyStellarChains(env cldf.Environment, inputs []transfertotimelock.ChainInput) error {
	chains := env.BlockChains.StellarChains()
	for _, input := range inputs {
		if _, ok := chains[input.ChainSelector]; !ok {
			return fmt.Errorf("stellar chain %d not found in environment", input.ChainSelector)
		}
		if input.MCMS == nil {
			continue
		}
		if err := stellarinternal.ValidateMCMSRefs(env, input.ChainSelector, *input.MCMS); err != nil {
			return err
		}
		if err := validateBypassTargets(input); err != nil {
			return err
		}
	}

	return nil
}

// validateBypassTargets rejects bypass-action transfers that include the
// Bypasser MCM itself. Executing that acceptance via bypasser_execute_batch
// re-enters the Bypasser MCM while it is executing, which Soroban forbids.
// Bypass stays allowed for other ownable targets (e.g. the CRE forwarder).
func validateBypassTargets(input transfertotimelock.ChainInput) error {
	if input.MCMS.TimelockAction != mcmstypes.TimelockActionBypass {
		return nil
	}

	bypasserType := cldfdatastore.ContractType(mcmscontracts.BypasserManyChainMultisig)
	for _, contract := range input.Contracts {
		if contract.Type == bypasserType {
			return fmt.Errorf(
				"stellar chain %d: cannot transfer the Bypasser MCM via a bypass proposal: "+
					"executing its accept_ownership through bypasser_execute_batch re-enters the "+
					"executing Soroban contract; use the schedule action instead",
				input.ChainSelector,
			)
		}
	}

	return nil
}
