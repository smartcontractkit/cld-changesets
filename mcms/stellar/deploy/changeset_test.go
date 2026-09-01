package stellardeploy_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"

	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"

	mcmsstellar "github.com/smartcontractkit/mcms/sdk/stellar"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	stellartestutils "github.com/smartcontractkit/cld-changesets/mcms/stellar/internal"

	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/deploy"
)

func TestChangeset_Stellar(t *testing.T) {
	selector := chainselectors.STELLAR_LOCALNET.Selector
	rt := stellartestutils.NewStellarRuntime(t, selector)

	chain, ok := rt.Environment().BlockChains.StellarChains()[selector]
	require.True(t, ok)

	deployer, err := stellardeployment.NewDeployerFromChain(chain)
	require.NoError(t, err)

	inspector := mcmsstellar.NewInspectorFromInvoker(deployer)
	timelockInspector := mcmsstellar.NewTimelockInspectorFromInvoker(deployer)

	// Per-role configs are deliberately distinct (same pattern as the EVM
	// deploy test) so assertStellarDeployment detects a swapped
	// proposer/canceller/bypasser pairing; identical configs would be
	// swap-blind.
	cfg := distinctRoleTimelockConfig(t)
	cfg.TimelockMinDelay = big.NewInt(7)

	t.Run("fresh deploy", func(t *testing.T) {
		stellartestutils.DeployMCMSWithTimelock(t, rt, selector, cfg)

		refs := stellarMCMSRefs(t, rt.Environment(), selector, "")
		assertStellarDeployment(
			t,
			inspector,
			timelockInspector,
			refs,
			cfg,
			deployer.SignerAddress(),
			7,
		)
	})

	t.Run("idempotent rerun", func(t *testing.T) {
		before := stellarMCMSRefs(t, rt.Environment(), selector, "")

		stellartestutils.DeployMCMSWithTimelock(t, rt, selector, cfg)

		after := stellarMCMSRefs(t, rt.Environment(), selector, "")
		require.Equal(t, before, after)

		assertStellarDeployment(
			t,
			inspector,
			timelockInspector,
			after,
			cfg,
			deployer.SignerAddress(),
			7,
		)
	})

	t.Run("qualifier creates independent deployment", func(t *testing.T) {
		original := stellarMCMSRefs(t, rt.Environment(), selector, "")

		qualifier := "qualified"
		qualifiedCfg := cfg
		qualifiedCfg.Qualifier = &qualifier

		stellartestutils.DeployMCMSWithTimelock(
			t,
			rt,
			selector,
			qualifiedCfg,
		)

		qualified := stellarMCMSRefs(
			t,
			rt.Environment(),
			selector,
			qualifier,
		)

		require.NotEqual(t, original.Proposer, qualified.Proposer)
		require.NotEqual(t, original.Canceller, qualified.Canceller)
		require.NotEqual(t, original.Bypasser, qualified.Bypasser)
		require.NotEqual(t, original.Timelock, qualified.Timelock)

		assertStellarDeployment(
			t,
			inspector,
			timelockInspector,
			qualified,
			qualifiedCfg,
			deployer.SignerAddress(),
			7,
		)
	})
}

// distinctRoleTimelockConfig mirrors the EVM deploy test's per-role config
// pattern: each MCM role gets a different single signer, so reading configs
// back per role fails on any pairwise swap.
func distinctRoleTimelockConfig(t *testing.T) cldfproposalutils.MCMSWithTimelockConfig {
	t.Helper()

	signer := func(addr string) mcmstypes.Config {
		cfg, err := mcmstypes.NewConfig(
			1,
			[]common.Address{common.HexToAddress(addr)},
			[]mcmstypes.Config{},
		)
		require.NoError(t, err)

		return cfg
	}

	return cldfproposalutils.MCMSWithTimelockConfig{
		Proposer:  signer("0x0000000000000000000000000000000000000001"),
		Canceller: signer("0x0000000000000000000000000000000000000002"),
		Bypasser:  signer("0x0000000000000000000000000000000000000003"),
	}
}

type stellarRefs struct {
	Proposer  string
	Canceller string
	Bypasser  string
	Timelock  string
}

func stellarMCMSRefs(
	t *testing.T,
	env cldf.Environment,
	selector uint64,
	qualifier string,
) stellarRefs {
	t.Helper()

	return stellarRefs{
		Proposer: stellartestutils.ResolveContract(
			t,
			env,
			selector,
			mcmscontracts.ProposerManyChainMultisig,
			qualifier,
		),
		Canceller: stellartestutils.ResolveContract(
			t,
			env,
			selector,
			mcmscontracts.CancellerManyChainMultisig,
			qualifier,
		),
		Bypasser: stellartestutils.ResolveContract(
			t,
			env,
			selector,
			mcmscontracts.BypasserManyChainMultisig,
			qualifier,
		),
		Timelock: stellartestutils.ResolveContract(
			t,
			env,
			selector,
			mcmscontracts.RBACTimelock,
			qualifier,
		),
	}
}

func assertStellarDeployment(
	t *testing.T,
	inspector *mcmsstellar.Inspector,
	timelockInspector *mcmsstellar.TimelockInspector,
	refs stellarRefs,
	cfg cldfproposalutils.MCMSWithTimelockConfig,
	deployerAddress string,
	wantMinDelay uint64,
) {
	t.Helper()

	require.NotEmpty(t, refs.Proposer)
	require.NotEmpty(t, refs.Canceller)
	require.NotEmpty(t, refs.Bypasser)
	require.NotEmpty(t, refs.Timelock)

	require.NotEqual(t, refs.Proposer, refs.Canceller)
	require.NotEqual(t, refs.Proposer, refs.Bypasser)
	require.NotEqual(t, refs.Canceller, refs.Bypasser)

	mcms := []struct {
		address string
		config  mcmstypes.Config
	}{
		{address: refs.Proposer, config: cfg.Proposer},
		{address: refs.Canceller, config: cfg.Canceller},
		{address: refs.Bypasser, config: cfg.Bypasser},
	}

	for _, mcm := range mcms {
		assertStellarConfigEquals(t, inspector, mcm.address, mcm.config)
		assertStellarOwnerEquals(t, inspector, mcm.address, deployerAddress)
	}

	assertStellarTimelockConfig(
		t,
		timelockInspector,
		refs,
		wantMinDelay,
	)
}

func assertStellarConfigEquals(
	t *testing.T,
	inspector *mcmsstellar.Inspector,
	address string,
	want mcmstypes.Config,
) {
	t.Helper()

	got, err := inspector.GetConfig(t.Context(), address)
	require.NoError(t, err)
	require.True(t, want.Equals(got))
}

func assertStellarOwnerEquals(
	t *testing.T,
	inspector *mcmsstellar.Inspector,
	address string,
	want string,
) {
	t.Helper()

	owner, err := inspector.GetOwner(t.Context(), address)
	require.NoError(t, err)
	require.NotNil(t, owner)
	require.Equal(t, want, *owner)

	pendingOwner, err := inspector.GetPendingOwner(t.Context(), address)
	require.NoError(t, err)
	require.Nil(t, pendingOwner)
}

func assertStellarTimelockConfig(
	t *testing.T,
	inspector *mcmsstellar.TimelockInspector,
	refs stellarRefs,
	wantMinDelay uint64,
) {
	t.Helper()

	proposers, err := inspector.GetProposers(t.Context(), refs.Timelock)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{refs.Proposer}, proposers)

	cancellers, err := inspector.GetCancellers(t.Context(), refs.Timelock)
	require.NoError(t, err)
	require.ElementsMatch(
		t,
		[]string{
			refs.Proposer,
			refs.Canceller,
			refs.Bypasser,
		},
		cancellers,
	)

	bypassers, err := inspector.GetBypassers(t.Context(), refs.Timelock)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{refs.Bypasser}, bypassers)

	minDelay, err := inspector.GetMinDelay(t.Context(), refs.Timelock)
	require.NoError(t, err)
	require.Equal(t, wantMinDelay, minDelay)
}
