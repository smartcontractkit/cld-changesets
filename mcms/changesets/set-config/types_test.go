package setconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
)

func TestEnvFromDeps(t *testing.T) {
	t.Parallel()

	ds := cldfdatastore.NewMemoryDataStore().Seal()
	blockChains := chain.NewBlockChains(nil)
	deps := Deps{
		BlockChains: blockChains,
		DataStore:   ds,
	}

	env := EnvFromDeps(deps)
	require.Equal(t, blockChains, env.BlockChains)
	require.Equal(t, ds, env.DataStore)
}
