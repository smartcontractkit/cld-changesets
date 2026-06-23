package firedrill_test

import (
	"context"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	firedrill "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill"
	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill/all"
)

func testEnvironment(t *testing.T, ds datastore.DataStore, chains cldf_chain.BlockChains) cldf.Environment {
	t.Helper()

	return *cldf.NewEnvironment(
		"test",
		logger.Test(t),
		nil,
		ds,
		nil,
		nil,
		func() context.Context { return t.Context() },
		ocr.OCRSecrets{},
		chains,
	)
}

func testMCMSInput() *cldf.MCMSTimelockProposalInput {
	return &cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
		Description:    "firedrill test",
	}
}

func TestChangeset_VerifyPreconditions_NoDatastore(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, nil, cldf_chain.NewBlockChains(nil))
	input := firedrill.Input{
		MCMS: testMCMSInput(),
	}

	err := firedrill.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "datastore is required")
}

func TestChangeset_VerifyPreconditions_NoMCMSInput(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal(), cldf_chain.NewBlockChains(nil))
	input := firedrill.Input{}

	err := firedrill.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "MCMS timelock proposal input is required")
}

func TestChangeset_VerifyPreconditions_NoChainsResolved(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal(), cldf_chain.NewBlockChains(nil))
	input := firedrill.Input{MCMS: testMCMSInput()}

	err := firedrill.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "no chain selectors resolved")
}

func TestChangeset_VerifyPreconditions_UnknownChain(t *testing.T) {
	t.Parallel()

	sel := uint64(999991)
	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal(), cldf_chain.NewBlockChains(nil))
	input := firedrill.Input{
		MCMS: testMCMSInput(),
		Cfg:  firedrill.Config{Selectors: []uint64{sel}},
	}

	err := firedrill.Changeset{}.VerifyPreconditions(env, input)
	require.Error(t, err)
	_, famErr := chainselectors.GetSelectorFamily(sel)
	if famErr != nil {
		require.ErrorContains(t, err, famErr.Error())
	} else {
		require.ErrorContains(t, err, "not found in environment")
	}
}

func TestChangeset_VerifyPreconditions_unsupportedChainFamily(t *testing.T) {
	t.Parallel()

	sel := chainselectors.APTOS_MAINNET.Selector
	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal(), cldf_chain.NewBlockChains(nil))
	input := firedrill.Input{
		MCMS: testMCMSInput(),
		Cfg:  firedrill.Config{Selectors: []uint64{sel}},
	}

	err := firedrill.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "no sequence registered for family")
}

func TestChangeset_VerifyPreconditions_evmChainNotInEnvironment(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	solSel := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		solSel: cldf_solana.Chain{Selector: solSel},
	}))
	input := firedrill.Input{
		MCMS: testMCMSInput(),
		Cfg:  firedrill.Config{Selectors: []uint64{evmSel}},
	}

	err := firedrill.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "EVM chain")
	require.ErrorContains(t, err, "not found in environment")
}

func TestChangeset_VerifyPreconditions_solanaChainNotInEnvironment(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	solSel := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		evmSel: cldf_evm.Chain{Selector: evmSel},
	}))
	input := firedrill.Input{
		MCMS: testMCMSInput(),
		Cfg:  firedrill.Config{Selectors: []uint64{solSel}},
	}

	err := firedrill.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "solana chain")
	require.ErrorContains(t, err, "not found in environment")
}

func TestChangeset_VerifyPreconditions_missingDatastoreEntry(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		evmSel: cldf_evm.Chain{Selector: evmSel},
	}))
	input := firedrill.Input{
		MCMS: testMCMSInput(),
		Cfg:  firedrill.Config{Selectors: []uint64{evmSel}},
	}

	err := firedrill.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "validate timelock ref")
}

func TestConfig_ResolvedSelectors_defaultOrderSolanaBeforeEVM(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	solSel := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		evmSel: cldf_evm.Chain{Selector: evmSel},
		solSel: cldf_solana.Chain{Selector: solSel},
	}))

	got := firedrill.Config{}.ResolvedSelectors(env)
	require.Equal(t, []uint64{solSel, evmSel}, got)
}

func TestConfig_ResolvedSelectors_explicitPreservesInputOrder(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	solSel := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		evmSel: cldf_evm.Chain{Selector: evmSel},
		solSel: cldf_solana.Chain{Selector: solSel},
	}))

	got := firedrill.Config{Selectors: []uint64{evmSel, solSel}}.ResolvedSelectors(env)
	require.Equal(t, []uint64{evmSel, solSel}, got)
}

func TestChangeset_Apply_returnsErrorWhenNoChainsResolved(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal(), cldf_chain.NewBlockChains(nil))
	input := firedrill.Input{MCMS: testMCMSInput()}

	out, err := firedrill.Changeset{}.Apply(env, input)
	require.ErrorContains(t, err, "no chain selectors resolved")
	require.Nil(t, out.DataStore)
	require.Empty(t, out.MCMSTimelockProposals)
}

func TestChangeset_VerifyPreconditions_evmRefsPresent(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "0x0000000000000000000000000000000000000100",
		ChainSelector: evmSel,
		Type:          datastore.ContractType(mcmscontracts.RBACTimelock),
		Version:       testVersion(t),
	}))
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "0x0000000000000000000000000000000000000200",
		ChainSelector: evmSel,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       testVersion(t),
	}))

	env := testEnvironment(t, ds.Seal(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		evmSel: cldf_evm.Chain{Selector: evmSel},
	}))
	input := firedrill.Input{
		MCMS: testMCMSInput(),
		Cfg:  firedrill.Config{Selectors: []uint64{evmSel}},
	}

	err := firedrill.Changeset{}.VerifyPreconditions(env, input)
	require.NoError(t, err)
}

func testVersion(t *testing.T) *semver.Version {
	t.Helper()

	return semver.MustParse("1.0.0")
}
