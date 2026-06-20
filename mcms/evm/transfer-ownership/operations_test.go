package evmtransferownership_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/mcms/evm/deploy-custom-topology"
	evmops "github.com/smartcontractkit/cld-changesets/mcms/evm/operations"
	evmtransferownership "github.com/smartcontractkit/cld-changesets/mcms/evm/transfer-ownership"
)

// TestOpTransferAndAcceptOwnership deploys a proposer MCM, transfers
// ownership to a placeholder address with the deployer key, and then builds an
// acceptOwnership() calldata via the no-send accept op.
func TestOpTransferAndAcceptOwnership(t *testing.T) {
	t.Parallel()

	sel := chain_selectors.TEST_90000001.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{sel}),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	env := rt.Environment()
	chain := env.BlockChains.EVMChains()[sel]
	bundle := env.OperationsBundle

	mcmReport, err := operations.ExecuteOperation(bundle, evmdeploytopology.OpDeployProposerMCM, chain,
		evmops.EVMDeployInput[any]{ChainSelector: sel})
	require.NoError(t, err)
	mcmAddr := mcmReport.Output.Address

	mcm, err := bindings.NewManyChainMultiSig(mcmAddr, chain.Client)
	require.NoError(t, err)

	target := common.HexToAddress("0x00000000000000000000000000000000000000bb")
	deps := evmtransferownership.OpOwnershipDeps{Chain: chain, OwnableC: mcm}
	_, err = operations.ExecuteOperation(bundle, evmtransferownership.OpTransferOwnership, deps,
		evmtransferownership.OpTransferOwnershipInput{ChainSelector: sel, TimelockAddress: target, Address: mcmAddr})
	require.NoError(t, err)

	acceptReport, err := operations.ExecuteOperation(bundle, evmtransferownership.OpAcceptOwnership, deps,
		evmtransferownership.OpTransferOwnershipInput{ChainSelector: sel, TimelockAddress: target, Address: mcmAddr})
	require.NoError(t, err)
	require.NotEmpty(t, acceptReport.Output.Tx.Data())
}
