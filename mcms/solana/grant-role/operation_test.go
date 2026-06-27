package solgrantrole

import (
	"crypto/ecdsa"
	"fmt"
	"testing"
	"time"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
)

func TestOpSolanaGrantRole(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	wallet := solanago.NewWallet()
	deployerKey := wallet.PrivateKey
	grantee := solanago.NewWallet().PublicKey().String()

	t.Run("missing deployer key", func(t *testing.T) {
		t.Parallel()

		_, err := operations.ExecuteOperation(
			optest.NewBundle(t),
			OpSolanaGrantRole,
			cldfsol.Chain{Selector: selector},
			OpSolanaGrantRoleInput{
				Target: GrantRoleTarget{
					Timelock: "timelock",
					Role:     mcmssdk.TimelockRoleExecutor,
					Address:  grantee,
				},
			},
		)
		require.EqualError(t, err, fmt.Sprintf("missing deployer key for chain %d", selector))
	})

	t.Run("missing rpc client", func(t *testing.T) {
		t.Parallel()

		_, err := operations.ExecuteOperation(
			optest.NewBundle(t),
			OpSolanaGrantRole,
			cldfsol.Chain{Selector: selector, DeployerKey: &deployerKey},
			OpSolanaGrantRoleInput{
				Target: GrantRoleTarget{
					Timelock: "not-a-valid-timelock",
					Role:     mcmssdk.TimelockRoleExecutor,
					Address:  grantee,
				},
			},
		)
		require.EqualError(t, err, fmt.Sprintf("missing rpc client for chain %d", selector))
	})

	t.Run("admin role rejected", func(t *testing.T) {
		t.Parallel()

		_, err := operations.ExecuteOperation(
			optest.NewBundle(t),
			OpSolanaGrantRole,
			cldfsol.Chain{Selector: selector, DeployerKey: &deployerKey},
			OpSolanaGrantRoleInput{
				Target: GrantRoleTarget{
					Timelock: "timelock",
					Role:     mcmssdk.TimelockRoleAdmin,
					Address:  grantee,
				},
			},
		)
		require.EqualError(t, err, "admin role not supported on solana")
	})
}

func TestValidateGrantRoleTarget(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateGrantRoleTarget(GrantRoleTarget{
		Timelock: "timelock",
		Role:     mcmssdk.TimelockRoleExecutor,
		Address:  solanago.NewWallet().PublicKey().String(),
	}))

	require.EqualError(t, validateGrantRoleTarget(GrantRoleTarget{
		Role:    mcmssdk.TimelockRoleExecutor,
		Address: "addr",
	}), "timelock address must not be empty")
}

//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func TestSolanaGrantRole(t *testing.T) {
	t.Run("sequence", testRunSolanaGrantRole)
	t.Run("operation", testOpSolanaGrantRole)
}

func testOpSolanaGrantRole(t *testing.T) {
	tests := []struct {
		name   string
		noSend bool
	}{
		{name: "direct send", noSend: false},
		{name: "MCMS proposal", noSend: true},
	}

	for _, tt := range tests { //nolint:paralleltest // global mcm.SetProgramID state
		t.Run(tt.name, func(t *testing.T) {
			selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
			rt := newSolanaGrantRoleRuntime(t, selector)
			chain := rt.Environment().BlockChains.SolanaChains()[selector]
			env := rt.Environment()
			timelock := timelockRefAddress(t, env, selector)
			fundSolanaGrantRolePDAs(t, rt, selector, chain)

			mcmsInput := &cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionSchedule,
				ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
				TimelockDelay:  mcmstypes.NewDuration(time.Second),
			}

			opInput := OpSolanaGrantRoleInput{
				Target: GrantRoleTarget{
					Timelock: timelock,
					Role:     mcmssdk.TimelockRoleExecutor,
					Address:  solanago.NewWallet().PublicKey().String(),
				},
				NoSend: tt.noSend,
			}
			if tt.noSend {
				transferSolanaMCMSToTimelock(t, rt, selector)
				fundSolanaGrantRolePDAs(t, rt, selector, chain)
				var err error
				opInput.AuthorityAccount, err = timelockSignerPDA(env, grantrole.SeqInput{
					ChainSelector: selector,
					MCMS:          mcmsInput,
				})
				require.NoError(t, err)
			}

			report, err := operations.ExecuteOperation(
				rt.Environment().OperationsBundle,
				OpSolanaGrantRole,
				chain,
				opInput,
			)
			require.NoError(t, err)
			require.Equal(t, !tt.noSend, report.Output.Confirmed)

			if tt.noSend {
				require.Equal(t, mcmstypes.ChainSelector(selector), report.Output.BatchOperation.ChainSelector)
				require.NotEmpty(t, report.Output.BatchOperation.Transactions)
				require.NoError(t, rt.Exec(
					newTimelockProposalTask([]mcmstypes.BatchOperation{report.Output.BatchOperation}, "solana grant role operation test"),
					runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
				))
			}

			executors, err := mcmssolana.NewTimelockInspector(chain.Client).GetExecutors(t.Context(), timelock)
			require.NoError(t, err)
			require.Contains(t, executors, opInput.Target.Address)
		})
	}
}
