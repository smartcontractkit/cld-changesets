package changesets

import (
	"testing"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"
)

func TestMCMSSignFireDrillChangeset_VerifyPreconditions_NoChainsResolved(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(nil))
	cfg := FireDrillConfig{TimelockCfg: cldfproposalutils.TimelockConfig{}}

	err := MCMSSignFireDrillChangeset{}.VerifyPreconditions(env, cfg)
	require.ErrorContains(t, err, "no chain selectors resolved")
}

func TestMCMSSignFireDrillChangeset_VerifyPreconditions_UnknownChain(t *testing.T) {
	t.Parallel()

	sel := uint64(999991)
	env := testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(nil))
	cfg := FireDrillConfig{
		TimelockCfg: cldfproposalutils.TimelockConfig{},
		Selectors:   []uint64{sel},
	}

	err := MCMSSignFireDrillChangeset{}.VerifyPreconditions(env, cfg)
	require.Error(t, err)
	_, famErr := chainselectors.GetSelectorFamily(sel)
	if famErr != nil {
		require.ErrorContains(t, err, famErr.Error())
	} else {
		require.ErrorContains(t, err, "not found in environment")
	}
}

func TestMCMSSignFireDrillChangeset_VerifyPreconditions_unsupportedChainFamily(t *testing.T) {
	t.Parallel()

	sel := chainselectors.APTOS_MAINNET.Selector
	env := testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(nil))
	cfg := FireDrillConfig{
		TimelockCfg: cldfproposalutils.TimelockConfig{MCMSAction: mcmstypes.TimelockActionSchedule},
		Selectors:   []uint64{sel},
	}

	err := MCMSSignFireDrillChangeset{}.VerifyPreconditions(env, cfg)
	require.ErrorContains(t, err, "unsupported chain family")
}

func TestMCMSSignFireDrillChangeset_VerifyPreconditions_evmChainNotInEnvironment(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	solSel := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		solSel: cldf_solana.Chain{Selector: solSel},
	}))
	cfg := FireDrillConfig{
		TimelockCfg: cldfproposalutils.TimelockConfig{MCMSAction: mcmstypes.TimelockActionSchedule},
		Selectors:   []uint64{evmSel},
	}

	err := MCMSSignFireDrillChangeset{}.VerifyPreconditions(env, cfg)
	require.ErrorContains(t, err, "evm chain")
	require.ErrorContains(t, err, "not found in environment")
}

func TestMCMSSignFireDrillChangeset_VerifyPreconditions_solanaChainNotInEnvironment(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	solSel := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		evmSel: cldf_evm.Chain{Selector: evmSel},
	}))
	cfg := FireDrillConfig{
		TimelockCfg: cldfproposalutils.TimelockConfig{MCMSAction: mcmstypes.TimelockActionSchedule},
		Selectors:   []uint64{solSel},
	}

	err := MCMSSignFireDrillChangeset{}.VerifyPreconditions(env, cfg)
	require.ErrorContains(t, err, "solana chain")
	require.ErrorContains(t, err, "not found in environment")
}

func TestMCMSSignFireDrillChangeset_VerifyPreconditions_missingAddressBookEntry(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	env := testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		evmSel: cldf_evm.Chain{Selector: evmSel},
	}))
	cfg := FireDrillConfig{
		TimelockCfg: cldfproposalutils.TimelockConfig{MCMSAction: mcmstypes.TimelockActionSchedule},
		Selectors:   []uint64{evmSel},
	}

	err := MCMSSignFireDrillChangeset{}.VerifyPreconditions(env, cfg)
	require.ErrorContains(t, err, "addresses for chain")
}

func TestFireDrillConfig_ResolvedSelectors_defaultOrderSolanaBeforeEVM(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	solSel := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		evmSel: cldf_evm.Chain{Selector: evmSel},
		solSel: cldf_solana.Chain{Selector: solSel},
	}))

	got := FireDrillConfig{TimelockCfg: cldfproposalutils.TimelockConfig{}}.ResolvedSelectors(env)
	require.Equal(t, []uint64{solSel, evmSel}, got)
}

func TestFireDrillConfig_ResolvedSelectors_explicitPreservesInputOrder(t *testing.T) {
	t.Parallel()

	evmSel := chainselectors.TEST_90000002.Selector
	solSel := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		evmSel: cldf_evm.Chain{Selector: evmSel},
		solSel: cldf_solana.Chain{Selector: solSel},
	}))

	got := FireDrillConfig{
		TimelockCfg: cldfproposalutils.TimelockConfig{},
		Selectors:   []uint64{evmSel, solSel},
	}.ResolvedSelectors(env)
	require.Equal(t, []uint64{evmSel, solSel}, got)
}

func TestMCMSSignFireDrillChangeset_Apply_returnsReportOnFailure(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(nil))
	cfg := FireDrillConfig{
		TimelockCfg: cldfproposalutils.TimelockConfig{MCMSAction: mcmstypes.TimelockActionSchedule},
	}

	out, err := MCMSSignFireDrillChangeset{}.Apply(env, cfg)
	require.ErrorContains(t, err, "no chain selectors resolved")
	require.Len(t, out.Reports, 1)
	require.Empty(t, out.MCMSTimelockProposals)
	require.NotNil(t, out.Reports[0].Err)
	require.ErrorContains(t, out.Reports[0].Err, "no chain selectors resolved")
}
