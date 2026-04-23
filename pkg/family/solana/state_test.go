package solana

import (
	"context"
	"fmt"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestMaybeLoadMCMSWithTimelockChainState_NoMatchingRefs(t *testing.T) {
	t.Parallel()

	t.Run("nil refs", func(t *testing.T) {
		t.Parallel()
		got, err := MaybeLoadMCMSWithTimelockChainState(nil)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.NotNil(t, got.MCMSWithTimelockPrograms)
		require.Equal(t, solana.PublicKey{}, got.McmProgram)
	})

	t.Run("empty refs", func(t *testing.T) {
		t.Parallel()
		got, err := MaybeLoadMCMSWithTimelockChainState([]datastore.AddressRef{})
		require.NoError(t, err)
		require.NotNil(t, got)
		require.NotNil(t, got.MCMSWithTimelockPrograms)
		require.Equal(t, solana.PublicKey{}, got.McmProgram)
	})
}

func TestMaybeLoadMCMSWithTimelockState(t *testing.T) {
	t.Parallel()

	const chain1 uint64 = 100_001
	const chain2 uint64 = 100_002
	const mcmProgramAddr = "11111111111111111111111111111111"
	const otherProgramAddr = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

	t.Run("nil and empty chain selector list", func(t *testing.T) {
		t.Parallel()
		env := testSolanaEnv(t, datastore.NewMemoryDataStore().Seal())

		got, err := MaybeLoadMCMSWithTimelockState(env, nil)
		require.NoError(t, err)
		require.Empty(t, got)

		got, err = MaybeLoadMCMSWithTimelockState(env, []uint64{})
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("single chain no refs", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		env := testSolanaEnv(t, ds.Seal())
		got, err := MaybeLoadMCMSWithTimelockState(env, []uint64{chain1})
		require.NoError(t, err)
		require.Len(t, got, 1)
		st := got[chain1]
		require.NotNil(t, st)
		require.NotNil(t, st.MCMSWithTimelockPrograms)
		require.Equal(t, solana.PublicKey{}, st.McmProgram)
	})

	t.Run("two chains isolated by selector", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       mcmProgramAddr,
			ChainSelector: chain1,
			Type:          datastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
		}))
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       otherProgramAddr,
			ChainSelector: chain2,
			Type:          datastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
		}))
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       otherProgramAddr,
			ChainSelector: chain2,
			Type:          datastore.ContractType(mcmscontracts.RBACTimelockProgram),
		}))
		env := testSolanaEnv(t, ds.Seal())
		got, err := MaybeLoadMCMSWithTimelockState(env, []uint64{chain1, chain2})
		require.NoError(t, err)
		require.Len(t, got, 2)

		wantMcm1 := solana.MustPublicKeyFromBase58(mcmProgramAddr)
		wantMcm2 := solana.MustPublicKeyFromBase58(otherProgramAddr)
		require.Equal(t, wantMcm1, got[chain1].McmProgram)
		require.Equal(t, solana.PublicKey{}, got[chain1].TimelockProgram)

		require.Equal(t, wantMcm2, got[chain2].McmProgram)
		require.Equal(t, wantMcm2, got[chain2].TimelockProgram)
	})

	t.Run("invalid address returns wrapped error", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "not-valid-base58",
			ChainSelector: chain1,
			Type:          datastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
		}))
		env := testSolanaEnv(t, ds.Seal())
		_, err := MaybeLoadMCMSWithTimelockState(env, []uint64{chain1})
		require.ErrorContains(t, err, fmt.Sprintf(
			"unable to load mcms and timelock solana chain state for chain selector %d: unable to parse public key from mcm address",
			chain1))
	})
}

func testSolanaEnv(t *testing.T, ds datastore.DataStore) cldf.Environment {
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
		chain.NewBlockChains(nil),
	)
}
