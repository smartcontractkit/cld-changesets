package evmtransfertomcms

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	transfertomcms "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-mcms"
)

// init auto-registers the EVM family when this package is imported.
// A blank import is sufficient to enable EVM chain support in [transfertomcms.Changeset].
func init() {
	transfertomcms.Register(Registration())
}

// Registration returns the EVM chain-family transfer-to-MCMS registration.
// Importing this package registers EVM automatically via init; use
// Registration() only in tests that call [transfertomcms.Register] manually
// without importing this package (for example to control registration order).
func Registration() transfertomcms.Registration {
	return transfertomcms.Registration{
		Family:   chainselectors.FamilyEVM,
		Sequence: seqTransferToMCMS,
		Verify:   verifyEVMChains,
	}
}

func verifyEVMChains(env cldf.Environment, chains []transfertomcms.ChainInput) error {
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
