package evm

import (
	"context"
	"fmt"
	"testing"

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
