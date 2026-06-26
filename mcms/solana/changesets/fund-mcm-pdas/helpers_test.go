package fundmcmpdas

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

func TestResolveFundingTargets(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	mcmProgram := solanago.NewWallet().PublicKey()
	timelockProgram := solanago.NewWallet().PublicKey()
	proposerSeed := testPDASeed(1)
	cancellerSeed := testPDASeed(2)
	bypasserSeed := testPDASeed(3)
	timelockSeed := testPDASeed(4)

	ds := datastore.NewMemoryDataStore()
	version := semver.MustParse("1.0.0")
	for _, ref := range []struct {
		typ     cldf.ContractType
		address string
	}{
		{mcmscontracts.RBACTimelock, mcmssolana.ContractAddress(timelockProgram, timelockSeed)},
		{mcmscontracts.ProposerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, proposerSeed)},
		{mcmscontracts.CancellerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, cancellerSeed)},
		{mcmscontracts.BypasserManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, bypasserSeed)},
	} {
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       ref.address,
			ChainSelector: selector,
			Type:          datastore.ContractType(ref.typ),
			Version:       version,
		}))
	}

	env := helperTestEnv(ds.Seal(), selector)
	cfg := FundingConfig{
		ProposeMCM:   10,
		CancellerMCM: 20,
		BypasserMCM:  30,
		Timelock:     40,
	}

	targets, err := ResolveFundingTargets(env, selector, cfg)
	require.NoError(t, err)
	require.Len(t, targets, 4)
	require.Equal(t, uint64(40), targets[0].Amount)
	require.Equal(t, uint64(10), targets[1].Amount)
	require.Equal(t, uint64(20), targets[2].Amount)
	require.Equal(t, uint64(30), targets[3].Amount)

	timelockSigner, err := mcmssolana.FindTimelockSignerPDA(timelockProgram, timelockSeed)
	require.NoError(t, err)
	proposerSigner, err := mcmssolana.FindSignerPDA(mcmProgram, proposerSeed)
	require.NoError(t, err)
	cancellerSigner, err := mcmssolana.FindSignerPDA(mcmProgram, cancellerSeed)
	require.NoError(t, err)
	bypasserSigner, err := mcmssolana.FindSignerPDA(mcmProgram, bypasserSeed)
	require.NoError(t, err)

	require.Equal(t, timelockSigner, targets[0].Address)
	require.Equal(t, proposerSigner, targets[1].Address)
	require.Equal(t, cancellerSigner, targets[2].Address)
	require.Equal(t, bypasserSigner, targets[3].Address)
}

func TestResolveFundingTargets_errors(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector

	env := helperTestEnv(datastore.NewMemoryDataStore().Seal(), selector)
	_, err := ResolveFundingTargets(env, selector, FundingConfig{})
	require.ErrorContains(t, err, "resolve timelock ref")

	ds := datastore.NewMemoryDataStore()
	version := semver.MustParse("1.0.0")
	mcmProgram := solanago.NewWallet().PublicKey()
	for _, ref := range []struct {
		typ     cldf.ContractType
		address string
	}{
		{mcmscontracts.RBACTimelock, "timelock"},
		{mcmscontracts.ProposerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(1))},
		{mcmscontracts.CancellerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(2))},
		{mcmscontracts.BypasserManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(3))},
	} {
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       ref.address,
			ChainSelector: selector,
			Type:          datastore.ContractType(ref.typ),
			Version:       version,
		}))
	}
	env = helperTestEnv(ds.Seal(), selector)
	_, err = ResolveFundingTargets(env, selector, FundingConfig{})
	require.ErrorContains(t, err, "parse timelock signer PDA")
}

func TestSignerPDAFromRef_errors(t *testing.T) {
	t.Parallel()

	_, err := mcmsSignerPDAFromRef("not-valid")
	require.Error(t, err)

	_, err = timelockSignerPDAFromRef("not-valid")
	require.Error(t, err)
}

func helperTestEnv(ds datastore.DataStore, selector uint64) cldf.Environment {
	return cldf.Environment{
		Logger:     logger.Nop(),
		DataStore:  ds,
		GetContext: context.Background,
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
	}
}

func testPDASeed(v byte) mcmssolana.PDASeed {
	var seed mcmssolana.PDASeed
	for i := range seed {
		seed[i] = v
	}

	return seed
}
