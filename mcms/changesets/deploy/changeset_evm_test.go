package deploy_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmsevmsdk "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/internal/testutil/evmtest"
	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"

	// Import EVM deploy package to auto-register the EVM family via init().
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy"
)

// mcmsConfig returns a minimal valid MCMS+timelock config for tests.
func mcmsConfig(t *testing.T, minDelay int64) cldfproposalutils.MCMSWithTimelockConfig {
	t.Helper()

	signer := func(hex string) mcmstypes.Config {
		return mcmstypes.Config{
			Quorum:       1,
			Signers:      []common.Address{common.HexToAddress(hex)},
			GroupSigners: []mcmstypes.Config{},
		}
	}

	return cldfproposalutils.MCMSWithTimelockConfig{
		Proposer:         signer("0x0000000000000000000000000000000000000001"),
		Canceller:        signer("0x0000000000000000000000000000000000000002"),
		Bypasser:         signer("0x0000000000000000000000000000000000000003"),
		TimelockMinDelay: big.NewInt(minDelay),
	}
}

func TestDeployMCMSWithTimelock_FreshDeploy(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	cfg := mcmsConfig(t, 0)
	err = rt.Exec(runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
		ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cfg,
		},
	}))
	require.NoError(t, err)

	var reportsLen int
	for _, out := range rt.State().Outputs {
		reportsLen += len(out.Reports)
	}
	require.NotZero(t, reportsLen, "expected operation reports in changeset output")

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 5, "expected 5 MCMS contract address refs")

	contractTypes := make(map[datastore.ContractType]struct{}, 5)
	for _, ref := range refs {
		require.Equal(t, selector, ref.ChainSelector)
		require.True(t, semvers.V1_0_0.Equal(ref.Version))
		contractTypes[ref.Type] = struct{}{}
	}
	require.Contains(t, contractTypes, datastore.ContractType(mcmscontracts.BypasserManyChainMultisig))
	require.Contains(t, contractTypes, datastore.ContractType(mcmscontracts.CancellerManyChainMultisig))
	require.Contains(t, contractTypes, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig))
	require.Contains(t, contractTypes, datastore.ContractType(mcmscontracts.RBACTimelock))
	require.Contains(t, contractTypes, datastore.ContractType(mcmscontracts.CallProxy))

	state, err := evmstate.MaybeLoadMCMSWithTimelockStateDataStore(rt.Environment(), []uint64{selector})
	require.NoError(t, err)
	require.NoError(t, state[selector].Validate())

	chain := rt.Environment().BlockChains.EVMChains()[selector]
	timelockInspector := mcmsevmsdk.NewTimelockInspector(chain.Client)
	timelockAddr := state[selector].Timelock.Address().Hex()

	proposers, err := timelockInspector.GetProposers(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{state[selector].ProposerMcm.Address().Hex()}, proposers)

	executors, err := timelockInspector.GetExecutors(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{state[selector].CallProxy.Address().Hex()}, executors)

	cancellers, err := timelockInspector.GetCancellers(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		state[selector].CancellerMcm.Address().Hex(),
		state[selector].ProposerMcm.Address().Hex(),
		state[selector].BypasserMcm.Address().Hex(),
	}, cancellers)

	bypassers, err := timelockInspector.GetBypassers(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{state[selector].BypasserMcm.Address().Hex()}, bypassers)
}

func TestDeployMCMSWithTimelock_PartialDeploy(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector

	bypasserAddr := evmtest.RandomAddress()
	cancellerAddr := evmtest.RandomAddress()

	v := semvers.V1_0_0
	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		ChainSelector: selector,
		Address:       bypasserAddr.Hex(),
		Type:          datastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
		Version:       &v,
	}))
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		ChainSelector: selector,
		Address:       cancellerAddr.Hex(),
		Type:          datastore.ContractType(mcmscontracts.CancellerManyChainMultisig),
		Version:       &v,
	}))

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
		environment.WithLogger(logger.Test(t)),
		environment.WithDatastore(ds.Seal()),
	))
	require.NoError(t, err)

	err = rt.Exec(runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
		ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: mcmsConfig(t, 0),
		},
	}))
	require.NoError(t, err)

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 5)

	state, err := evmstate.MaybeLoadMCMSWithTimelockStateDataStore(rt.Environment(), []uint64{selector})
	require.NoError(t, err)
	require.NoError(t, state[selector].Validate())
	require.Equal(t, bypasserAddr, state[selector].BypasserMcm.Address())
	require.Equal(t, cancellerAddr, state[selector].CancellerMcm.Address())
}

func TestDeployMCMSWithTimelock_MultiChain(t *testing.T) {
	t.Parallel()

	selector1 := chain_selectors.TEST_90000001.Selector
	selector2 := chain_selectors.TEST_90000002.Selector
	selectors := []uint64{selector1, selector2}

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, selectors),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	err = rt.Exec(runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
		ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector1: mcmsConfig(t, 0),
			selector2: mcmsConfig(t, 1),
		},
	}))
	require.NoError(t, err)

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 10, "expected 5 contracts per chain")

	state, err := evmstate.MaybeLoadMCMSWithTimelockStateDataStore(rt.Environment(), selectors)
	require.NoError(t, err)
	require.NoError(t, state[selector1].Validate())
	require.NoError(t, state[selector2].Validate())
}
