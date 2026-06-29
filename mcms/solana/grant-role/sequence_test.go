package solgrantrole

import (
	"crypto/ecdsa"
	"fmt"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	legacymcms "github.com/smartcontractkit/cld-changesets/legacy/mcms/changesets"
	solstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	solchangesets "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/changesets"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
	solreaders "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func TestRunSolanaGrantRole(t *testing.T) {
	tests := []struct {
		name    string
		useMCMS bool
	}{
		{name: "direct send", useMCMS: false},
		{name: "MCMS proposal", useMCMS: true},
	}

	for _, tt := range tests { //nolint:paralleltest // global mcm.SetProgramID state
		t.Run(tt.name, func(t *testing.T) {
			selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
			rt := newSolanaGrantRoleRuntime(t, selector)
			chain := rt.Environment().BlockChains.SolanaChains()[selector]
			timelock := timelockRefAddress(t, rt.Environment(), selector)
			fundSolanaGrantRolePDAs(t, rt, selector, chain)

			var mcmsInput *cldf.MCMSTimelockProposalInput
			if tt.useMCMS {
				transferSolanaMCMSToTimelock(t, rt, selector)
				fundSolanaGrantRolePDAs(t, rt, selector, chain)
				mcmsInput = &cldf.MCMSTimelockProposalInput{
					TimelockAction: mcmstypes.TimelockActionSchedule,
					ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
					TimelockDelay:  mcmstypes.NewDuration(time.Second),
				}
			}

			grantee := solanago.NewWallet().PublicKey().String()
			out, err := runSolanaGrantRole(
				rt.Environment().OperationsBundle,
				grantrole.Deps{
					BlockChains: rt.Environment().BlockChains,
					DataStore:   rt.Environment().DataStore,
				},
				grantrole.SeqInput{
					ChainSelector: selector,
					Grants: []grantrole.RoleGrant{{
						Role:      mcmssdk.TimelockRoleExecutor,
						Addresses: []string{grantee},
					}},
					MCMS: mcmsInput,
				},
			)
			require.NoError(t, err)

			if tt.useMCMS {
				require.Len(t, out.BatchOps, 1)
				require.NotEmpty(t, out.BatchOps[0].Transactions)
				require.NoError(t, rt.Exec(
					newTimelockProposalTask(out.BatchOps, "solana grant role sequence test"),
					runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
				))
			} else {
				require.Empty(t, out.BatchOps)
			}

			executors, err := mcmssolana.NewTimelockInspector(chain.Client).GetExecutors(t.Context(), timelock)
			require.NoError(t, err)
			require.Contains(t, executors, grantee)
		})
	}
}

func TestRunSolanaGrantRole_idempotent(t *testing.T) {
	//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	rt := newSolanaGrantRoleRuntime(t, selector)
	chain := rt.Environment().BlockChains.SolanaChains()[selector]
	timelock := timelockRefAddress(t, rt.Environment(), selector)
	fundSolanaGrantRolePDAs(t, rt, selector, chain)

	grantee := solanago.NewWallet().PublicKey().String()
	deps := grantrole.Deps{
		BlockChains: rt.Environment().BlockChains,
		DataStore:   rt.Environment().DataStore,
	}
	input := grantrole.SeqInput{
		ChainSelector: selector,
		Grants: []grantrole.RoleGrant{{
			Role:      mcmssdk.TimelockRoleProposer,
			Addresses: []string{grantee},
		}},
	}

	_, err := runSolanaGrantRole(rt.Environment().OperationsBundle, deps, input)
	require.NoError(t, err)

	out, err := runSolanaGrantRole(rt.Environment().OperationsBundle, deps, input)
	require.NoError(t, err)
	require.Empty(t, out.BatchOps)

	proposers, err := mcmssolana.NewTimelockInspector(chain.Client).GetProposers(t.Context(), timelock)
	require.NoError(t, err)
	require.Contains(t, proposers, grantee)
}

func TestRunSolanaGrantRole_errors(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	grantee := solanago.NewWallet().PublicKey().String()

	_, err := runSolanaGrantRole(
		optest.NewBundle(t),
		grantrole.Deps{BlockChains: chain.NewBlockChains(nil)},
		grantrole.SeqInput{ChainSelector: selector},
	)
	require.EqualError(t, err, fmt.Sprintf("solana chain %d not found in environment", selector))

	deps := grantrole.Deps{
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
		DataStore: datastore.NewMemoryDataStore().Seal(),
	}
	_, err = runSolanaGrantRole(
		optest.NewBundle(t),
		deps,
		grantrole.SeqInput{
			ChainSelector: selector,
			Grants: []grantrole.RoleGrant{{
				Role:      mcmssdk.TimelockRoleExecutor,
				Addresses: []string{grantee},
			}},
		},
	)
	require.EqualError(t, err, fmt.Sprintf(
		"resolve timelock for chain %d: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %d, Type: RBACTimelock}, found 0",
		selector, selector,
	))
}

func TestProgramRef(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	programID := solanago.NewWallet().PublicKey().String()
	version := semver.MustParse("1.0.0")

	ds := datastore.NewMemoryDataStore()
	addValidateRef(t, ds, selector, mcmscontracts.AccessControllerProgram, programID, version, "")
	env := validateTestEnv(ds.Seal(), selector)

	got, err := programRef(env, selector, mcmscontracts.AccessControllerProgram)
	require.NoError(t, err)
	require.Equal(t, programID, got)

	_, err = programRef(cldf.Environment{}, selector, mcmscontracts.AccessControllerProgram)
	require.EqualError(t, err, fmt.Sprintf("datastore not available for chain %d", selector))

	_, err = programRef(env, selector, mcmscontracts.ManyChainMultisigProgram)
	require.EqualError(t, err, fmt.Sprintf(
		"resolve ManyChainMultiSigProgram for chain %d: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %d, Type: ManyChainMultiSigProgram}, found 0",
		selector, selector,
	))
}

func TestAccessControllerProgramAndAccount(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	programID := solanago.NewWallet().PublicKey()
	accountID := solanago.NewWallet().PublicKey()
	version := semver.MustParse("1.0.0")

	ds := datastore.NewMemoryDataStore()
	addValidateRef(t, ds, selector, mcmscontracts.AccessControllerProgram, programID.String(), version, "")
	addValidateRef(t, ds, selector, mcmscontracts.ExecutorAccessControllerAccount, accountID.String(), version, "")
	env := validateTestEnv(ds.Seal(), selector)

	gotProgram, err := accessControllerProgram(env, selector)
	require.NoError(t, err)
	require.Equal(t, programID, gotProgram)

	gotAccount, err := accessControllerAccount(env, selector, mcmssdk.TimelockRoleExecutor)
	require.NoError(t, err)
	require.Equal(t, accountID, gotAccount)
}

func TestTimelockContractAddress(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	timelockProgram := solanago.NewWallet().PublicKey()
	timelockAddr := mcmssolana.ContractAddress(timelockProgram, testPDASeed(4))
	version := semver.MustParse("1.0.0")

	ds := datastore.NewMemoryDataStore()
	addValidateRef(t, ds, selector, mcmscontracts.RBACTimelock, timelockAddr, version, "")
	env := validateTestEnv(ds.Seal(), selector)

	got, err := timelockContractAddress(env, grantrole.SeqInput{ChainSelector: selector})
	require.NoError(t, err)
	require.Equal(t, timelockAddr, got)

	_, err = timelockContractAddress(validateTestEnv(datastore.NewMemoryDataStore().Seal(), selector), grantrole.SeqInput{ChainSelector: selector})
	require.EqualError(t, err, fmt.Sprintf(
		"resolve timelock for chain %d: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %d, Type: RBACTimelock}, found 0",
		selector, selector,
	))
}

func TestTimelockSignerPDA(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	timelockProgram := solanago.NewWallet().PublicKey()
	timelockSeed := testPDASeed(4)
	timelockAddr := mcmssolana.ContractAddress(timelockProgram, timelockSeed)
	version := semver.MustParse("1.0.0")

	ds := datastore.NewMemoryDataStore()
	addValidateRef(t, ds, selector, mcmscontracts.RBACTimelock, timelockAddr, version, "")
	env := validateTestEnv(ds.Seal(), selector)

	mcmsInput := cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     1,
		TimelockDelay:  mcmstypes.NewDuration(0),
	}
	got, err := timelockSignerPDA(env, grantrole.SeqInput{ChainSelector: selector, MCMS: &mcmsInput})
	require.NoError(t, err)

	parsedProgram, parsedSeed, err := mcmssolana.ParseContractAddress(timelockAddr)
	require.NoError(t, err)
	var legacySeed solstate.PDASeed
	copy(legacySeed[:], parsedSeed[:])
	require.Equal(t, familysolana.GetTimelockSignerPDA(parsedProgram, legacySeed), got)
}

func testPDASeed(v byte) mcmssolana.PDASeed {
	var seed mcmssolana.PDASeed
	seed[31] = v

	return seed
}

func newSolanaGrantRoleRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	programsPath, programIDs, ab := soltestutils.PreloadMCMS(t, selector)
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{selector}, programsPath, programIDs),
		environment.WithAddressBook(ab),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)
	require.Contains(t, rt.Environment().BlockChains.SolanaChains(), selector)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(legacymcms.DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	return rt
}

func timelockRefAddress(t *testing.T, env cldf.Environment, selector uint64) string {
	t.Helper()

	reader := solreaders.Reader{}
	ref, err := reader.GetTimelockRef(env, selector, cldf.MCMSTimelockProposalInput{})
	require.NoError(t, err)

	return ref.Address
}

func fundSolanaGrantRolePDAs(t *testing.T, rt *runtime.Runtime, selector uint64, chain cldfsol.Chain) {
	t.Helper()

	addrs, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)
	mcmsState, err := solstate.MaybeLoadMCMSWithTimelockChainState(chain, addrs)
	require.NoError(t, err)
	soltestutils.FundSignerPDAs(t, chain, mcmsState)
}

func transferSolanaMCMSToTimelock(t *testing.T, rt *runtime.Runtime, selector uint64) {
	t.Helper()

	err := rt.Exec(
		runtime.ChangesetTask(solchangesets.TransferMCMSToTimelockSolana{}, solchangesets.TransferMCMSToTimelockSolanaConfig{
			Chains:  []uint64{selector},
			MCMSCfg: cldfproposalutils.TimelockConfig{MinDelay: time.Second},
		}),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)
}

type timelockProposalTask struct {
	id          string
	batchOps    []mcmstypes.BatchOperation
	description string
}

func newTimelockProposalTask(batchOps []mcmstypes.BatchOperation, description string) timelockProposalTask {
	return timelockProposalTask{
		id:          ksuid.New().String(),
		batchOps:    batchOps,
		description: description,
	}
}

func (t timelockProposalTask) ID() string {
	return t.id
}

func (t timelockProposalTask) Run(e cldf.Environment, state *runtime.State) error {
	out, err := cldf.NewOutputBuilder(e, datastore.NewMemoryDataStore()).
		WithTimelockProposal(cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			TimelockDelay:  mcmstypes.NewDuration(time.Second),
			Description:    t.description,
		}, t.batchOps).
		Build()
	if err != nil {
		return err
	}

	return state.MergeChangesetOutput(t.id, out)
}
