package changesets

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/require"

	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"

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

	err = rt.Exec(runtime.ChangesetTask(DeployLinkTokenChangeset{}, DeployLinkTokenInput{
		EVM: map[uint64]EVMLinkConfig{
			selectors[0]: {},
			selectors[1]: {},
		},
	}))
	require.NoError(t, err)

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, len(selectors))
	for _, ref := range refs {
		require.Equal(t, datastore.ContractType(linkcontracts.LinkToken), ref.Type)
		require.True(t, semvers.V1_0_0.Equal(ref.Version))
	}
}

func TestDeployStaticLinkToken(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
	))
	require.NoError(t, err)

	err = rt.Exec(runtime.ChangesetTask(DeployLinkTokenChangeset{}, DeployLinkTokenInput{
		EVM: map[uint64]EVMLinkConfig{
			selector: {Variant: EVMLinkStatic},
		},
	}))
	require.NoError(t, err)

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, datastore.ContractType(linkcontracts.StaticLinkToken), refs[0].Type)
	require.True(t, semvers.V1_0_0.Equal(refs[0].Version))
}

func TestDeployLinkTokenZk(t *testing.T) {
	t.Skip("https://smartcontract-it.atlassian.net/browse/CCIP-6427")
	t.Parallel()

	selector := chain_selectors.TEST_90000050.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithZKSyncContainer(t, []uint64{selector}),
	))
	require.NoError(t, err)

	err = rt.Exec(runtime.ChangesetTask(DeployLinkTokenChangeset{}, DeployLinkTokenInput{
		EVM: map[uint64]EVMLinkConfig{selector: {}},
	}))
	require.NoError(t, err)

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, datastore.ContractType(linkcontracts.LinkToken), refs[0].Type)
	require.True(t, semvers.V1_0_0.Equal(refs[0].Version))
}

func TestDeploySolanaLinkToken(t *testing.T) {
	t.Skip("requires Solana validator container")
	t.Parallel()

	solSelector := chain_selectors.TEST_22222222222222222222222222222222222222222222.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{solSelector}, "", nil),
	))
	require.NoError(t, err)

	mintKey, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	err = rt.Exec(runtime.ChangesetTask(DeployLinkTokenChangeset{}, DeployLinkTokenInput{
		Solana: map[uint64]SolanaLinkConfig{
			solSelector: {TokenPrivKey: mintKey, TokenDecimals: 9},
		},
	}))
	require.NoError(t, err)

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 1)
	require.Equal(t, datastore.ContractType(linkcontracts.LinkToken), refs[0].Type)
	require.True(t, semvers.V1_0_0.Equal(refs[0].Version))
	require.Equal(t, mintKey.PublicKey().String(), refs[0].Address)
}

func TestDeployLinkTokenRejectsWrongFamilyBeforeDeploy(t *testing.T) {
	t.Parallel()

	evmSelector := chain_selectors.TEST_90000001.Selector
	solSelector := chain_selectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := cldf.Environment{
		BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
			cldf_evm.Chain{Selector: evmSelector},
			cldf_solana.Chain{Selector: solSelector},
		}),
	}

	err := DeployLinkTokenChangeset{}.VerifyPreconditions(env, DeployLinkTokenInput{
		EVM: map[uint64]EVMLinkConfig{solSelector: {}},
	})
	require.ErrorContains(t, err, "is not in the evm family")

	err = DeployLinkTokenChangeset{}.VerifyPreconditions(env, DeployLinkTokenInput{
		Solana: map[uint64]SolanaLinkConfig{evmSelector: {}},
	})
	require.ErrorContains(t, err, "is not in the solana family")
}

func TestDeployLinkTokenRejectsExistingStateBeforeDeploy(t *testing.T) {
	t.Parallel()

	evmSelector := chain_selectors.TEST_90000001.Selector
	solSelector := chain_selectors.TEST_22222222222222222222222222222222222222222222.Selector
	const (
		evmAddress = "0xeC91988D7dD84d8adE801b739172ad15c860A700"
		solAddress = "J6oVJ42pE6eXdTCcCidhjzHWS7Sxz6yMsXHxXphT1U7Y"
	)

	tcs := []struct {
		name    string
		env     cldf.Environment
		input   DeployLinkTokenInput
		wantErr string
	}{
		{
			name: "burn/mint link token already exists",
			env: cldf.Environment{
				BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
					cldf_evm.Chain{Selector: evmSelector},
				}),
				DataStore: datastoreWith(t, evmSelector, evmAddress, linkTokenTypeAndVersion(), ""),
			},
			input:   DeployLinkTokenInput{EVM: map[uint64]EVMLinkConfig{evmSelector: {}}},
			wantErr: "LinkToken contract already exists",
		},
		{
			name: "static link token already exists",
			env: cldf.Environment{
				BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
					cldf_evm.Chain{Selector: evmSelector},
				}),
				DataStore: datastoreWith(t, evmSelector, evmAddress, staticLinkTokenTypeAndVersion(), ""),
			},
			input:   DeployLinkTokenInput{EVM: map[uint64]EVMLinkConfig{evmSelector: {Variant: EVMLinkStatic}}},
			wantErr: "StaticLinkToken contract already exists",
		},
		{
			name: "solana link token already exists",
			env: cldf.Environment{
				BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
					cldf_solana.Chain{Selector: solSelector},
				}),
				DataStore: datastoreWith(t, solSelector, solAddress, linkTokenTypeAndVersion(), ""),
			},
			input: func() DeployLinkTokenInput {
				key, err := solana.NewRandomPrivateKey()
				require.NoError(t, err)

				return DeployLinkTokenInput{Solana: map[uint64]SolanaLinkConfig{solSelector: {TokenPrivKey: key}}}
			}(),
			wantErr: "LinkToken contract already exists",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := DeployLinkTokenChangeset{}.VerifyPreconditions(tc.env, tc.input)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestDeployLinkTokenEmptyInputRejected(t *testing.T) {
	t.Parallel()

	err := DeployLinkTokenChangeset{}.VerifyPreconditions(cldf.Environment{}, DeployLinkTokenInput{})
	require.ErrorContains(t, err, "no chains specified")
}

func TestDeployLinkTokenRejectsMissingMintKey(t *testing.T) {
	t.Parallel()

	solSelector := chain_selectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := cldf.Environment{
		BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
			cldf_solana.Chain{Selector: solSelector},
		}),
	}

	err := DeployLinkTokenChangeset{}.VerifyPreconditions(env, DeployLinkTokenInput{
		Solana: map[uint64]SolanaLinkConfig{solSelector: {}},
	})
	require.ErrorContains(t, err, "TokenPrivKey must be set")
}

func TestDeployLinkTokenRejectsUnknownVariant(t *testing.T) {
	t.Parallel()

	evmSelector := chain_selectors.TEST_90000001.Selector
	env := cldf.Environment{
		BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
			cldf_evm.Chain{Selector: evmSelector},
		}),
	}

	err := DeployLinkTokenChangeset{}.VerifyPreconditions(env, DeployLinkTokenInput{
		EVM: map[uint64]EVMLinkConfig{evmSelector: {Variant: "invalid"}},
	})
	require.ErrorContains(t, err, "unknown EVM LINK variant")
}

func TestDeployLinkTokenBurnMintAndStaticSameInput(t *testing.T) {
	t.Parallel()

	selectors := []uint64{
		chain_selectors.TEST_90000001.Selector,
		chain_selectors.TEST_90000002.Selector,
	}
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, selectors),
	))
	require.NoError(t, err)

	err = rt.Exec(runtime.ChangesetTask(DeployLinkTokenChangeset{}, DeployLinkTokenInput{
		EVM: map[uint64]EVMLinkConfig{
			selectors[0]: {},
			selectors[1]: {Variant: EVMLinkStatic},
		},
	}))
	require.NoError(t, err)

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 2)

	byChain := make(map[uint64]datastore.ContractType, 2)
	for _, ref := range refs {
		byChain[ref.ChainSelector] = ref.Type
	}

	require.Equal(t, datastore.ContractType(linkcontracts.LinkToken), byChain[selectors[0]])
	require.Equal(t, datastore.ContractType(linkcontracts.StaticLinkToken), byChain[selectors[1]])
}

func TestDeployLinkTokenDifferentQualifierDoesNotBlock(t *testing.T) {
	t.Parallel()

	evmSelector := chain_selectors.TEST_90000001.Selector
	const evmAddress = "0xeC91988D7dD84d8adE801b739172ad15c860A700"

	// A LINK token with qualifier "migrated" already exists.
	// Deploying with qualifier "" (primary) should not be blocked.
	env := cldf.Environment{
		BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
			cldf_evm.Chain{Selector: evmSelector},
		}),
		DataStore: datastoreWith(t, evmSelector, evmAddress, linkTokenTypeAndVersion(), "migrated"),
	}

	err := DeployLinkTokenChangeset{}.VerifyPreconditions(env, DeployLinkTokenInput{
		EVM: map[uint64]EVMLinkConfig{evmSelector: {}},
	})
	require.NoError(t, err)
}

func datastoreWith(t *testing.T, selector uint64, address string, tv cldf.TypeAndVersion, qualifier string) datastore.DataStore {
	t.Helper()

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, saveAddressRef(ds, selector, address, tv, qualifier))

	return ds.Seal()
}
