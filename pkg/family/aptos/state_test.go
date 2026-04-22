package aptos

import (
	"context"
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	aptoschain "github.com/aptos-labs/aptos-go-sdk"
	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
)

const testAptosMCMSAddr = "0x3"

var version1_6_0 = *semver.MustParse("1.6.0")

func TestLoadMCMSAddresses(t *testing.T) {
	t.Parallel()

	chainSel := chainsel.APTOS_TESTNET.Selector

	t.Run("empty selectors", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		ds := datastore.NewMemoryDataStore().Seal()
		env := testEnv(t, ab, ds)

		got, err := LoadMCMSAddresses(env, nil, version1_6_0)

		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		ds := datastore.NewMemoryDataStore()
		v := version1_6_0
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       testAptosMCMSAddr,
			ChainSelector: chainSel,
			Type:          datastore.ContractType(AptosMCMSType),
			Version:       &v,
		}))
		env := testEnv(t, ab, ds.Seal())

		got, err := LoadMCMSAddresses(env, []uint64{chainSel}, version1_6_0)

		require.NoError(t, err)
		require.Len(t, got, 1)
		var want aptoschain.AccountAddress
		require.NoError(t, want.ParseStringRelaxed(testAptosMCMSAddr))
		require.Equal(t, want, got[chainSel])
	})

	t.Run("success with explicit contract version", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		ds := datastore.NewMemoryDataStore()
		v := *semver.MustParse("1.5.0")
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       testAptosMCMSAddr,
			ChainSelector: chainSel,
			Type:          datastore.ContractType(AptosMCMSType),
			Version:       &v,
		}))
		env := testEnv(t, ab, ds.Seal())

		got, err := LoadMCMSAddresses(env, []uint64{chainSel}, v)

		require.NoError(t, err)
		require.Len(t, got, 1)
		var want aptoschain.AccountAddress
		require.NoError(t, want.ParseStringRelaxed(testAptosMCMSAddr))
		require.Equal(t, want, got[chainSel])
	})

	t.Run("no MCMS in datastore", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		ds := datastore.NewMemoryDataStore().Seal()
		env := testEnv(t, ab, ds)

		_, err := LoadMCMSAddresses(env, []uint64{chainSel}, version1_6_0)

		require.ErrorContains(t, err, "no MCMS address found for Aptos chain selector")
	})

	t.Run("wrong version ignored", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		ds := datastore.NewMemoryDataStore()
		wrong := semver.MustParse("1.5.0")
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       testAptosMCMSAddr,
			ChainSelector: chainSel,
			Type:          datastore.ContractType(AptosMCMSType),
			Version:       wrong,
		}))
		env := testEnv(t, ab, ds.Seal())

		_, err := LoadMCMSAddresses(env, []uint64{chainSel}, version1_6_0)

		require.ErrorContains(t, err, "no MCMS address found for Aptos chain selector")
	})

	t.Run("nil version ignored", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       testAptosMCMSAddr,
			ChainSelector: chainSel,
			Type:          datastore.ContractType(AptosMCMSType),
		}))
		env := testEnv(t, ab, ds.Seal())

		var err error
		require.NotPanics(t, func() {
			_, err = LoadMCMSAddresses(env, []uint64{chainSel}, version1_6_0)
		})

		require.ErrorContains(t, err, "no MCMS address found for Aptos chain selector")
	})

	t.Run("invalid address", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		ds := datastore.NewMemoryDataStore()
		v := version1_6_0
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "NotHex",
			ChainSelector: chainSel,
			Type:          datastore.ContractType(AptosMCMSType),
			Version:       &v,
		}))
		env := testEnv(t, ab, ds.Seal())

		_, err := LoadMCMSAddresses(env, []uint64{chainSel}, version1_6_0)

		wantPrefix := fmt.Sprintf(
			"failed to parse MCMS address for Aptos chain selector %d (type=%s, version=%s, address=%s): ",
			chainSel, AptosMCMSType, version1_6_0.String(), "NotHex",
		)
		require.ErrorContains(t, err, wantPrefix)
		var scratch aptoschain.AccountAddress
		parseErr := scratch.ParseStringRelaxed("NotHex")
		require.Error(t, parseErr)
		require.ErrorContains(t, err, parseErr.Error())
	})
}

func testEnv(t *testing.T, ab cldf.AddressBook, ds datastore.DataStore) cldf.Environment {
	t.Helper()
	return *cldf.NewEnvironment(
		"test",
		logger.Nop(),
		ab,
		ds,
		nil,
		nil,
		func() context.Context { return t.Context() },
		ocr.OCRSecrets{},
		chain.NewBlockChains(nil),
	)
}
