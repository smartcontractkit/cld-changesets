package evmtransfertotimelock

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
)

// init auto-registers the EVM family when this package is imported.
// A blank import is sufficient to enable EVM chain support in [transfertotimelock.Changeset].
func init() {
	transfertotimelock.Register(Registration())
}

// Registration returns the EVM chain-family transfer-to-timelock registration.
// Importing this package registers EVM automatically via init; use
// Registration() only in tests that call [transfertotimelock.Register] manually
// without importing this package (for example to control registration order).
func Registration() transfertotimelock.Registration {
	return transfertotimelock.Registration{
		Family:   chainselectors.FamilyEVM,
		Sequence: seqTransferToTimelock,
		Verify:   verifyEVMChains,
	}
}

func verifyEVMChains(env cldf.Environment, chains []transfertotimelock.ChainInput) error {
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
