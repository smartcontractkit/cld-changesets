package stellardeploy

import (
	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
)

func Registration() deploy.Registration {
	return deploy.Registration{
		Family:   chainselectors.FamilyStellar,
		Sequence: seqDeployMCMSWithTimelock,
		Verify:   verifyStellarChains,
	}
}

func init() {
	deploy.Registry.Register(Registration())
}
