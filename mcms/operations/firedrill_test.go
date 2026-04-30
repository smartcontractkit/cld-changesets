package operations

import (
	"context"
	"testing"

	chainsel "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	evmstate "github.com/smartcontractkit/cld-changesets/pkg/family/evm"
)

func testFireDrillEnv(t *testing.T, chains cldf_chain.BlockChains) cldf.Environment {
	t.Helper()

	return *cldf.NewEnvironment(
		"test",
		logger.Test(t),
		cldf.NewMemoryAddressBook(),
		datastore.NewMemoryDataStore().Seal(),
		nil,
		nil,
		func() context.Context { return t.Context() },
		ocr.OCRSecrets{},
		chains,
	)
}

func TestBuildMCMSFiredrillProposalOp_noSelectorsResolved(t *testing.T) {
	t.Parallel()

	env := testFireDrillEnv(t, cldf_chain.NewBlockChains(nil))
	input := FireDrillInput{TimelockCfg: cldfproposalutils.TimelockConfig{MCMSAction: mcmstypes.TimelockActionSchedule}}

	_, err := fwops.ExecuteOperation[FireDrillInput, FireDrillOutput, FireDrillDeps](
		env.OperationsBundle,
		BuildMCMSFiredrillProposalOp,
		FireDrillDeps{Environment: env},
		input,
		fwops.WithForceExecute[FireDrillInput, FireDrillDeps](),
	)
	require.ErrorContains(t, err, "no chain selectors resolved")
}

func TestBuildMCMSFiredrillProposalOp_evmChainMissingFromEnvironment(t *testing.T) {
	t.Parallel()

	evmSel := chainsel.TEST_90000002.Selector
	env := testFireDrillEnv(t, cldf_chain.NewBlockChains(nil))
	input := FireDrillInput{
		TimelockCfg: cldfproposalutils.TimelockConfig{MCMSAction: mcmstypes.TimelockActionSchedule},
		Selectors:   []uint64{evmSel},
	}

	_, err := fwops.ExecuteOperation[FireDrillInput, FireDrillOutput, FireDrillDeps](
		env.OperationsBundle,
		BuildMCMSFiredrillProposalOp,
		FireDrillDeps{Environment: env},
		input,
		fwops.WithForceExecute[FireDrillInput, FireDrillDeps](),
	)
	require.ErrorContains(t, err, "evm chain")
	require.ErrorContains(t, err, "not found in environment")
}

func TestBuildNoOPSolana(t *testing.T) {
	t.Parallel()

	tx, err := buildNoOPSolana()
	require.NoError(t, err)
	require.Equal(t, "Memo", tx.ContractType)
	require.NotEmpty(t, tx.To)
}

func TestBuildNoOPEVM_requiresTimelockBinding(t *testing.T) {
	t.Parallel()

	_, err := buildNoOPEVM(nil)
	require.ErrorContains(t, err, "timelock binding is required")

	_, err = buildNoOPEVM(&evmstate.MCMSWithTimelockState{})
	require.ErrorContains(t, err, "timelock binding is required")
}
