package changesets

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	eth_types "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
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
		addrs, addrErr := rt.State().AddressBook.AddressesForChain(selector)
		require.NoError(t, addrErr)

		require.Len(t, addrs, 1)
		for _, addr := range addrs {
			require.Equal(t, linkcontracts.LinkToken, addr.Type)
			require.True(t, semvers.V1_0_0.Equal(&addr.Version))
		}
	}

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, len(selectors))
	for _, ref := range refs {
		require.Equal(t, datastore.ContractType(linkcontracts.LinkToken), ref.Type)
		require.True(t, semvers.V1_0_0.Equal(ref.Version))
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

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(DeployStaticLinkToken), []uint64{selector}),
	)
	require.NoError(t, err)

	addrs, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)

	require.Len(t, addrs, 1)
	for _, tv := range addrs {
		require.Equal(t, linkcontracts.StaticLinkToken, tv.Type)
		require.True(t, semvers.V1_0_0.Equal(&tv.Version))
	}

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, datastore.ContractType(linkcontracts.StaticLinkToken), refs[0].Type)
	require.True(t, semvers.V1_0_0.Equal(refs[0].Version))
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

func typeAndVersionWithLabels(tv cldf.TypeAndVersion, labels ...string) cldf.TypeAndVersion {
	for _, label := range labels {
		tv.Labels.Add(label)
	}

	return tv
}

func TestDeployLinkTokenZk(t *testing.T) {
	t.Skip("https://smartcontract-it.atlassian.net/browse/CCIP-6427")

	t.Parallel()

	selector := chain_selectors.TEST_90000050.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithZKSyncContainer(t, []uint64{selector}),
	))
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(DeployLinkToken), []uint64{selector}),
	)
	require.NoError(t, err)

	addrs, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)
	require.Len(t, addrs, 1)

	for _, tv := range addrs {
		require.Equal(t, datastore.ContractType(linkcontracts.LinkToken), tv.Type)
		require.True(t, semvers.V1_0_0.Equal(&tv.Version))
	}

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, datastore.ContractType(linkcontracts.LinkToken), refs[0].Type)
	require.True(t, semvers.V1_0_0.Equal(refs[0].Version))
}

// TestDeployLinkTokenContractEVMZkSyncEmulator covers the zkSync EVM-emulator path without a
// zkSync node: a simulated chain flagged IsZkSyncVM must still deploy through the standard
// client, have its tx confirmed and the address recorded. ClientZkSyncVM and
// DeployerKeyZkSyncVM stay nil, so any native zkSync deploy attempt would fail.
func TestDeployLinkTokenContractEVMZkSyncEmulator(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
	))
	require.NoError(t, err)

	chain := rt.Environment().BlockChains.EVMChains()[selector]
	chain.IsZkSyncVM = true
	require.Nil(t, chain.ClientZkSyncVM)
	require.Nil(t, chain.DeployerKeyZkSyncVM)

	var confirmed []common.Hash
	confirm := chain.Confirm
	chain.Confirm = func(tx *eth_types.Transaction) (uint64, error) {
		confirmed = append(confirmed, tx.Hash())
		return confirm(tx)
	}

	ab := cldf.NewMemoryAddressBook()
	deploy, err := deployLinkTokenContractEVM(logger.Test(t), chain, ab)
	require.NoError(t, err)
	require.NotNil(t, deploy.Tx)
	require.Equal(t, []common.Hash{deploy.Tx.Hash()}, confirmed)

	code, err := chain.Client.CodeAt(t.Context(), deploy.Address, nil)
	require.NoError(t, err)
	require.NotEmpty(t, code)

	addrs, err := ab.AddressesForChain(selector)
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	tv, ok := addrs[deploy.Address.String()]
	require.True(t, ok)
	require.Equal(t, linkcontracts.LinkToken, tv.Type)
	require.True(t, semvers.V1_0_0.Equal(&tv.Version))
}
