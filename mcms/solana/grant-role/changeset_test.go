package solgrantrole_test

import (
	"crypto/ecdsa"
	"testing"
	"time"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

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
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/internal/testutil/solanatest"
	solstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	deploy "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/deploy"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/grant-role"
	solreaders "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/transfer-to-timelock"
)

// TestChangeset_GrantRole covers the direct-send and MCMS-proposal paths through the
// public grant-role changeset on a single shared container. The "direct send" case also
// re-runs the same grant to confirm the changeset is idempotent, so a second container
// isn't needed just to exercise that path.
//
//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func TestChangeset_GrantRole(t *testing.T) {
	tests := []struct {
		name    string
		useMCMS bool
		role    mcmssdk.TimelockRole
	}{
		{name: "direct send", useMCMS: false, role: mcmssdk.TimelockRoleProposer},
		{name: "MCMS proposal", useMCMS: true, role: mcmssdk.TimelockRoleExecutor},
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
					Description:    "grant role changeset test",
				}
			}

			grantee := solanago.NewWallet().PublicKey().String()
			input := grantrole.Input{
				Cfg: grantrole.Config{
					GrantsByChain: map[uint64][]grantrole.RoleGrant{
						selector: {{Role: tt.role, Addresses: []string{grantee}}},
					},
				},
				MCMS: mcmsInput,
			}

			taskID, err := runtime.ExecChangeset(rt, grantrole.Changeset{}, input)
			require.NoError(t, err)

			if tt.useMCMS {
				output, ok := rt.State().Outputs[taskID]
				require.True(t, ok)
				require.Len(t, output.MCMSTimelockProposals, 1)
				require.NotEmpty(t, output.MCMSTimelockProposals[0].Operations)
				require.NoError(t, rt.Exec(runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner})))
			} else {
				// Re-run the same grant to confirm the changeset is idempotent on direct send.
				_, err = runtime.ExecChangeset(rt, grantrole.Changeset{}, input)
				require.NoError(t, err)
			}

			holders, err := roleHolders(t, chain, timelock, tt.role)
			require.NoError(t, err)
			require.Contains(t, holders, grantee)
		})
	}
}

func roleHolders(t *testing.T, chain cldfsol.Chain, timelock string, role mcmssdk.TimelockRole) ([]string, error) {
	t.Helper()

	inspector := mcmssolana.NewTimelockInspector(chain.Client)
	switch role {
	case mcmssdk.TimelockRoleProposer:
		return inspector.GetProposers(t.Context(), timelock)
	case mcmssdk.TimelockRoleExecutor:
		return inspector.GetExecutors(t.Context(), timelock)
	case mcmssdk.TimelockRoleCanceller:
		return inspector.GetCancellers(t.Context(), timelock)
	case mcmssdk.TimelockRoleBypasser:
		return inspector.GetBypassers(t.Context(), timelock)
	case mcmssdk.TimelockRoleAdmin:
		t.Fatalf("unsupported role %s", role)

		return nil, nil
	default:
		t.Fatalf("unsupported role %s", role)

		return nil, nil
	}
}

func newSolanaGrantRoleRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	programsPath, programIDs, _ := soltestutils.PreloadMCMS(t, selector)
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{selector}, programsPath, programIDs),
		environment.WithDatastore(solanatest.NewDataStoreWithMCMSPrograms(t, selector)),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)
	require.Contains(t, rt.Environment().BlockChains.SolanaChains(), selector)

	err = rt.Exec(
		runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
			ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
				selector: cldftesthelpers.SingleGroupTimelockConfig(t),
			},
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

	refs := rt.Environment().DataStore.Addresses().Filter(datastore.AddressRefByChainSelector(selector))
	mcmsState, err := solstate.MaybeLoadMCMSWithTimelockChainStateV2(refs)
	require.NoError(t, err)
	soltestutils.FundSignerPDAs(t, chain, mcmsState)
}

func transferSolanaMCMSToTimelock(t *testing.T, rt *runtime.Runtime, selector uint64) {
	t.Helper()

	err := rt.Exec(
		runtime.ChangesetTask(transfertotimelock.Changeset{}, transfertotimelock.Input{
			Cfg: transfertotimelock.Config{
				ContractsByChain: map[uint64][]refkey.RefKey{
					selector: {
						grantRoleContractRef(selector, mcmscontracts.ProposerManyChainMultisig),
						grantRoleContractRef(selector, mcmscontracts.CancellerManyChainMultisig),
						grantRoleContractRef(selector, mcmscontracts.BypasserManyChainMultisig),
						grantRoleContractRef(selector, mcmscontracts.RBACTimelock),
						grantRoleContractRef(selector, mcmscontracts.ProposerAccessControllerAccount),
						grantRoleContractRef(selector, mcmscontracts.ExecutorAccessControllerAccount),
						grantRoleContractRef(selector, mcmscontracts.CancellerAccessControllerAccount),
						grantRoleContractRef(selector, mcmscontracts.BypasserAccessControllerAccount),
					},
				},
			},
			MCMS: &cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionSchedule,
				ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
				Description:    "Transfer MCMS ownership to timelock",
				TimelockDelay:  mcmstypes.NewDuration(time.Second),
			},
		}),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)
}

func grantRoleContractRef(chainSelector uint64, contractType cldf.ContractType) refkey.RefKey {
	return refkey.New(chainSelector, datastore.ContractType(contractType), &semvers.V1_0_0, "")
}
