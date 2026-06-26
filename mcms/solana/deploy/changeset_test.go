package soldeploy_test

import (
	"testing"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	solutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	solreaders "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/deploy"
)

// All Solana integration tests share global mcm/timelock/ac SetProgramID state;
// they must run sequentially to avoid races.

type solanaMCMSTimelockRefs struct {
	Bypasser  string
	Canceller string
	Proposer  string
	Timelock  string
}

// datastoreWithMCMSPrograms seeds the datastore with canonical MCMS program IDs.
// The test validator preloads the same programs via WithSolanaContainer; program
// deploy is skipped because artifacts lack -keypair.json files required by
// solana program deploy for fixed program IDs.
func datastoreWithMCMSPrograms(t *testing.T, selector uint64) datastore.DataStore {
	t.Helper()

	v := semvers.V1_0_0
	ds := datastore.NewMemoryDataStore()
	for _, entry := range []struct {
		addr string
		ct   datastore.ContractType
	}{
		{solutils.GetProgramID(solutils.ProgAccessController), datastore.ContractType(mcmscontracts.AccessControllerProgram)},
		{solutils.GetProgramID(solutils.ProgMCM), datastore.ContractType(mcmscontracts.ManyChainMultisigProgram)},
		{solutils.GetProgramID(solutils.ProgTimelock), datastore.ContractType(mcmscontracts.RBACTimelockProgram)},
	} {
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			ChainSelector: selector,
			Address:       entry.addr,
			Type:          entry.ct,
			Version:       &v,
		}))
	}

	return ds.Seal()
}

func newSolanaDeployRuntime(t *testing.T, selector uint64, ds datastore.DataStore) *runtime.Runtime {
	t.Helper()

	programsPath, programIDs, _ := soltestutils.PreloadMCMS(t, selector)
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{selector}, programsPath, programIDs),
		environment.WithDatastore(ds),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	return rt
}

func execSolanaDeployChangeset(
	t *testing.T,
	rt *runtime.Runtime,
	selector uint64,
	cfg cldfproposalutils.MCMSWithTimelockConfig,
) {
	t.Helper()

	err := rt.Exec(runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
		ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cfg,
		},
	}))
	require.NoError(t, err)
}

func loadSolanaMCMSTimelockRefs(t *testing.T, env cldf.Environment, selector uint64) solanaMCMSTimelockRefs {
	t.Helper()

	reader := solreaders.Reader{}

	timelockRef, err := reader.GetTimelockRef(env, selector, cldf.MCMSTimelockProposalInput{})
	require.NoError(t, err)

	proposerRef, err := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
	})
	require.NoError(t, err)

	cancellerRef, err := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionCancel,
	})
	require.NoError(t, err)

	bypasserRef, err := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionBypass,
	})
	require.NoError(t, err)

	return solanaMCMSTimelockRefs{
		Bypasser:  bypasserRef.Address,
		Canceller: cancellerRef.Address,
		Proposer:  proposerRef.Address,
		Timelock:  timelockRef.Address,
	}
}

func assertSolanaDeployDatastoreRefs(t *testing.T, rt *runtime.Runtime, selector uint64) map[datastore.ContractType]datastore.AddressRef {
	t.Helper()

	refs, err := rt.State().DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, refs, 11, "expected 11 MCMS contract address refs")

	byType := make(map[datastore.ContractType]datastore.AddressRef, 11)
	for _, ref := range refs {
		require.Equal(t, selector, ref.ChainSelector)
		require.True(t, semvers.V1_0_0.Equal(ref.Version))
		byType[ref.Type] = ref
	}

	for _, ct := range []datastore.ContractType{
		datastore.ContractType(mcmscontracts.AccessControllerProgram),
		datastore.ContractType(mcmscontracts.ProposerAccessControllerAccount),
		datastore.ContractType(mcmscontracts.ExecutorAccessControllerAccount),
		datastore.ContractType(mcmscontracts.CancellerAccessControllerAccount),
		datastore.ContractType(mcmscontracts.BypasserAccessControllerAccount),
		datastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
		datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		datastore.ContractType(mcmscontracts.CancellerManyChainMultisig),
		datastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
		datastore.ContractType(mcmscontracts.RBACTimelockProgram),
		datastore.ContractType(mcmscontracts.RBACTimelock),
	} {
		require.Contains(t, byType, ct)
	}

	return byType
}

func assertSolanaMCMConfig(t *testing.T, chain cldfsol.Chain, address string, want mcmstypes.Config) {
	t.Helper()

	got, err := mcmssolana.NewInspector(chain.Client).GetConfig(t.Context(), address)
	require.NoError(t, err)
	require.ElementsMatch(t, want.Signers, got.Signers)
	require.Equal(t, want.Quorum, got.Quorum)
}

func assertSolanaTimelockRoles(t *testing.T, chain cldfsol.Chain, timelockAddr string, refs solanaMCMSTimelockRefs) {
	t.Helper()

	mcmProgram, proposerSeed, err := mcmssolana.ParseContractAddress(refs.Proposer)
	require.NoError(t, err)
	_, cancellerSeed, err := mcmssolana.ParseContractAddress(refs.Canceller)
	require.NoError(t, err)
	_, bypasserSeed, err := mcmssolana.ParseContractAddress(refs.Bypasser)
	require.NoError(t, err)

	proposerPDA := familysolana.GetMCMSignerPDA(mcmProgram, legacysolana.PDASeed(proposerSeed))
	cancellerPDA := familysolana.GetMCMSignerPDA(mcmProgram, legacysolana.PDASeed(cancellerSeed))
	bypasserPDA := familysolana.GetMCMSignerPDA(mcmProgram, legacysolana.PDASeed(bypasserSeed))
	deployer := chain.DeployerKey.PublicKey().String()

	inspector := mcmssolana.NewTimelockInspector(chain.Client)

	proposers, err := inspector.GetProposers(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{proposerPDA.String()}, proposers)

	executors, err := inspector.GetExecutors(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{deployer}, executors)

	cancellers, err := inspector.GetCancellers(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		cancellerPDA.String(),
		proposerPDA.String(),
		bypasserPDA.String(),
	}, cancellers)

	bypassers, err := inspector.GetBypassers(t.Context(), timelockAddr)
	require.NoError(t, err)
	require.Equal(t, []string{bypasserPDA.String()}, bypassers)
}

func assertSolanaDeployOnChain(
	t *testing.T,
	rt *runtime.Runtime,
	selector uint64,
	cfg cldfproposalutils.MCMSWithTimelockConfig,
) {
	t.Helper()

	env := rt.Environment()
	chain := env.BlockChains.SolanaChains()[selector]
	refs := loadSolanaMCMSTimelockRefs(t, env, selector)

	assertSolanaTimelockRoles(t, chain, refs.Timelock, refs)
	assertSolanaMCMConfig(t, chain, refs.Proposer, cfg.Proposer)
	assertSolanaMCMConfig(t, chain, refs.Canceller, cfg.Canceller)
	assertSolanaMCMConfig(t, chain, refs.Bypasser, cfg.Bypasser)

	minDelay, err := mcmssolana.NewTimelockInspector(chain.Client).GetMinDelay(t.Context(), refs.Timelock)
	require.NoError(t, err)
	require.Equal(t, uint64(0), minDelay)
}

//nolint:paralleltest // global SetProgramID binding state is shared in-process
func TestDeployMCMSWithTimelock_Solana_FreshDeploy(t *testing.T) {
	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	cfg := cldftesthelpers.SingleGroupTimelockConfig(t)

	rt := newSolanaDeployRuntime(t, selector, datastoreWithMCMSPrograms(t, selector))
	execSolanaDeployChangeset(t, rt, selector, cfg)

	assertSolanaDeployDatastoreRefs(t, rt, selector)
	assertSolanaDeployOnChain(t, rt, selector, cfg)
}

//nolint:paralleltest // global SetProgramID binding state is shared in-process
func TestDeployMCMSWithTimelock_Solana_PartialDeploy(t *testing.T) {
	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	cfg := cldftesthelpers.SingleGroupTimelockConfig(t)

	rt := newSolanaDeployRuntime(t, selector, datastoreWithMCMSPrograms(t, selector))
	execSolanaDeployChangeset(t, rt, selector, cfg)

	byType := assertSolanaDeployDatastoreRefs(t, rt, selector)

	// Pre-existing programs must retain their canonical IDs (not be re-deployed).
	require.Equal(t, solutils.GetProgramID(solutils.ProgAccessController),
		byType[datastore.ContractType(mcmscontracts.AccessControllerProgram)].Address)
	require.Equal(t, solutils.GetProgramID(solutils.ProgMCM),
		byType[datastore.ContractType(mcmscontracts.ManyChainMultisigProgram)].Address)
	require.Equal(t, solutils.GetProgramID(solutils.ProgTimelock),
		byType[datastore.ContractType(mcmscontracts.RBACTimelockProgram)].Address)

	assertSolanaDeployOnChain(t, rt, selector, cfg)
}

//nolint:paralleltest // global SetProgramID binding state is shared in-process
func TestDeployMCMSWithTimelock_Solana_IdempotentReRun(t *testing.T) {
	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	cfg := cldftesthelpers.SingleGroupTimelockConfig(t)

	rt := newSolanaDeployRuntime(t, selector, datastoreWithMCMSPrograms(t, selector))

	execSolanaDeployChangeset(t, rt, selector, cfg)
	afterFirst := assertSolanaDeployDatastoreRefs(t, rt, selector)
	assertSolanaDeployOnChain(t, rt, selector, cfg)

	execSolanaDeployChangeset(t, rt, selector, cfg)
	afterSecond := assertSolanaDeployDatastoreRefs(t, rt, selector)

	for ct, ref := range afterFirst {
		require.Equal(t, ref.Address, afterSecond[ct].Address, "address changed for %s on re-run", ct)
	}
}
