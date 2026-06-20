package evmdeploytopology_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/stretchr/testify/require"

	evmdeploytopology "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy-custom-topology"
	evmops "github.com/smartcontractkit/cld-changesets/mcms/evm/operations"
)

// TestOpDeployTimelock deploys an RBAC timelock with the deployer as admin.
func TestOpDeployTimelock(t *testing.T) {
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

	report, err := operations.ExecuteOperation(bundle, evmdeploytopology.OpDeployTimelock, chain,
		evmops.EVMDeployInput[evmdeploytopology.OpDeployTimelockInput]{
			ChainSelector: sel,
			DeployInput: evmdeploytopology.OpDeployTimelockInput{
				TimelockMinDelay: big.NewInt(0),
				Admin:            chain.DeployerKey.From,
				Proposers:        []common.Address{},
				Executors:        []common.Address{},
				Cancellers:       []common.Address{},
				Bypassers:        []common.Address{},
			},
		})
	require.NoError(t, err)
	require.NotEqual(t, common.Address{}, report.Output.Address)
}

// TestOpDeployCallProxy deploys a CallProxy attached to a freshly deployed timelock.
func TestOpDeployCallProxy(t *testing.T) {
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

	tlReport, err := operations.ExecuteOperation(bundle, evmdeploytopology.OpDeployTimelock, chain,
		evmops.EVMDeployInput[evmdeploytopology.OpDeployTimelockInput]{
			ChainSelector: sel,
			DeployInput: evmdeploytopology.OpDeployTimelockInput{
				TimelockMinDelay: big.NewInt(0),
				Admin:            chain.DeployerKey.From,
				Proposers:        []common.Address{},
				Executors:        []common.Address{},
				Cancellers:       []common.Address{},
				Bypassers:        []common.Address{},
			},
		})
	require.NoError(t, err)

	cpReport, err := operations.ExecuteOperation(bundle, evmdeploytopology.OpDeployCallProxy, chain,
		evmops.EVMDeployInput[evmdeploytopology.OpDeployCallProxyInput]{
			ChainSelector: sel,
			DeployInput:   evmdeploytopology.OpDeployCallProxyInput{Timelock: tlReport.Output.Address},
		})
	require.NoError(t, err)
	require.NotEqual(t, common.Address{}, cpReport.Output.Address)
}
