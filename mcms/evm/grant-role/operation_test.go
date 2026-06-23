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
	mcmsevmsdk "github.com/smartcontractkit/mcms/sdk/evm"

	"github.com/smartcontractkit/cld-changesets/internal/mcmsrole"
	timelockops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/rbac_timelock"
	evmgrantrole "github.com/smartcontractkit/cld-changesets/mcms/evm/grant-role"
	evmops "github.com/smartcontractkit/cld-changesets/mcms/evm/operations"
)

// TestOpGrantRole deploys a timelock with the deployer as admin and grants the
// executor role to a fresh address, asserting it shows up in the inspector.
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
	_, err = operations.ExecuteOperation(bundle, evmgrantrole.OpGrantRole, chain,
		evmops.EVMCallInput[evmgrantrole.OpGrantRoleInput]{
			ChainSelector: sel,
			Address:       timelockAddr,
			CallInput: evmgrantrole.OpGrantRoleInput{
				Account: grantee,
				RoleID:  [32]byte(mcmsrole.ExecutorRole.ID),
			},
		})
	require.NoError(t, err)

	executors, err := mcmsevmsdk.NewTimelockInspector(chain.Client).GetExecutors(t.Context(), timelockAddr.Hex())
	require.NoError(t, err)
	require.Contains(t, executors, grantee.Hex())
}
