package stellartransfertotimelock

import (
	chainselectors "github.com/smartcontractkit/chain-selectors"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
)

func Registration() transfertotimelock.Registration {
	return transfertotimelock.Registration{
		Family:   chainselectors.FamilyStellar,
		Sequence: seqTransferToTimelock,
		Verify:   verifyStellarChains,
	}
}

func init() {
	transfertotimelock.Registry.Register(Registration())
}
