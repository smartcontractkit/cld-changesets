package deploy_test

import (
	"context"
	"testing"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	timelockBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/timelock"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	"github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"

	// Import Solana deploy package to auto-register the Solana family via init().
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/deploy"
)

//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func TestDeployMCMSWithTimelock_SolanaFreshDeploy(t *testing.T) {
	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	rt := newSolanaDeployRuntime(t, selector)

	cfg := cldftesthelpers.SingleGroupTimelockConfig(t)
	err := rt.Exec(runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
		ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cfg,
		},
	}))
	require.NoError(t, err)

	assertSolanaDeployState(t, rt, selector, cfg)
}

//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func TestDeployMCMSWithTimelock_SolanaIdempotentRedeploy(t *testing.T) {
	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	rt := newSolanaDeployRuntime(t, selector)

	cfg := cldftesthelpers.SingleGroupTimelockConfig(t)
	task := runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
		ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cfg,
		},
	})

	require.NoError(t, rt.Exec(task))

	refsAfterFirst, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.NotEmpty(t, refsAfterFirst)

	require.NoError(t, rt.Exec(task))

	refsAfterSecond, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refsAfterSecond, len(refsAfterFirst))

	assertSolanaDeployState(t, rt, selector, cfg)
}

func newSolanaDeployRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	programsPath, programIDs, _ := soltestutils.PreloadMCMS(t, selector)

	ds := datastore.NewMemoryDataStore()
	for _, program := range []struct {
		name string
		typ  datastore.ContractType
	}{
		{solutils.ProgMCM, datastore.ContractType(mcmscontracts.ManyChainMultisigProgram)},
		{solutils.ProgAccessController, datastore.ContractType(mcmscontracts.AccessControllerProgram)},
		{solutils.ProgTimelock, datastore.ContractType(mcmscontracts.RBACTimelockProgram)},
	} {
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			ChainSelector: selector,
			Address:       programIDs[program.name],
			Type:          program.typ,
			Version:       &semvers.V1_0_0,
		}))
	}

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{selector}, programsPath, programIDs),
		environment.WithDatastore(ds.Seal()),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	return rt
}

func assertSolanaDeployState(
	t *testing.T,
	rt *runtime.Runtime,
	selector uint64,
	cfg cldfproposalutils.MCMSWithTimelockConfig,
) {
	t.Helper()

	allRefs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.NotEmpty(t, allRefs)

	chain := rt.Environment().BlockChains.SolanaChains()[selector]
	chainRefs := rt.State().DataStore.Addresses().Filter(datastore.AddressRefByChainSelector(selector))
	state, err := legacysolana.MaybeLoadMCMSWithTimelockChainStateV2(chainRefs)
	require.NoError(t, err)
	require.NoError(t, state.Validate())

	assertSolanaMCMSDeploy(t, t.Context(), chain, state, cfg)
}

func assertSolanaMCMSDeploy(
	t *testing.T,
	ctx context.Context,
	chain cldfsol.Chain,
	state *legacysolana.MCMSWithTimelockState,
	cfg cldfproposalutils.MCMSWithTimelockConfig,
) {
	t.Helper()

	inspector := mcmssolanasdk.NewInspector(chain.Client)
	timelockInspector := mcmssolanasdk.NewTimelockInspector(chain.Client)

	proposerAddr := mcmssolanasdk.ContractAddress(state.McmProgram, mcmssolanasdk.PDASeed(state.ProposerMcmSeed))
	proposerConfig, err := inspector.GetConfig(ctx, proposerAddr)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(*proposerConfig, cfg.Proposer))

	cancellerAddr := mcmssolanasdk.ContractAddress(state.McmProgram, mcmssolanasdk.PDASeed(state.CancellerMcmSeed))
	cancellerConfig, err := inspector.GetConfig(ctx, cancellerAddr)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(*cancellerConfig, cfg.Canceller))

	bypasserAddr := mcmssolanasdk.ContractAddress(state.McmProgram, mcmssolanasdk.PDASeed(state.BypasserMcmSeed))
	bypasserConfig, err := inspector.GetConfig(ctx, bypasserAddr)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(*bypasserConfig, cfg.Bypasser))

	timelockAddr := mcmssolanasdk.ContractAddress(state.TimelockProgram, mcmssolanasdk.PDASeed(state.TimelockSeed))

	proposers, err := timelockInspector.GetProposers(ctx, timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{mcmSignerPDA(state.McmProgram, state.ProposerMcmSeed)}, proposers)

	executors, err := timelockInspector.GetExecutors(ctx, timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{chain.DeployerKey.PublicKey().String()}, executors)

	cancellers, err := timelockInspector.GetCancellers(ctx, timelockAddr)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		mcmSignerPDA(state.McmProgram, state.CancellerMcmSeed),
		mcmSignerPDA(state.McmProgram, state.ProposerMcmSeed),
		mcmSignerPDA(state.McmProgram, state.BypasserMcmSeed),
	}, cancellers)

	bypassers, err := timelockInspector.GetBypassers(ctx, timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{mcmSignerPDA(state.McmProgram, state.BypasserMcmSeed)}, bypassers)

	timelockConfig := solanaTimelockConfig(ctx, t, chain, state.TimelockProgram, state.TimelockSeed)
	require.Equal(t, "11111111111111111111111111111111", timelockConfig.ProposedOwner.String())
}

func mcmSignerPDA(program solanago.PublicKey, seed legacysolana.PDASeed) string {
	return familysolana.GetMCMSignerPDA(program, seed).String()
}

func solanaTimelockConfig(
	ctx context.Context,
	t *testing.T,
	chain cldfsol.Chain,
	timelockProgram solanago.PublicKey,
	timelockSeed legacysolana.PDASeed,
) timelockBindings.Config {
	t.Helper()

	var config timelockBindings.Config
	err := chain.GetAccountDataBorshInto(ctx, familysolana.GetTimelockConfigPDA(timelockProgram, timelockSeed), &config)
	require.NoError(t, err)

	return config
}
