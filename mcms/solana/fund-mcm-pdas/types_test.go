package solfundmcmpdas

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
)

func TestFundingConfigRequiredFunding(t *testing.T) {
	cfg := FundingConfig{
		ProposeMCM:   100,
		CancellerMCM: 200,
		BypasserMCM:  300,
		Timelock:     400,
	}
	require.Equal(t, uint64(1000), cfg.RequiredFunding())
}

func TestEnvFromDeps(t *testing.T) {
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
