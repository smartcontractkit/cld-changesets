package evmgrantrole_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmsevmsdk "github.com/smartcontractkit/mcms/sdk/evm"

	timelockops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/rbac_timelock"
	evmgrantrole "github.com/smartcontractkit/cld-changesets/mcms/evm/grant-role"
)

func TestOpEVMGrantRoleInputGasOverridable(t *testing.T) {
	t.Parallel()

	in := evmgrantrole.OpEVMGrantRoleInput{GasLimit: 100, GasPrice: 200}
	gotLimit, gotPrice := in.GasBoostValues()
	require.Equal(t, uint64(100), gotLimit)
	require.Equal(t, uint64(200), gotPrice)

	boosted := in.WithGasBoost(500, 600)
	require.Equal(t, uint64(500), boosted.GasLimit)
	require.Equal(t, uint64(600), boosted.GasPrice)
}

// TestOpGrantRole deploys a timelock with the deployer as admin and grants the
// executor role to a fresh address via the MCMS SDK timelock configurer.
func TestOpGrantRole(t *testing.T) {
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

	tlReport, err := operations.ExecuteOperation(bundle, timelockops.Deploy, chain,
		opscontract.DeployInput[timelockops.ConstructorArgs]{
			TypeAndVersion: timelockops.TypeAndVersion,
			Args: timelockops.ConstructorArgs{
				MinDelay:   big.NewInt(0),
				Admin:      chain.DeployerKey.From,
				Proposers:  []common.Address{},
				Executors:  []common.Address{},
				Cancellers: []common.Address{},
				Bypassers:  []common.Address{},
			},
		})
	require.NoError(t, err)
	timelockAddr := common.HexToAddress(tlReport.Output.Address)

	grantee := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	report, err := operations.ExecuteOperation(bundle, evmgrantrole.OpEVMGrantRole, chain,
		evmgrantrole.OpEVMGrantRoleInput{
			Target: evmgrantrole.GrantRoleTarget{
				Timelock: timelockAddr,
				Role:     mcmssdk.TimelockRoleExecutor,
				Address:  grantee,
			},
		})
	require.NoError(t, err)
	require.True(t, report.Output.Executed())

	executors, err := mcmsevmsdk.NewTimelockInspector(chain.Client).GetExecutors(t.Context(), timelockAddr.Hex())
	require.NoError(t, err)
	require.Contains(t, executors, grantee.Hex())
}
