package evmreaders

import (
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

func init() {
	if err := cldf.GetMCMSReaderRegistry().Register(chainselectors.FamilyEVM, Reader{}); err != nil {
		panic(fmt.Sprintf("register MCMS reader for %q: %v", chainselectors.FamilyEVM, err))
	}
}
