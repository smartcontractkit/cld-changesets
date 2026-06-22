package deploy_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmsevmsdk "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/internal/testutil/evmtest"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	evmreaders "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"

	// Import EVM deploy package to auto-register the EVM family via init().
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy"
)

type mcmsTimelockRefs struct {
	Bypasser  common.Address
	Canceller common.Address
	Proposer  common.Address
	Timelock  common.Address
	CallProxy common.Address
}

func loadMCMSTimelockRefs(t *testing.T, env cldf.Environment, chainSelector uint64) mcmsTimelockRefs {
	t.Helper()

	reader := evmreaders.Reader{}
	proposalInput := cldf.MCMSTimelockProposalInput{}

	timelockRef, err := reader.GetTimelockRef(env, chainSelector, proposalInput)
	require.NoError(t, err)

	proposerRef, err := reader.GetMCMSRef(env, chainSelector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
	})
	require.NoError(t, err)

	cancellerRef, err := reader.GetMCMSRef(env, chainSelector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionCancel,
	})
	require.NoError(t, err)

	bypasserRef, err := reader.GetMCMSRef(env, chainSelector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionBypass,
	})
	require.NoError(t, err)

	callProxyRef, err := reader.GetCallProxyRef(env, chainSelector, proposalInput.Qualifier)
	require.NoError(t, err)

	return mcmsTimelockRefs{
		Bypasser:  common.HexToAddress(bypasserRef.Address),
		Canceller: common.HexToAddress(cancellerRef.Address),
		Proposer:  common.HexToAddress(proposerRef.Address),
		Timelock:  common.HexToAddress(timelockRef.Address),
		CallProxy: common.HexToAddress(callProxyRef.Address),
	}
}

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

	addresses := loadMCMSTimelockRefs(t, rt.Environment(), selector)

	chain := rt.Environment().BlockChains.EVMChains()[selector]
	timelockInspector := mcmsevmsdk.NewTimelockInspector(chain.Client)
	timelockAddr := addresses.Timelock.Hex()

	proposers, err := timelockInspector.GetProposers(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{addresses.Proposer.Hex()}, proposers)

	executors, err := timelockInspector.GetExecutors(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{addresses.CallProxy.Hex()}, executors)

	cancellers, err := timelockInspector.GetCancellers(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		addresses.Canceller.Hex(),
		addresses.Proposer.Hex(),
		addresses.Bypasser.Hex(),
	}, cancellers)

	bypassers, err := timelockInspector.GetBypassers(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{addresses.Bypasser.Hex()}, bypassers)
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

	addresses := loadMCMSTimelockRefs(t, rt.Environment(), selector)
	require.Equal(t, bypasserAddr, addresses.Bypasser)
	require.Equal(t, cancellerAddr, addresses.Canceller)
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

	for _, selector := range selectors {
		loadMCMSTimelockRefs(t, rt.Environment(), selector)
	}
}
