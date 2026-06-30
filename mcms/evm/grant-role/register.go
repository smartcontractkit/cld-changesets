package evmgrantrole

import (
	chainselectors "github.com/smartcontractkit/chain-selectors"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
)

func init() {
	grantrole.Registry.Register(Registration())
}

func Registration() grantrole.Registration {
	return grantrole.Registration{
		Family:   chainselectors.FamilyEVM,
		Sequence: seqGrantRole,
		Verify:   validateEVMChains,
	}
}
