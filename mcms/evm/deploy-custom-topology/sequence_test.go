package evmdeploytopology

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmsevmsdk "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	deploycustomtopology "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy-custom-topology"
)

// TestRunEVMDeployTopology_StandardTopology deploys the classic 3-MCM + timelock +
// call-proxy stack and asserts MCM configs and the standard role wiring.
func TestRunEVMDeployTopology_StandardTopology(t *testing.T) {
	t.Parallel()

	sel := chain_selectors.TEST_90000001.Selector
	rt := newEVMRuntime(t, sel)
	env := rt.Environment()
	chain := env.BlockChains.EVMChains()[sel]
	cfg := mcmConfig(t)

	out := runDeployTopologySequence(t, rt, chainTopologyConfig(sel, deploycustomtopology.ChainTopologyConfig{
		MCMs: []deploycustomtopology.MCMSpec{
			mcmSpec("proposer", mcmscontracts.ProposerManyChainMultisig, "CCIP", cfg),
			mcmSpec("canceller", mcmscontracts.CancellerManyChainMultisig, "CCIP", cfg),
			mcmSpec("bypasser", mcmscontracts.BypasserManyChainMultisig, "CCIP", cfg),
		},
		Timelocks: []deploycustomtopology.TimelockSpec{{
			Ref:       "timelock",
			MinDelay:  big.NewInt(0),
			Qualifier: "CCIP",
			Roles: deploycustomtopology.RoleAssignments{
				Proposers:  []deploycustomtopology.RoleHolder{{MCMRef: "proposer"}},
				Cancellers: []deploycustomtopology.RoleHolder{{MCMRef: "proposer"}, {MCMRef: "canceller"}, {MCMRef: "bypasser"}},
				Bypassers:  []deploycustomtopology.RoleHolder{{MCMRef: "bypasser"}},
			},
		}},
	}))

	proposer := fetchAddrFromOutput(t, out, sel, mcmscontracts.ProposerManyChainMultisig, "CCIP")
	canceller := fetchAddrFromOutput(t, out, sel, mcmscontracts.CancellerManyChainMultisig, "CCIP")
	bypasser := fetchAddrFromOutput(t, out, sel, mcmscontracts.BypasserManyChainMultisig, "CCIP")
	timelock := fetchAddrFromOutput(t, out, sel, mcmscontracts.RBACTimelock, "CCIP")
	callProxy := fetchAddrFromOutput(t, out, sel, mcmscontracts.CallProxy, "CCIP")

	ctx := t.Context()
	inspector := mcmsevmsdk.NewInspector(chain.Client)
	for _, addr := range []common.Address{proposer, canceller, bypasser} {
		got, err := inspector.GetConfig(ctx, addr.Hex())
		require.NoError(t, err)
		require.ElementsMatch(t, cfg.Signers, got.Signers)
		require.Equal(t, cfg.Quorum, got.Quorum)
	}

	requireTimelockRoleMembers(
		ctx, t, chain, timelock,
		[]string{proposer.Hex()},
		[]string{callProxy.Hex()},
		[]string{proposer.Hex(), canceller.Hex(), bypasser.Hex()},
		[]string{bypasser.Hex()},
	)
}

// TestRunEVMDeployTopology_ProposerOnly deploys a single proposer MCM + timelock + call proxy.
func TestRunEVMDeployTopology_ProposerOnly(t *testing.T) {
	t.Parallel()

	sel := chain_selectors.TEST_90000001.Selector
	rt := newEVMRuntime(t, sel)
	env := rt.Environment()
	chain := env.BlockChains.EVMChains()[sel]
	cfg := mcmConfig(t)

	out := runDeployTopologySequence(t, rt, chainTopologyConfig(sel, deploycustomtopology.ChainTopologyConfig{
		MCMs: []deploycustomtopology.MCMSpec{
			mcmSpec("proposer", mcmscontracts.ProposerManyChainMultisig, "CCIP", cfg),
		},
		Timelocks: []deploycustomtopology.TimelockSpec{{
			Ref:       "timelock",
			MinDelay:  big.NewInt(3600),
			Qualifier: "CCIP",
			Roles: deploycustomtopology.RoleAssignments{
				Proposers: []deploycustomtopology.RoleHolder{{MCMRef: "proposer"}},
			},
		}},
	}))

	proposer := fetchAddrFromOutput(t, out, sel, mcmscontracts.ProposerManyChainMultisig, "CCIP")
	timelock := fetchAddrFromOutput(t, out, sel, mcmscontracts.RBACTimelock, "CCIP")
	callProxy := fetchAddrFromOutput(t, out, sel, mcmscontracts.CallProxy, "CCIP")
	require.Empty(t, fetchAddrsFromOutput(t, out, sel, mcmscontracts.CancellerManyChainMultisig, "CCIP"))

	requireTimelockRoleMembers(
		t.Context(), t, chain, timelock,
		[]string{proposer.Hex()},
		[]string{callProxy.Hex()},
		nil,
		nil,
	)
}

// TestRunEVMDeployTopology_ExtraArgsDisablesCallProxy disables the call proxy for a
// timelock via ExtraArgs.
func TestRunEVMDeployTopology_ExtraArgsDisablesCallProxy(t *testing.T) {
	t.Parallel()

	sel := chain_selectors.TEST_90000001.Selector
	rt := newEVMRuntime(t, sel)
	env := rt.Environment()
	chain := env.BlockChains.EVMChains()[sel]
	cfg := mcmConfig(t)

	out := runDeployTopologySequence(t, rt, chainTopologyConfig(sel, deploycustomtopology.ChainTopologyConfig{
		MCMs: []deploycustomtopology.MCMSpec{
			mcmSpec("proposer", mcmscontracts.ProposerManyChainMultisig, "CCIP", cfg),
		},
		Timelocks: []deploycustomtopology.TimelockSpec{{
			Ref:       "timelock",
			MinDelay:  big.NewInt(0),
			Qualifier: "CCIP",
			Roles: deploycustomtopology.RoleAssignments{
				Proposers: []deploycustomtopology.RoleHolder{{MCMRef: "proposer"}},
			},
		}},
		ExtraArgs: EVMExtraArgs{
			DeployCallProxyByTimelockRef: map[string]bool{"timelock": false},
		},
	}))

	require.Empty(t, fetchAddrsFromOutput(t, out, sel, mcmscontracts.CallProxy, "CCIP"))

	proposer := fetchAddrFromOutput(t, out, sel, mcmscontracts.ProposerManyChainMultisig, "CCIP")
	timelock := fetchAddrFromOutput(t, out, sel, mcmscontracts.RBACTimelock, "CCIP")

	requireTimelockRoleMembers(
		t.Context(), t, chain, timelock,
		[]string{proposer.Hex()},
		nil,
		nil,
		nil,
	)
}

// TestRunEVMDeployTopology_TwoTimelocksWithOwnershipTransfer deploys two independent
// stacks (CCIP + RMN) on one chain, each taking ownership of its bypasser MCM, and
// emits accept-ownership batch ops grouped per timelock qualifier.
func TestRunEVMDeployTopology_TwoTimelocksWithOwnershipTransfer(t *testing.T) {
	t.Parallel()

	sel := chain_selectors.TEST_90000001.Selector
	rt := newEVMRuntime(t, sel)
	env := rt.Environment()
	chain := env.BlockChains.EVMChains()[sel]
	cfg := mcmConfig(t)

	stack := func(qualifier string) deploycustomtopology.ChainTopologyConfig {
		return deploycustomtopology.ChainTopologyConfig{
			MCMs: []deploycustomtopology.MCMSpec{
				mcmSpec(qualifier+"-proposer", mcmscontracts.ProposerManyChainMultisig, qualifier, cfg),
				mcmSpec(qualifier+"-bypasser", mcmscontracts.BypasserManyChainMultisig, qualifier, cfg),
			},
			Timelocks: []deploycustomtopology.TimelockSpec{{
				Ref:       qualifier + "-timelock",
				MinDelay:  big.NewInt(0),
				Qualifier: qualifier,
				Roles: deploycustomtopology.RoleAssignments{
					Proposers: []deploycustomtopology.RoleHolder{{MCMRef: qualifier + "-proposer"}},
					Bypassers: []deploycustomtopology.RoleHolder{{MCMRef: qualifier + "-bypasser"}},
				},
				TransferOwnership: []deploycustomtopology.RoleHolder{{MCMRef: qualifier + "-bypasser"}},
			}},
		}
	}

	ccip := stack("CCIP")
	rmn := stack("RMN")
	merged := deploycustomtopology.ChainTopologyConfig{
		MCMs:      append(append([]deploycustomtopology.MCMSpec{}, ccip.MCMs...), rmn.MCMs...),
		Timelocks: append(append([]deploycustomtopology.TimelockSpec{}, ccip.Timelocks...), rmn.Timelocks...),
	}

	out := runDeployTopologySequence(t, rt, deploycustomtopology.ChainInput{
		ChainSelector: sel,
		Config:        merged,
		MCMS: &cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			TimelockDelay:  mcmstypes.NewDuration(time.Second),
			Description:    "accept ownership",
		},
	})

	require.Len(t, out.ProposalGroups, 2)

	ccipProposer := fetchAddrFromOutput(t, out, sel, mcmscontracts.ProposerManyChainMultisig, "CCIP")
	ccipBypasser := fetchAddrFromOutput(t, out, sel, mcmscontracts.BypasserManyChainMultisig, "CCIP")
	ccipTimelock := fetchAddrFromOutput(t, out, sel, mcmscontracts.RBACTimelock, "CCIP")
	ccipCallProxy := fetchAddrFromOutput(t, out, sel, mcmscontracts.CallProxy, "CCIP")

	rmnProposer := fetchAddrFromOutput(t, out, sel, mcmscontracts.ProposerManyChainMultisig, "RMN")
	rmnBypasser := fetchAddrFromOutput(t, out, sel, mcmscontracts.BypasserManyChainMultisig, "RMN")
	rmnTimelock := fetchAddrFromOutput(t, out, sel, mcmscontracts.RBACTimelock, "RMN")
	rmnCallProxy := fetchAddrFromOutput(t, out, sel, mcmscontracts.CallProxy, "RMN")

	ctx := t.Context()
	requireTimelockRoleMembers(
		ctx, t, chain, ccipTimelock,
		[]string{ccipProposer.Hex()},
		[]string{ccipCallProxy.Hex()},
		nil,
		[]string{ccipBypasser.Hex()},
	)
	requireTimelockRoleMembers(
		ctx, t, chain, rmnTimelock,
		[]string{rmnProposer.Hex()},
		[]string{rmnCallProxy.Hex()},
		nil,
		[]string{rmnBypasser.Hex()},
	)

	gotQualifiers := map[string]bool{}
	gotAcceptTargets := map[string]bool{}
	for _, g := range out.ProposalGroups {
		gotQualifiers[g.Qualifier] = true
		for _, batch := range g.BatchOps {
			for _, tx := range batch.Transactions {
				gotAcceptTargets[common.HexToAddress(tx.To).Hex()] = true
			}
		}
	}
	require.Equal(t, map[string]bool{"CCIP": true, "RMN": true}, gotQualifiers)
	require.Equal(t, map[string]bool{ccipBypasser.Hex(): true, rmnBypasser.Hex(): true}, gotAcceptTargets)
}

func TestResolveHolders(t *testing.T) {
	t.Parallel()

	mcm := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	direct := common.HexToAddress("0x00000000000000000000000000000000000000bb")
	refToAddr := map[string]common.Address{"proposer": mcm}

	t.Run("resolves mcmRef and address", func(t *testing.T) {
		t.Parallel()
		got, err := resolveHolders([]deploycustomtopology.RoleHolder{
			{MCMRef: "proposer"},
			{Address: &direct},
		}, refToAddr)
		require.NoError(t, err)
		require.Equal(t, []common.Address{mcm, direct}, got)
	})

	t.Run("missing mcmRef errors", func(t *testing.T) {
		t.Parallel()
		_, err := resolveHolders([]deploycustomtopology.RoleHolder{{MCMRef: "missing"}}, refToAddr)
		require.ErrorContains(t, err, "not been deployed")
	})

	t.Run("empty holder errors", func(t *testing.T) {
		t.Parallel()
		_, err := resolveHolders([]deploycustomtopology.RoleHolder{{}}, refToAddr)
		require.ErrorContains(t, err, "exactly one of")
	})
}

func newEVMRuntime(t *testing.T, selectors ...uint64) *runtime.Runtime {
	t.Helper()

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, selectors),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	return rt
}

func runDeployTopologySequence(
	t *testing.T,
	rt *runtime.Runtime,
	chainInput deploycustomtopology.ChainInput,
) deploycustomtopology.ChainOutput {
	t.Helper()

	env := rt.Environment()
	report, err := operations.ExecuteSequence(
		env.OperationsBundle,
		Registration().Sequence,
		deploycustomtopology.Deps{
			BlockChains: env.BlockChains,
			DataStore:   env.DataStore,
		},
		chainInput,
	)
	require.NoError(t, err)

	return report.Output
}

func fetchAddrsFromOutput(
	t *testing.T,
	out deploycustomtopology.ChainOutput,
	sel uint64,
	ct cldf.ContractType,
	qualifier string,
) []common.Address {
	t.Helper()

	var addrs []common.Address
	for _, ref := range out.Metadata.Addresses {
		if ref.ChainSelector == sel && ref.Type == cldfdatastore.ContractType(ct) && ref.Qualifier == qualifier {
			addrs = append(addrs, common.HexToAddress(ref.Address))
		}
	}

	return addrs
}

func fetchAddrFromOutput(
	t *testing.T,
	out deploycustomtopology.ChainOutput,
	sel uint64,
	ct cldf.ContractType,
	qualifier string,
) common.Address {
	t.Helper()

	addrs := fetchAddrsFromOutput(t, out, sel, ct, qualifier)
	require.Lenf(t, addrs, 1, "expected exactly one %s (%s)", ct, qualifier)

	return addrs[0]
}

func mcmConfig(t *testing.T) mcmstypes.Config {
	t.Helper()

	return cldftesthelpers.SingleGroupMCMS(t)
}

func mcmSpec(ref string, ct cldf.ContractType, qualifier string, cfg mcmstypes.Config) deploycustomtopology.MCMSpec {
	return deploycustomtopology.MCMSpec{Ref: ref, Config: cfg, ContractType: ct, Qualifier: qualifier}
}

func chainTopologyConfig(sel uint64, cfg deploycustomtopology.ChainTopologyConfig) deploycustomtopology.ChainInput {
	return deploycustomtopology.ChainInput{
		ChainSelector: sel,
		Config:        cfg,
		ExtraArgs:     cfg.ExtraArgs,
	}
}

func timelockRoles(ctx context.Context, t *testing.T, chain cldf_evm.Chain, timelock common.Address) (proposers, executors, cancellers, bypassers []string) {
	t.Helper()

	insp := mcmsevmsdk.NewTimelockInspector(chain.Client)
	var err error
	proposers, err = insp.GetProposers(ctx, timelock.Hex())
	require.NoError(t, err)
	executors, err = insp.GetExecutors(ctx, timelock.Hex())
	require.NoError(t, err)
	cancellers, err = insp.GetCancellers(ctx, timelock.Hex())
	require.NoError(t, err)
	bypassers, err = insp.GetBypassers(ctx, timelock.Hex())
	require.NoError(t, err)

	return proposers, executors, cancellers, bypassers
}

// requireTimelockRoleMembers asserts timelock role membership via the on-chain inspector.
func requireTimelockRoleMembers(
	ctx context.Context,
	t *testing.T,
	chain cldf_evm.Chain,
	timelock common.Address,
	wantProposers, wantExecutors, wantCancellers, wantBypassers []string,
) {
	t.Helper()

	proposers, executors, cancellers, bypassers := timelockRoles(ctx, t, chain, timelock)
	require.Equal(t, normalizeRoleMembers(wantProposers), proposers, "proposer role members")
	require.Equal(t, normalizeRoleMembers(wantExecutors), executors, "executor role members")
	require.Equal(t, normalizeRoleMembers(wantCancellers), cancellers, "canceller role members")
	require.Equal(t, normalizeRoleMembers(wantBypassers), bypassers, "bypasser role members")
}

func normalizeRoleMembers(addrs []string) []string {
	if addrs == nil {
		return []string{}
	}

	return addrs
}
