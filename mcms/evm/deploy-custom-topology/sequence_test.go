package evmdeploytopology

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmsevmsdk "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	deploycustomtopology "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy-custom-topology"
)

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

// TestDeployStandardTopology deploys the classic 3-MCM + timelock + call-proxy
// stack and asserts MCM configs and the standard role wiring.
func TestDeployStandardTopology(t *testing.T) {
	t.Parallel()

	sel := chain_selectors.TEST_90000001.Selector
	rt := newEVMRuntime(t, sel)
	env := rt.Environment()
	chain := env.BlockChains.EVMChains()[sel]
	cfg := mcmConfig(t)

	input := deploycustomtopology.Input{Cfg: deploycustomtopology.Config{
		ChainConfigs: map[uint64]deploycustomtopology.ChainTopologyConfig{
			sel: {
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
			},
		},
	}}

	require.NoError(t, deploycustomtopology.Changeset{}.VerifyPreconditions(env, input))
	out, err := deploycustomtopology.Changeset{}.Apply(env, input)
	require.NoError(t, err)
	require.Empty(t, out.MCMSTimelockProposals)

	proposer := fetchAddr(t, out.DataStore, sel, mcmscontracts.ProposerManyChainMultisig, "CCIP")
	canceller := fetchAddr(t, out.DataStore, sel, mcmscontracts.CancellerManyChainMultisig, "CCIP")
	bypasser := fetchAddr(t, out.DataStore, sel, mcmscontracts.BypasserManyChainMultisig, "CCIP")
	timelock := fetchAddr(t, out.DataStore, sel, mcmscontracts.RBACTimelock, "CCIP")
	callProxy := fetchAddr(t, out.DataStore, sel, mcmscontracts.CallProxy, "CCIP")

	ctx := t.Context()
	inspector := mcmsevmsdk.NewInspector(chain.Client)
	for _, addr := range []common.Address{proposer, canceller, bypasser} {
		got, err := inspector.GetConfig(ctx, addr.Hex())
		require.NoError(t, err)
		require.ElementsMatch(t, cfg.Signers, got.Signers)
		require.Equal(t, cfg.Quorum, got.Quorum)
	}

	proposers, executors, cancellers, bypassers := timelockRoles(ctx, t, chain, timelock)
	require.Equal(t, []string{proposer.Hex()}, proposers)
	require.Equal(t, []string{callProxy.Hex()}, executors)
	require.ElementsMatch(t, []string{proposer.Hex(), canceller.Hex(), bypasser.Hex()}, cancellers)
	require.Equal(t, []string{bypasser.Hex()}, bypassers)
}

// TestDeployProposerOnlyTopology deploys a single proposer MCM + timelock + call proxy.
func TestDeployProposerOnlyTopology(t *testing.T) {
	t.Parallel()

	sel := chain_selectors.TEST_90000001.Selector
	rt := newEVMRuntime(t, sel)
	env := rt.Environment()
	chain := env.BlockChains.EVMChains()[sel]
	cfg := mcmConfig(t)

	input := deploycustomtopology.Input{Cfg: deploycustomtopology.Config{
		ChainConfigs: map[uint64]deploycustomtopology.ChainTopologyConfig{
			sel: {
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
			},
		},
	}}

	out, err := deploycustomtopology.Changeset{}.Apply(env, input)
	require.NoError(t, err)

	proposer := fetchAddr(t, out.DataStore, sel, mcmscontracts.ProposerManyChainMultisig, "CCIP")
	timelock := fetchAddr(t, out.DataStore, sel, mcmscontracts.RBACTimelock, "CCIP")
	callProxy := fetchAddr(t, out.DataStore, sel, mcmscontracts.CallProxy, "CCIP")
	require.Empty(t, fetchAddrs(t, out.DataStore, sel, mcmscontracts.CancellerManyChainMultisig, "CCIP"))

	proposers, executors, _, _ := timelockRoles(t.Context(), t, chain, timelock)
	require.Equal(t, []string{proposer.Hex()}, proposers)
	require.Equal(t, []string{callProxy.Hex()}, executors)
}

// TestExtraArgsDisablesCallProxy disables the call proxy for a timelock via ExtraArgs.
func TestExtraArgsDisablesCallProxy(t *testing.T) {
	t.Parallel()

	sel := chain_selectors.TEST_90000001.Selector
	rt := newEVMRuntime(t, sel)
	env := rt.Environment()
	chain := env.BlockChains.EVMChains()[sel]
	cfg := mcmConfig(t)

	input := deploycustomtopology.Input{Cfg: deploycustomtopology.Config{
		ChainConfigs: map[uint64]deploycustomtopology.ChainTopologyConfig{
			sel: {
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
				ExtraArgs: map[string]any{
					"deployCallProxyByTimelockRef": map[string]bool{"timelock": false},
				},
			},
		},
	}}

	out, err := deploycustomtopology.Changeset{}.Apply(env, input)
	require.NoError(t, err)

	require.Empty(t, fetchAddrs(t, out.DataStore, sel, mcmscontracts.CallProxy, "CCIP"))

	timelock := fetchAddr(t, out.DataStore, sel, mcmscontracts.RBACTimelock, "CCIP")
	executors, err := mcmsevmsdk.NewTimelockInspector(chain.Client).GetExecutors(t.Context(), timelock.Hex())
	require.NoError(t, err)
	require.Empty(t, executors)
}

// TestDeployTwoTimelocksWithOwnershipTransfer deploys two independent stacks
// (CCIP + RMN) on one chain, each taking ownership of its bypasser MCM, and
// asserts one accept-ownership proposal is built per timelock qualifier.
func TestDeployTwoTimelocksWithOwnershipTransfer(t *testing.T) {
	t.Parallel()

	sel := chain_selectors.TEST_90000001.Selector
	rt := newEVMRuntime(t, sel)
	env := rt.Environment()
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

	// Merge the two stacks into one chain config.
	ccip := stack("CCIP")
	rmn := stack("RMN")
	merged := deploycustomtopology.ChainTopologyConfig{
		MCMs:      append(append([]deploycustomtopology.MCMSpec{}, ccip.MCMs...), rmn.MCMs...),
		Timelocks: append(append([]deploycustomtopology.TimelockSpec{}, ccip.Timelocks...), rmn.Timelocks...),
	}

	input := deploycustomtopology.Input{
		Cfg:  deploycustomtopology.Config{ChainConfigs: map[uint64]deploycustomtopology.ChainTopologyConfig{sel: merged}},
		MCMS: newMCMSInput(),
	}

	require.NoError(t, deploycustomtopology.Changeset{}.VerifyPreconditions(env, input))
	out, err := deploycustomtopology.Changeset{}.Apply(env, input)
	require.NoError(t, err)

	// One proposal per timelock qualifier.
	require.Len(t, out.MCMSTimelockProposals, 2)

	ccipTimelock := fetchAddr(t, out.DataStore, sel, mcmscontracts.RBACTimelock, "CCIP")
	rmnTimelock := fetchAddr(t, out.DataStore, sel, mcmscontracts.RBACTimelock, "RMN")
	ccipBypasser := fetchAddr(t, out.DataStore, sel, mcmscontracts.BypasserManyChainMultisig, "CCIP")
	rmnBypasser := fetchAddr(t, out.DataStore, sel, mcmscontracts.BypasserManyChainMultisig, "RMN")

	gotTimelocks := map[string]bool{}
	gotAcceptTargets := map[string]bool{}
	for _, p := range out.MCMSTimelockProposals {
		for _, addr := range p.TimelockAddresses {
			gotTimelocks[common.HexToAddress(addr).Hex()] = true
		}
		for _, batch := range p.Operations {
			for _, tx := range batch.Transactions {
				gotAcceptTargets[common.HexToAddress(tx.To).Hex()] = true
			}
		}
	}
	require.Equal(t, map[string]bool{ccipTimelock.Hex(): true, rmnTimelock.Hex(): true}, gotTimelocks)
	require.Equal(t, map[string]bool{ccipBypasser.Hex(): true, rmnBypasser.Hex(): true}, gotAcceptTargets)
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

func fetchAddrs(t *testing.T, ds cldfdatastore.MutableDataStore, sel uint64, ct cldf.ContractType, qualifier string) []common.Address {
	t.Helper()

	refs, err := ds.Addresses().Fetch()
	require.NoError(t, err)

	var addrs []common.Address
	for _, r := range refs {
		if r.ChainSelector == sel && r.Type == cldfdatastore.ContractType(ct) && r.Qualifier == qualifier {
			addrs = append(addrs, common.HexToAddress(r.Address))
		}
	}

	return addrs
}

func fetchAddr(t *testing.T, ds cldfdatastore.MutableDataStore, sel uint64, ct cldf.ContractType, qualifier string) common.Address {
	t.Helper()

	addrs := fetchAddrs(t, ds, sel, ct, qualifier)
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

func newMCMSInput() *cldf.MCMSTimelockProposalInput {
	return &cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
		Description:    "test",
	}
}
