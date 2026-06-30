package solgrantrole

import (
	"fmt"
	"testing"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
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
