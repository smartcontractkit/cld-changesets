package stellarsetconfig

import (
	chainselectors "github.com/smartcontractkit/chain-selectors"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
)

func Registration() setconfig.Registration {
	return setconfig.Registration{
		Family:   chainselectors.FamilyStellar,
		Sequence: seqSetConfig,
		Verify:   verifyStellarChains,
	}
}

func init() {
	setconfig.Registry.Register(Registration())
}
