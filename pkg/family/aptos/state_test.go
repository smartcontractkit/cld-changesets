package aptos

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	aptoschain "github.com/aptos-labs/aptos-go-sdk"
	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/pkg/common"
)

const testAptosMCMSAddr = "0x3"

func TestLoadMCMSAddresses(t *testing.T) {
	t.Parallel()

	chainSel := chainsel.APTOS_TESTNET.Selector

	t.Run("empty selectors", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		ds := datastore.NewMemoryDataStore().Seal()
		env := testEnv(t, ab, ds)

		got, err := LoadMCMSAddresses(env, nil)

		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		require.NoError(t, ab.Save(chainSel, testAptosMCMSAddr, cldf.NewTypeAndVersion(AptosMCMSType, common.Version1_6_0)))
		ds := datastore.NewMemoryDataStore().Seal()
		env := testEnv(t, ab, ds)

		got, err := LoadMCMSAddresses(env, []uint64{chainSel})

		require.NoError(t, err)
		require.Len(t, got, 1)
		var want aptoschain.AccountAddress
		require.NoError(t, want.ParseStringRelaxed(testAptosMCMSAddr))
		require.Equal(t, want, got[chainSel])
	})

	t.Run("no MCMS on chain", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		other := cldf.NewTypeAndVersion("SomeOtherContract", common.Version1_0_0)
		require.NoError(t, ab.Save(chainSel, "0x1", other))
		ds := datastore.NewMemoryDataStore().Seal()
		env := testEnv(t, ab, ds)

		_, err := LoadMCMSAddresses(env, []uint64{chainSel})

		require.ErrorContains(t, err, fmt.Sprintf("no MCMS address found for Aptos chain: %d", chainSel))
	})

	t.Run("chain not in address book", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		ds := datastore.NewMemoryDataStore().Seal()
		env := testEnv(t, ab, ds)

		_, err := LoadMCMSAddresses(env, []uint64{chainSel})

		require.ErrorContains(t, err, fmt.Sprintf("failed to load addresses for Aptos chain %d:", chainSel))
		require.ErrorIs(t, err, cldf.ErrChainNotFound)
	})

	t.Run("wrong version ignored", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		require.NoError(t, ab.Save(chainSel, testAptosMCMSAddr, cldf.NewTypeAndVersion(AptosMCMSType, common.Version1_5_0)))
		ds := datastore.NewMemoryDataStore().Seal()
		env := testEnv(t, ab, ds)

		_, err := LoadMCMSAddresses(env, []uint64{chainSel})

		require.ErrorContains(t, err, fmt.Sprintf("no MCMS address found for Aptos chain: %d", chainSel))
	})

	t.Run("invalid address", func(t *testing.T) {
		t.Parallel()

		ab := cldf.NewMemoryAddressBook()
		require.NoError(t, ab.Save(chainSel, "NotHex", cldf.NewTypeAndVersion(AptosMCMSType, common.Version1_6_0)))
		ds := datastore.NewMemoryDataStore().Seal()
		env := testEnv(t, ab, ds)

		_, err := LoadMCMSAddresses(env, []uint64{chainSel})

		require.ErrorContains(t, err, fmt.Sprintf(
			"failed to parse Aptos MCMS address for chain %d (type=%s, version=%s, address=NotHex)",
			chainSel, AptosMCMSType, common.Version1_6_0.String(),
		))
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
