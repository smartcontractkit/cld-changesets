package stellarreaders

import (
	chainselectors "github.com/smartcontractkit/chain-selectors"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

func init() {
	if err := cldf.GetMCMSReaderRegistry().Register(
		chainselectors.FamilyStellar,
		Reader{},
	); err != nil {
		panic(err)
	}
}
