package mcms

import (
	"context"
	"testing"

	chainsel "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"
)

func TestChainMetadata_Set_initializesNestedMap(t *testing.T) {
	t.Parallel()

	m := make(ChainMetadata)
	m.Set(42, "role", "proposer")

	require.Contains(t, m, uint64(42))
	require.Equal(t, "proposer", m[42]["role"])
}

func TestBuildProposalFromBatchesV2_emptyBatches(t *testing.T) {
	t.Parallel()

	evmSel := chainsel.TEST_90000002.Selector
	env := *cldf.NewEnvironment(
		"test",
		logger.Nop(),
		cldf.NewMemoryAddressBook(),
		datastore.NewMemoryDataStore().Seal(),
		nil,
		nil,
		func() context.Context { return t.Context() },
		ocr.OCRSecrets{},
		cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
			evmSel: cldf_evm.Chain{Selector: evmSel},
		}),
	)

	_, err := BuildProposalFromBatchesV2(
		env,
		map[uint64]string{evmSel: "0x1"},
		map[uint64]string{evmSel: "0x2"},
		nil,
		nil,
		"desc",
		cldfproposalutils.TimelockConfig{MCMSAction: mcmstypes.TimelockActionSchedule},
	)
	require.ErrorContains(t, err, "no operations in batch")
}
