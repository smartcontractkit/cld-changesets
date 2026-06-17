package solreaders

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

func init() {
	if err := cldf.GetMCMSReaderRegistry().Register(chainselectors.FamilySolana, Reader{}); err != nil {
		panic(fmt.Sprintf("register MCMS reader for %q: %v", chainselectors.FamilySolana, err))
	}
}
