package changesets

import (
	"testing"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/utils/tests"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"

	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
	evmstate "github.com/smartcontractkit/cld-changesets/pkg/family/evm"
)

func TestDeployLinkToken(t *testing.T) {
	t.Parallel()

	selectors := []uint64{
		chain_selectors.TEST_90000001.Selector,
		chain_selectors.TEST_90000002.Selector,
	}
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, selectors),
	))
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(DeployLinkToken), selectors),
	)
	require.NoError(t, err)

	for _, selector := range selectors {
		chain := rt.Environment().BlockChains.EVMChains()[selector]
		addrs, addrsErr := rt.State().AddressBook.AddressesForChain(selector)
		require.NoError(t, addrsErr)

		state, stateErr := evmstate.MaybeLoadLinkTokenChainState(chain, addrs)
		require.NoError(t, stateErr)

		_, viewErr := state.GenerateLinkView()
		require.NoError(t, viewErr)
	}

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, len(selectors))
	for _, ref := range refs {
		require.Equal(t, datastore.ContractType(linkcontracts.LinkToken), ref.Type)
		require.True(t, cldchangesetscommon.Version1_0_0.Equal(ref.Version))
	}
}

func TestDeployLinkTokenRejectsInvalidSelectorsBeforeDeploy(t *testing.T) {
	t.Parallel()

	evmSelector := chain_selectors.TEST_90000001.Selector
	solSelector := chain_selectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := cldf.Environment{
		BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
			cldf_evm.Chain{Selector: evmSelector},
			cldf_solana.Chain{Selector: solSelector},
		}),
	}

	_, err := DeployLinkToken(env, []uint64{evmSelector, evmSelector})
	require.ErrorContains(t, err, "duplicate chain selector found")

	_, err = DeployLinkToken(env, []uint64{evmSelector, solSelector})
	require.ErrorContains(t, err, "is not in the evm family")

	_, err = DeployStaticLinkToken(env, []uint64{evmSelector, evmSelector})
	require.ErrorContains(t, err, "duplicate chain selector found")

	_, err = DeployStaticLinkToken(env, []uint64{evmSelector, solSelector})
	require.ErrorContains(t, err, "is not in the evm family")
}

func TestDeployLinkTokenRejectsExistingStateBeforeDeploy(t *testing.T) {
	t.Parallel()

	evmSelector := chain_selectors.TEST_90000001.Selector
	solSelector := chain_selectors.TEST_22222222222222222222222222222222222222222222.Selector
	const (
		evmAddress = "0xeC91988D7dD84d8adE801b739172ad15c860A700"
		solAddress = "J6oVJ42pE6eXdTCcCidhjzHWS7Sxz6yMsXHxXphT1U7Y"
	)

	tests := []struct {
		name    string
		env     cldf.Environment
		run     func(cldf.Environment) (cldf.ChangesetOutput, error)
		wantErr string
	}{
		{
			name: "link token exists in address book with labels",
			env: cldf.Environment{
				BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
					cldf_evm.Chain{Selector: evmSelector},
				}),
				ExistingAddresses: addressBookWith(t, evmSelector, evmAddress, typeAndVersionWithLabels(linkTokenTypeAndVersion(), "migrated")),
			},
			run: func(env cldf.Environment) (cldf.ChangesetOutput, error) {
				return DeployLinkToken(env, []uint64{evmSelector})
			},
			wantErr: "LinkToken contract already exists",
		},
		{
			name: "link token exists in address book without labels",
			env: cldf.Environment{
				BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
					cldf_evm.Chain{Selector: evmSelector},
				}),
				ExistingAddresses: addressBookWith(t, evmSelector, evmAddress, linkTokenTypeAndVersion()),
			},
			run: func(env cldf.Environment) (cldf.ChangesetOutput, error) {
				return DeployLinkToken(env, []uint64{evmSelector})
			},
			wantErr: "LinkToken contract already exists",
		},
		{
			name: "link token exists in datastore with non-empty qualifier",
			env: cldf.Environment{
				BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
					cldf_evm.Chain{Selector: evmSelector},
				}),
				DataStore: datastoreWith(t, evmSelector, evmAddress, linkTokenTypeAndVersion(), "migrated"),
			},
			run: func(env cldf.Environment) (cldf.ChangesetOutput, error) {
				return DeployLinkToken(env, []uint64{evmSelector})
			},
			wantErr: "LinkToken contract already exists",
		},
		{
			name: "link token exists in datastore with nil version",
			env: cldf.Environment{
				BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
					cldf_evm.Chain{Selector: evmSelector},
				}),
				DataStore: datastoreWithNilVersion(t, evmSelector, evmAddress, linkcontracts.LinkToken, "migrated"),
			},
			run: func(env cldf.Environment) (cldf.ChangesetOutput, error) {
				return DeployLinkToken(env, []uint64{evmSelector})
			},
			wantErr: "LinkToken contract already exists",
		},
		{
			name: "static link token exists in datastore",
			env: cldf.Environment{
				BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
					cldf_evm.Chain{Selector: evmSelector},
				}),
				DataStore: datastoreWith(t, evmSelector, evmAddress, staticLinkTokenTypeAndVersion(), ""),
			},
			run: func(env cldf.Environment) (cldf.ChangesetOutput, error) {
				return DeployStaticLinkToken(env, []uint64{evmSelector})
			},
			wantErr: "StaticLinkToken contract already exists",
		},
		{
			name: "solana link token exists in datastore",
			env: cldf.Environment{
				BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
					cldf_solana.Chain{Selector: solSelector},
				}),
				DataStore: datastoreWith(t, solSelector, solAddress, linkTokenTypeAndVersion(), ""),
			},
			run: func(env cldf.Environment) (cldf.ChangesetOutput, error) {
				return DeploySolanaLinkToken(env, DeploySolanaLinkTokenConfig{ChainSelector: solSelector})
			},
			wantErr: "LinkToken contract already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.run(tt.env)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestDeployStaticLinkToken(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
	))
	require.NoError(t, err)

	chain := rt.Environment().BlockChains.EVMChains()[selector]

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(DeployStaticLinkToken), []uint64{selector}),
	)
	require.NoError(t, err)

	addrs, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)

	state, err := evmstate.MaybeLoadStaticLinkTokenState(chain, addrs)
	require.NoError(t, err)

	_, err = state.GenerateStaticLinkView()
	require.NoError(t, err)

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, datastore.ContractType(linkcontracts.StaticLinkToken), refs[0].Type)
	require.True(t, cldchangesetscommon.Version1_0_0.Equal(refs[0].Version))
}

func addressBookWith(t *testing.T, selector uint64, address string, tv cldf.TypeAndVersion) cldf.AddressBook {
	t.Helper()

	ab := cldf.NewMemoryAddressBook()
	require.NoError(t, ab.Save(selector, address, tv))

	return ab
}

func datastoreWith(t *testing.T, selector uint64, address string, tv cldf.TypeAndVersion, qualifier string) datastore.DataStore {
	t.Helper()

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, saveAddressRef(ds, selector, address, tv, qualifier))

	return ds.Seal()
}

func datastoreWithNilVersion(t *testing.T, selector uint64, address string, contractType cldf.ContractType, qualifier string) datastore.DataStore {
	t.Helper()

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		ChainSelector: selector,
		Address:       address,
		Type:          datastore.ContractType(contractType.String()),
		Qualifier:     qualifier,
	}))

	return ds.Seal()
}

func typeAndVersionWithLabels(tv cldf.TypeAndVersion, labels ...string) cldf.TypeAndVersion {
	for _, label := range labels {
		tv.Labels.Add(label)
	}

	return tv
}

func TestDeployLinkTokenZk(t *testing.T) {
	tests.SkipFlakey(t, "https://smartcontract-it.atlassian.net/browse/CCIP-6427")

	t.Parallel()

	selector := chain_selectors.TEST_90000050.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithZKSyncContainer(t, []uint64{selector}),
	))
	require.NoError(t, err)

	chain := rt.Environment().BlockChains.EVMChains()[selector]

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(DeployLinkToken), []uint64{selector}),
	)
	require.NoError(t, err)

	addrs, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)

	state, err := evmstate.MaybeLoadLinkTokenChainState(chain, addrs)
	require.NoError(t, err)

	_, err = state.GenerateLinkView()
	require.NoError(t, err)
}
