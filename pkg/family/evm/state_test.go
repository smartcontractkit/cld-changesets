package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/mcms/sdk"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
)

func TestGetAddressTypeVersionByQualifier(t *testing.T) {
	t.Parallel()

	chainSel := chainsel.ETHEREUM_TESTNET_SEPOLIA.Selector
	otherSel := chainSel + 1
	v := version1_0_0

	t.Run("no addresses for chain", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore().Seal()
		_, err := GetAddressTypeVersionByQualifier(ds.Addresses(), chainSel, "")
		require.ErrorContains(t, err, "no addresses found for chain")
	})

	t.Run("wrong chain selector", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000001",
			ChainSelector: otherSel,
			Type:          datastore.ContractType(linkcontracts.LinkToken),
			Version:       &v,
		}))
		_, err := GetAddressTypeVersionByQualifier(ds.Seal().Addresses(), chainSel, "")
		require.ErrorContains(t, err, "no addresses found for chain")
	})

	t.Run("success without qualifier", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000001",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(linkcontracts.LinkToken),
			Version:       &v,
		}))
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000002",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(mcmscontracts.CallProxy),
			Version:       &v,
		}))
		got, err := GetAddressTypeVersionByQualifier(ds.Seal().Addresses(), chainSel, "")
		require.NoError(t, err)
		require.Len(t, got, 2)
		require.Equal(t, linkcontracts.LinkToken, got["0x0000000000000000000000000000000000000001"].Type)
		require.Equal(t, mcmscontracts.CallProxy, got["0x0000000000000000000000000000000000000002"].Type)
	})

	t.Run("qualifier filters", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000001",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(linkcontracts.LinkToken),
			Version:       &v,
			Qualifier:     "a",
		}))
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000002",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(mcmscontracts.CallProxy),
			Version:       &v,
			Qualifier:     "b",
		}))
		got, err := GetAddressTypeVersionByQualifier(ds.Seal().Addresses(), chainSel, "a")
		require.NoError(t, err)
		require.Len(t, got, 1)
		_, ok := got["0x0000000000000000000000000000000000000001"]
		require.True(t, ok)
	})

	t.Run("qualifier excludes all on chain", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000001",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(linkcontracts.LinkToken),
			Version:       &v,
			Qualifier:     "a",
		}))
		_, err := GetAddressTypeVersionByQualifier(ds.Seal().Addresses(), chainSel, "z")
		require.ErrorContains(t, err, fmt.Sprintf("no addresses found for chain %d with qualifier %q", chainSel, "z"))
	})

	t.Run("labels copied to TypeAndVersion", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		labels := datastore.NewLabelSet(mcmscontracts.ProposerRole.String())
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000001",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(mcmscontracts.ManyChainMultisig),
			Version:       &v,
			Labels:        labels,
		}))
		got, err := GetAddressTypeVersionByQualifier(ds.Seal().Addresses(), chainSel, "")
		require.NoError(t, err)
		require.Len(t, got, 1)
		tv := got["0x0000000000000000000000000000000000000001"]
		require.True(t, tv.Labels.Contains(mcmscontracts.ProposerRole.String()))
	})

	t.Run("nil version only returns error without panic", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000001",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(linkcontracts.LinkToken),
		}))
		var err error
		require.NotPanics(t, func() {
			_, err = GetAddressTypeVersionByQualifier(ds.Seal().Addresses(), chainSel, "")
		})
		require.ErrorContains(t, err, "no address refs with a non-nil contract version")
	})

	t.Run("nil version skipped when another ref has version", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000001",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(linkcontracts.LinkToken),
		}))
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000002",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(mcmscontracts.CallProxy),
			Version:       &v,
		}))
		got, err := GetAddressTypeVersionByQualifier(ds.Seal().Addresses(), chainSel, "")
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Contains(t, got, "0x0000000000000000000000000000000000000002")
	})

	t.Run("invalid hex address in datastore", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "not-a-valid-hex-address",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(linkcontracts.LinkToken),
			Version:       &v,
		}))
		_, err := GetAddressTypeVersionByQualifier(ds.Seal().Addresses(), chainSel, "")
		require.ErrorContains(t, err, "not a valid hex-encoded EVM address")
	})

	t.Run("zero address in datastore", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000000",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(linkcontracts.LinkToken),
			Version:       &v,
		}))
		_, err := GetAddressTypeVersionByQualifier(ds.Seal().Addresses(), chainSel, "")
		require.ErrorContains(t, err, "must not be the zero address")
	})
}

func TestMaybeLoadMCMSWithTimelockChainState(t *testing.T) {
	t.Parallel()

	ch := cldf_evm.Chain{
		Selector: chainsel.ETHEREUM_TESTNET_SEPOLIA.Selector,
		Client:   nil,
	}

	t.Run("empty addresses returns nil bindings", func(t *testing.T) {
		t.Parallel()
		state, err := MaybeLoadMCMSWithTimelockChainState(ch, map[string]cldf.TypeAndVersion{})
		require.NoError(t, err)
		require.NotNil(t, state)
		require.Nil(t, state.Timelock)
		require.Nil(t, state.CallProxy)
		require.Nil(t, state.ProposerMcm)
		require.Nil(t, state.BypasserMcm)
		require.Nil(t, state.CancellerMcm)
	})

	t.Run("duplicate RBACTimelock in bundle", func(t *testing.T) {
		t.Parallel()
		tv := cldf.NewTypeAndVersion(mcmscontracts.RBACTimelock, version1_0_0)
		addrs := map[string]cldf.TypeAndVersion{
			"0x0000000000000000000000000000000000000001": tv,
			"0x0000000000000000000000000000000000000002": tv,
		}
		_, err := MaybeLoadMCMSWithTimelockChainState(ch, addrs)
		require.ErrorContains(t, err, "error: found more than one instance")
	})

	t.Run("invalid hex address in bundle", func(t *testing.T) {
		t.Parallel()
		tv := cldf.NewTypeAndVersion(mcmscontracts.RBACTimelock, version1_0_0)
		_, err := MaybeLoadMCMSWithTimelockChainState(ch, map[string]cldf.TypeAndVersion{
			"not-a-valid-hex-address": tv,
		})
		require.ErrorContains(t, err, "not a valid hex-encoded EVM address")
	})
}

func TestMaybeLoadMCMSWithTimelockStateWithQualifier(t *testing.T) {
	t.Parallel()

	chainSel := chainsel.ETHEREUM_TESTNET_SEPOLIA.Selector
	v := version1_0_0

	t.Run("chain not in environment", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore().Seal()
		env := testEVMEnv(t, ds, chain.NewBlockChains(nil))
		_, err := MaybeLoadMCMSWithTimelockStateWithQualifier(env, []uint64{chainSel}, "")
		require.ErrorContains(t, err, "not found")
	})

	t.Run("no addresses in datastore for chain", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore().Seal()
		evmCh := cldf_evm.Chain{Selector: chainSel, Client: nil}
		env := testEVMEnv(t, ds, chain.NewBlockChains(map[uint64]chain.BlockChain{
			chainSel: evmCh,
		}))
		_, err := MaybeLoadMCMSWithTimelockStateWithQualifier(env, []uint64{chainSel}, "")
		require.ErrorContains(t, err, "no addresses found for chain")
	})

	t.Run("chain present with non MCMS ref succeeds with empty bindings", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000001",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(linkcontracts.LinkToken),
			Version:       &v,
		}))
		evmCh := cldf_evm.Chain{Selector: chainSel, Client: nil}
		env := testEVMEnv(t, ds.Seal(), chain.NewBlockChains(map[uint64]chain.BlockChain{
			chainSel: evmCh,
		}))
		got, err := MaybeLoadMCMSWithTimelockStateWithQualifier(env, []uint64{chainSel}, "")
		require.NoError(t, err)
		require.Len(t, got, 1)
		st := got[chainSel]
		require.NotNil(t, st)
		require.Nil(t, st.Timelock)
	})

	t.Run("datastore refs missing version returns error", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000001",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(linkcontracts.LinkToken),
		}))
		evmCh := cldf_evm.Chain{Selector: chainSel, Client: nil}
		env := testEVMEnv(t, ds.Seal(), chain.NewBlockChains(map[uint64]chain.BlockChain{
			chainSel: evmCh,
		}))
		_, err := MaybeLoadMCMSWithTimelockStateWithQualifier(env, []uint64{chainSel}, "")
		require.ErrorContains(t, err, "no address refs with a non-nil contract version")
	})
}

func testEVMEnv(t *testing.T, ds datastore.DataStore, chains chain.BlockChains) cldf.Environment {
	t.Helper()
	return *cldf.NewEnvironment(
		"test",
		logger.Nop(),
		cldf.NewMemoryAddressBook(),
		ds,
		nil,
		nil,
		func() context.Context { return t.Context() },
		ocr.OCRSecrets{},
		chains,
	)
}

func TestMCMSWithTimelockState_GenerateMCMSWithTimelockViewV2(t *testing.T) {
	t.Parallel()

	selector := chainsel.TEST_90000001.Selector
	env, err := environment.New(t.Context(),
		environment.WithEVMSimulated(t, []uint64{selector}),
	)
	require.NoError(t, err)

	chain := env.BlockChains.EVMChains()[selector]

	proposerMcm := deployMCMEvm(t, chain, &mcmstypes.Config{Quorum: 1, Signers: []common.Address{
		common.HexToAddress("0x0000000000000000000000000000000000000001"),
	}})
	cancellerMcm := deployMCMEvm(t, chain, &mcmstypes.Config{Quorum: 1, Signers: []common.Address{
		common.HexToAddress("0x0000000000000000000000000000000000000002"),
	}})
	bypasserMcm := deployMCMEvm(t, chain, &mcmstypes.Config{Quorum: 1, Signers: []common.Address{
		common.HexToAddress("0x0000000000000000000000000000000000000003"),
	}})
	timelock := deployTimelockEvm(t, chain, big.NewInt(1),
		common.HexToAddress("0x0000000000000000000000000000000000000004"),
		[]common.Address{common.HexToAddress("0x0000000000000000000000000000000000000005")},
		[]common.Address{common.HexToAddress("0x0000000000000000000000000000000000000006")},
		[]common.Address{common.HexToAddress("0x0000000000000000000000000000000000000007")},
		[]common.Address{common.HexToAddress("0x0000000000000000000000000000000000000008")},
	)
	callProxy := deployCallProxyEvm(t, chain,
		common.HexToAddress("0x0000000000000000000000000000000000000009"))

	tests := []struct {
		name      string
		contracts *MCMSWithTimelockState
		want      string
		wantErr   string
	}{
		{
			name: "success",
			contracts: &MCMSWithTimelockState{
				ProposerMcm:  proposerMcm,
				CancellerMcm: cancellerMcm,
				BypasserMcm:  bypasserMcm,
				Timelock:     timelock,
				CallProxy:    callProxy,
			},
			want: fmt.Sprintf(`{
				"proposer": {
					"address": "%s",
					"owner":   "%s",
					"config":  {
						"quorum":       1,
						"signers":      ["0x0000000000000000000000000000000000000001"],
						"groupSigners": []
					}
				},
				"canceller": {
					"address": "%s",
					"owner":   "%s",
					"config":  {
						"quorum":       1,
						"signers":      ["0x0000000000000000000000000000000000000002"],
						"groupSigners": []
					}
				},
				"bypasser": {
					"address": "%s",
					"owner":   "%s",
					"config":  {
						"quorum":       1,
						"signers":      ["0x0000000000000000000000000000000000000003"],
						"groupSigners": []
					}
				},
				"timelock": {
					"address": "%s",
					"owner":   "0x0000000000000000000000000000000000000000",
					"membersByRole": {
						"ADMIN_ROLE":     [ "0x0000000000000000000000000000000000000004" ],
						"PROPOSER_ROLE":  [ "0x0000000000000000000000000000000000000005" ],
						"EXECUTOR_ROLE":  [ "0x0000000000000000000000000000000000000006" ],
						"CANCELLER_ROLE": [ "0x0000000000000000000000000000000000000007" ],
						"BYPASSER_ROLE":  [ "0x0000000000000000000000000000000000000008" ]
					}
				},
				"callProxy": {
					"address": "%s",
					"owner":   "0x0000000000000000000000000000000000000000"
				}
			}`, evmAddr(proposerMcm.Address()), evmAddr(chain.DeployerKey.From),
				evmAddr(cancellerMcm.Address()), evmAddr(chain.DeployerKey.From),
				evmAddr(bypasserMcm.Address()), evmAddr(chain.DeployerKey.From),
				evmAddr(timelock.Address()), evmAddr(callProxy.Address())),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := tt.contracts

			got, err := state.GenerateMCMSWithTimelockView()

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.JSONEq(t, tt.want, toJSON(t, &got))
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestAddressesForChain(t *testing.T) {
	t.Parallel()

	chainSelector := chainsel.ETHEREUM_MAINNET.Selector

	t.Run("environment with AddressBook only", func(t *testing.T) {
		t.Parallel()

		// Create environment with only AddressBook
		addressBook := cldf.NewMemoryAddressBook()
		err := addressBook.Save(chainSelector, "0x1234567890123456789012345678901234567890",
			cldf.NewTypeAndVersion(linkcontracts.LinkToken, cldchangesetscommon.Version1_0_0))
		require.NoError(t, err)

		env := cldf.Environment{
			ExistingAddresses: addressBook,
			DataStore:         nil, // No DataStore
		}

		// Test the merge function
		mergedAddresses, err := AddressesForChain(env, chainSelector, "")
		require.NoError(t, err)

		// Should have address from AddressBook only
		require.Len(t, mergedAddresses, 1)
		require.Contains(t, mergedAddresses, "0x1234567890123456789012345678901234567890")
	})

	t.Run("environment with DataStore only", func(t *testing.T) {
		t.Parallel()

		// Create environment with only DataStore
		dataStore := datastore.NewMemoryDataStore()
		err := dataStore.Addresses().Add(datastore.AddressRef{
			ChainSelector: chainSelector,
			Address:       "0xABCDEF1234567890123456789012345678901234",
			Type:          datastore.ContractType(mcmscontracts.RBACTimelock),
			Version:       &cldchangesetscommon.Version1_0_0,
		})
		require.NoError(t, err)

		addressBook := cldf.NewMemoryAddressBook()

		env := cldf.Environment{
			ExistingAddresses: addressBook,
			DataStore:         dataStore.Seal(),
		}

		// Test the merge function
		mergedAddresses, err := AddressesForChain(env, chainSelector, "")
		require.NoError(t, err)

		// Should have address from DataStore only
		require.Len(t, mergedAddresses, 1)
		require.Contains(t, mergedAddresses, "0xABCDEF1234567890123456789012345678901234")
	})
	t.Run("environment with AddressBook and DataStore without qualifier", func(t *testing.T) {
		t.Parallel()

		// Create a mock environment with both AddressBook and DataStore
		addressBook := cldf.NewMemoryAddressBook()
		err := addressBook.Save(chainSelector, "0x1234567890123456789012345678901234567890",
			cldf.NewTypeAndVersion(linkcontracts.LinkToken, cldchangesetscommon.Version1_0_0))
		require.NoError(t, err)

		dataStore := datastore.NewMemoryDataStore()
		err = dataStore.Addresses().Add(datastore.AddressRef{
			ChainSelector: chainSelector,
			Address:       "0xABCDEF1234567890123456789012345678901234",
			Type:          datastore.ContractType(mcmscontracts.RBACTimelock),
			Version:       &cldchangesetscommon.Version1_0_0,
			Labels: datastore.NewLabelSet(
				"team:core",
				"environment:production",
				"role:timelock",
			),
		})
		require.NoError(t, err)

		env := cldf.Environment{
			ExistingAddresses: addressBook,
			DataStore:         dataStore.Seal(),
		}

		// Test the merge function
		mergedAddresses, err := AddressesForChain(env, chainSelector, "")
		require.NoError(t, err)

		// Should have addresses from both sources
		require.Len(t, mergedAddresses, 2)
		require.Contains(t, mergedAddresses, "0x1234567890123456789012345678901234567890")
		require.Contains(t, mergedAddresses, "0xABCDEF1234567890123456789012345678901234")

		// Verify that types are correctly preserved
		linkTokenTV := mergedAddresses["0x1234567890123456789012345678901234567890"]
		require.Equal(t, linkcontracts.LinkToken, linkTokenTV.Type)
		require.Equal(t, cldchangesetscommon.Version1_0_0, linkTokenTV.Version)

		timelockTV := mergedAddresses["0xABCDEF1234567890123456789012345678901234"]
		require.Equal(t, mcmscontracts.RBACTimelock, timelockTV.Type)
		require.Equal(t, cldchangesetscommon.Version1_0_0, timelockTV.Version)

		// Verify labels are preserved in DataStore
		refs := env.DataStore.Addresses().Filter(datastore.AddressRefByChainSelector(chainSelector))
		require.Len(t, refs, 1)

		timelockRef := refs[0]
		require.Equal(t, "0xABCDEF1234567890123456789012345678901234", timelockRef.Address)
		require.True(t, timelockRef.Labels.Contains("team:core"))
		require.True(t, timelockRef.Labels.Contains("environment:production"))
		require.True(t, timelockRef.Labels.Contains("role:timelock"))
	})

	t.Run("environment with AddressBook and DataStore with qualifier", func(t *testing.T) {
		t.Parallel()

		dataStore := datastore.NewMemoryDataStore()

		// Add contracts with different qualifiers
		err := dataStore.Addresses().Add(datastore.AddressRef{
			ChainSelector: chainSelector,
			Address:       "0x1111111111111111111111111111111111111111",
			Type:          datastore.ContractType(mcmscontracts.RBACTimelock),
			Version:       &cldchangesetscommon.Version1_0_0,
			Qualifier:     "team-a",
			Labels: datastore.NewLabelSet(
				"team:team-a",
				"role:timelock",
			),
		})
		require.NoError(t, err)

		err = dataStore.Addresses().Add(datastore.AddressRef{
			ChainSelector: chainSelector,
			Address:       "0x2222222222222222222222222222222222222222",
			Type:          datastore.ContractType(mcmscontracts.RBACTimelock),
			Version:       &cldchangesetscommon.Version1_0_0,
			Qualifier:     "team-b",
			Labels: datastore.NewLabelSet(
				"team:team-b",
				"role:timelock",
			),
		})
		require.NoError(t, err)

		env := cldf.Environment{
			ExistingAddresses: cldf.NewMemoryAddressBook(),
			DataStore:         dataStore.Seal(),
		}

		// Test filtering by qualifier
		mergedAddresses, err := AddressesForChain(env, chainSelector, "team-a")
		require.NoError(t, err)

		// Should only have team-a contract
		require.Len(t, mergedAddresses, 1)
		require.Contains(t, mergedAddresses, "0x1111111111111111111111111111111111111111")
		require.NotContains(t, mergedAddresses, "0x2222222222222222222222222222222222222222")

		// Verify the correct contract type
		timelockTV := mergedAddresses["0x1111111111111111111111111111111111111111"]
		require.Equal(t, mcmscontracts.RBACTimelock, timelockTV.Type)

		// Verify labels are preserved for the filtered contract
		refs := env.DataStore.Addresses().Filter(
			datastore.AddressRefByChainSelector(chainSelector),
			datastore.AddressRefByQualifier("team-a"),
		)
		require.Len(t, refs, 1)

		teamARef := refs[0]
		require.Equal(t, "0x1111111111111111111111111111111111111111", teamARef.Address)
		require.Equal(t, "team-a", teamARef.Qualifier)
		require.True(t, teamARef.Labels.Contains("team:team-a"))
		require.True(t, teamARef.Labels.Contains("role:timelock"))
	})

	t.Run("environment with duplicated addresses in AddressBook and DataStore", func(t *testing.T) {
		t.Parallel()

		const (
			duplicateAddress = "0x1234567890123456789012345678901234567890"
			uniqueAddress    = "0xABCDEF1234567890123456789012345678901234"
		)

		// Create environment with same address in both AddressBook and DataStore
		addressBook := cldf.NewMemoryAddressBook()
		// Add LinkToken to AddressBook
		err := addressBook.Save(chainSelector, duplicateAddress,
			cldf.NewTypeAndVersion(linkcontracts.LinkToken, cldchangesetscommon.Version1_0_0))
		require.NoError(t, err)

		dataStore := datastore.NewMemoryDataStore()

		// Add the SAME address to DataStore but with different type/version and labels
		err = dataStore.Addresses().Add(datastore.AddressRef{
			ChainSelector: chainSelector,
			Address:       duplicateAddress,                                   // Same address as AddressBook
			Type:          datastore.ContractType(mcmscontracts.RBACTimelock), // Different type from AddressBook LinkToken
			Version:       &cldchangesetscommon.Version1_6_0,                  // Different version
			Labels: datastore.NewLabelSet(
				"team:datastore-team",
				"environment:staging",
				"override:true",
			),
		})
		require.NoError(t, err)

		// Also add a unique DataStore address
		err = dataStore.Addresses().Add(datastore.AddressRef{
			ChainSelector: chainSelector,
			Address:       uniqueAddress,
			Type:          datastore.ContractType(mcmscontracts.RBACTimelock),
			Version:       &cldchangesetscommon.Version1_0_0,
			Labels: datastore.NewLabelSet(
				"team:unique-entry",
				"role:timelock",
			),
		})
		require.NoError(t, err)

		env := cldf.Environment{
			ExistingAddresses: addressBook,
			DataStore:         dataStore.Seal(),
		}

		// Test the merge function
		mergedAddresses, err := AddressesForChain(env, chainSelector, "")
		require.NoError(t, err)

		// Should have 2 addresses total (duplicate should be merged, unique should be included)
		require.Len(t, mergedAddresses, 2)
		require.Contains(t, mergedAddresses, duplicateAddress)
		require.Contains(t, mergedAddresses, uniqueAddress)

		// The duplicate address should use DataStore values (DataStore takes precedence)
		duplicateTV := mergedAddresses[duplicateAddress]
		require.Equal(t, mcmscontracts.RBACTimelock, duplicateTV.Type, "DataStore type should override AddressBook type")
		require.Equal(t, cldchangesetscommon.Version1_6_0, duplicateTV.Version, "DataStore version should override AddressBook version")

		// The unique address should have correct type
		uniqueTV := mergedAddresses[uniqueAddress]
		require.Equal(t, mcmscontracts.RBACTimelock, uniqueTV.Type)
		require.Equal(t, cldchangesetscommon.Version1_0_0, uniqueTV.Version)

		// Verify that DataStore labels are preserved for both addresses
		refs := env.DataStore.Addresses().Filter(datastore.AddressRefByChainSelector(chainSelector))
		require.Len(t, refs, 2)

		// Find the refs by address
		var duplicateRef, uniqueRef *datastore.AddressRef
		for i := range refs {
			switch refs[i].Address {
			case duplicateAddress:
				duplicateRef = &refs[i]
			case uniqueAddress:
				uniqueRef = &refs[i]
			}
		}

		require.NotNil(t, duplicateRef, "Should find duplicate address in DataStore")
		require.NotNil(t, uniqueRef, "Should find unique address in DataStore")

		// Verify labels are preserved for the duplicate address (which should come from DataStore)
		require.True(t, duplicateRef.Labels.Contains("team:datastore-team"))
		require.True(t, duplicateRef.Labels.Contains("environment:staging"))
		require.True(t, duplicateRef.Labels.Contains("override:true"))

		// Verify labels for the unique address
		require.True(t, uniqueRef.Labels.Contains("team:unique-entry"))
		require.True(t, uniqueRef.Labels.Contains("role:timelock"))
	})
}

func TestGetMCMSWithTimelockState(t *testing.T) {
	t.Parallel()

	selector := chainsel.TEST_90000001.Selector
	env, err := environment.New(t.Context(),
		environment.WithEVMSimulated(t, []uint64{selector}),
	)
	require.NoError(t, err)

	chain := env.BlockChains.EVMChains()[selector]

	sharedMcm := deployMCMEvm(t, chain, &mcmstypes.Config{Quorum: 1, Signers: []common.Address{
		common.HexToAddress("0x0000000000000000000000000000000000000001"),
	}})
	sharedAddress := strings.ToLower(sharedMcm.Address().Hex())

	timelock := deployTimelockEvm(t, chain, big.NewInt(1),
		common.HexToAddress("0x0000000000000000000000000000000000000004"),
		[]common.Address{common.HexToAddress("0x0000000000000000000000000000000000000005")},
		[]common.Address{common.HexToAddress("0x0000000000000000000000000000000000000006")},
		[]common.Address{common.HexToAddress("0x0000000000000000000000000000000000000007")},
		[]common.Address{common.HexToAddress("0x0000000000000000000000000000000000000008")},
	)
	callProxy := deployCallProxyEvm(t, chain,
		common.HexToAddress("0x0000000000000000000000000000000000000009"))
	proposerMcm := deployMCMEvm(t, chain, &mcmstypes.Config{Quorum: 1, Signers: []common.Address{
		common.HexToAddress("0x0000000000000000000000000000000000000002"),
	}})

	// timelock, callProxy, proposer shared by both stores
	commonRefs := []datastore.AddressRef{
		{ChainSelector: selector, Address: strings.ToLower(timelock.Address().Hex()), Type: datastore.ContractType(mcmscontracts.RBACTimelock), Version: &cldchangesetscommon.Version1_0_0},
		{ChainSelector: selector, Address: strings.ToLower(callProxy.Address().Hex()), Type: datastore.ContractType(mcmscontracts.CallProxy), Version: &cldchangesetscommon.Version1_0_0},
		{ChainSelector: selector, Address: strings.ToLower(proposerMcm.Address().Hex()), Type: datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), Version: &cldchangesetscommon.Version1_0_0},
	}

	t.Run("shared address for bypasser and canceller", func(t *testing.T) {
		t.Parallel()

		// Store DS with bypasser/canceller sharing the same address
		store := datastore.NewMemoryDataStore()
		for _, ref := range commonRefs {
			require.NoError(t, store.Addresses().Add(ref))
		}
		require.NoError(t, store.Addresses().Add(datastore.AddressRef{
			ChainSelector: selector, Address: sharedAddress,
			Type: datastore.ContractType(mcmscontracts.BypasserManyChainMultisig), Version: &cldchangesetscommon.Version1_0_0, Qualifier: "bypasser",
		}))
		require.NoError(t, store.Addresses().Add(datastore.AddressRef{
			ChainSelector: selector, Address: sharedAddress,
			Type: datastore.ContractType(mcmscontracts.CancellerManyChainMultisig), Version: &cldchangesetscommon.Version1_0_0, Qualifier: "canceller",
		}))

		state, err := GetMCMSWithTimelockState(store.Seal().Addresses(), chain, "")
		require.NoError(t, err)

		require.NotNil(t, state.Timelock, "timelock should be loaded")
		require.NotNil(t, state.CallProxy, "call proxy should be loaded")
		require.NotNil(t, state.ProposerMcm, "proposer should be loaded")
		require.NotNil(t, state.BypasserMcm, "bypasser should be loaded despite shared address")
		require.NotNil(t, state.CancellerMcm, "canceller should be loaded despite shared address")

		require.Equal(t, sharedMcm.Address(), state.BypasserMcm.Address())
		require.Equal(t, sharedMcm.Address(), state.CancellerMcm.Address())
	})

	t.Run("legacy ManyChainMultisig type is ignored", func(t *testing.T) {
		t.Parallel()

		// Store with legacy ManyChainMultisig typed bypasser/canceller
		legacyStore := datastore.NewMemoryDataStore()
		for _, ref := range commonRefs {
			require.NoError(t, legacyStore.Addresses().Add(ref))
		}
		require.NoError(t, legacyStore.Addresses().Add(datastore.AddressRef{
			ChainSelector: selector, Address: sharedAddress,
			Type: datastore.ContractType(mcmscontracts.ManyChainMultisig), Version: &cldchangesetscommon.Version1_0_0, Qualifier: "bypasser",
			Labels: datastore.NewLabelSet(mcmscontracts.BypasserRole.String()),
		}))
		require.NoError(t, legacyStore.Addresses().Add(datastore.AddressRef{
			ChainSelector: selector, Address: sharedAddress,
			Type: datastore.ContractType(mcmscontracts.ManyChainMultisig), Version: &cldchangesetscommon.Version1_0_0, Qualifier: "canceller",
			Labels: datastore.NewLabelSet(mcmscontracts.CancellerRole.String()),
		}))

		state, err := GetMCMSWithTimelockState(legacyStore.Seal().Addresses(), chain, "")
		require.NoError(t, err)

		require.NotNil(t, state.Timelock, "timelock should still load")
		require.NotNil(t, state.CallProxy, "callProxy should still load")
		require.NotNil(t, state.ProposerMcm, "proposer should still load")
		require.Nil(t, state.BypasserMcm, "legacy ManyChainMultisig bypasser should not load")
		require.Nil(t, state.CancellerMcm, "legacy ManyChainMultisig canceller should not load")
	})
}

// ----- helpers -----

func toJSON[T any](t *testing.T, value T) string {
	t.Helper()

	bytes, err := json.Marshal(value)
	require.NoError(t, err)

	return string(bytes)
}

func deployMCMEvm(
	t *testing.T, chain cldf_evm.Chain, config *mcmstypes.Config,
) *bindings.ManyChainMultiSig {
	t.Helper()

	_, tx, contract, err := bindings.DeployManyChainMultiSig(chain.DeployerKey, chain.Client)
	require.NoError(t, err)
	_, err = chain.Confirm(tx)
	require.NoError(t, err)

	groupQuorums, groupParents, signerAddresses, signerGroups, err := sdk.ExtractSetConfigInputs(config)
	require.NoError(t, err)
	tx, err = contract.SetConfig(chain.DeployerKey, signerAddresses, signerGroups, groupQuorums, groupParents, false)
	require.NoError(t, err)
	_, err = chain.Confirm(tx)
	require.NoError(t, err)

	return contract
}

func deployTimelockEvm(
	t *testing.T, chain cldf_evm.Chain, minDelay *big.Int, admin common.Address,
	proposers, executors, cancellers, bypassers []common.Address,
) *bindings.RBACTimelock {
	t.Helper()
	_, tx, contract, err := bindings.DeployRBACTimelock(
		chain.DeployerKey, chain.Client, minDelay, admin, proposers, executors, cancellers, bypassers)
	require.NoError(t, err)
	_, err = chain.Confirm(tx)
	require.NoError(t, err)

	return contract
}

func deployCallProxyEvm(
	t *testing.T, chain cldf_evm.Chain, target common.Address,
) *bindings.CallProxy {
	t.Helper()
	_, tx, contract, err := bindings.DeployCallProxy(chain.DeployerKey, chain.Client, target)
	require.NoError(t, err)
	_, err = chain.Confirm(tx)
	require.NoError(t, err)

	return contract
}

func evmAddr(addr common.Address) string {
	return strings.ToLower(addr.Hex())
}
