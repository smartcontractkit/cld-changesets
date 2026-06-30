package solgrantrole

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

type validateRefSpec struct {
	contractType cldf.ContractType
	address      string
}

func TestValidateMCMSIfPresent(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	version := semver.MustParse("1.0.0")
	mcmsInput := cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
	}

	tests := []struct {
		name    string
		refs    []validateRefSpec
		mcms    *cldf.MCMSTimelockProposalInput
		wantErr string
	}{
		{name: "nil MCMS"},
		{
			name: "missing timelock ref",
			mcms: &mcmsInput,
			wantErr: fmt.Sprintf(
				"validate timelock ref for chain %d: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %d, Type: RBACTimelock}, found 0",
				selector, selector,
			),
		},
		{
			name: "missing mcms ref",
			refs: []validateRefSpec{{mcmscontracts.RBACTimelock, "timelock"}},
			mcms: &mcmsInput,
			wantErr: fmt.Sprintf(
				"validate mcms ref for chain %d: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %d, Type: ProposerManyChainMultiSig}, found 0",
				selector, selector,
			),
		},
		{
			name: "success",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, "timelock"},
				{mcmscontracts.ProposerManyChainMultisig, "proposer"},
			},
			mcms: &mcmsInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ds := datastore.NewMemoryDataStore()
			for _, ref := range tt.refs {
				addValidateRef(t, ds, selector, ref.contractType, ref.address, version)
			}

			err := validateMCMSIfPresent(
				validateTestEnv(ds.Seal(), selector),
				grantrole.SeqInput{ChainSelector: selector, MCMS: tt.mcms},
			)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestValidateRoles(t *testing.T) {
	t.Parallel()

	err := validateRoles(grantrole.SeqInput{
		Grants: []grantrole.RoleGrant{{Role: mcmssdk.TimelockRole(99), Addresses: []string{"11111111111111111111111111111111"}}},
	})
	require.EqualError(t, err, "grants[0]: unsupported timelock role Unknown")

	err = validateRoles(grantrole.SeqInput{
		Grants: []grantrole.RoleGrant{{Role: mcmssdk.TimelockRoleAdmin, Addresses: []string{"11111111111111111111111111111111"}}},
	})
	require.EqualError(t, err, "grants[0]: admin role not supported on solana")
}

func TestAccessControllerContractType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role    mcmssdk.TimelockRole
		want    cldf.ContractType
		wantErr string
	}{
		{role: mcmssdk.TimelockRoleProposer, want: mcmscontracts.ProposerAccessControllerAccount},
		{role: mcmssdk.TimelockRoleExecutor, want: mcmscontracts.ExecutorAccessControllerAccount},
		{role: mcmssdk.TimelockRoleCanceller, want: mcmscontracts.CancellerAccessControllerAccount},
		{role: mcmssdk.TimelockRoleBypasser, want: mcmscontracts.BypasserAccessControllerAccount},
		{role: mcmssdk.TimelockRoleAdmin, wantErr: "admin role not supported on solana"},
		{role: mcmssdk.TimelockRole(99), wantErr: "unsupported timelock role Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.role.String(), func(t *testing.T) {
			t.Parallel()

			got, err := accessControllerContractType(tt.role)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseSolanaAddress(t *testing.T) {
	t.Parallel()

	valid := "11111111111111111111111111111112"
	addr, err := parseSolanaAddress(valid)
	require.NoError(t, err)
	require.Equal(t, valid, addr.String())

	_, err = parseSolanaAddress("not-a-pubkey")
	require.EqualError(t, err, `address "not-a-pubkey" is not a valid solana address: decode: invalid base58 digit ('-')`)
}

func TestValidateGrantAddresses(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	version := semver.MustParse("1.0.0")
	grantee := "11111111111111111111111111111112"

	t.Run("missing access controller ref", func(t *testing.T) {
		t.Parallel()

		err := validateGrantAddresses(
			validateTestEnv(datastore.NewMemoryDataStore().Seal(), selector),
			grantrole.SeqInput{
				ChainSelector: selector,
				Grants: []grantrole.RoleGrant{{
					Role:      mcmssdk.TimelockRoleExecutor,
					Addresses: []string{grantee},
				}},
			},
		)
		require.EqualError(t, err, fmt.Sprintf(
			"grants[0]: error fetching address in datastore for ExecutorAccessControllerAccount in chain %d: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %d, Type: ExecutorAccessControllerAccount}, found 0",
			selector, selector,
		))
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ds := datastore.NewMemoryDataStore()
		addValidateRef(t, ds, selector, mcmscontracts.ExecutorAccessControllerAccount, "executor-ac", version)

		err := validateGrantAddresses(
			validateTestEnv(ds.Seal(), selector),
			grantrole.SeqInput{
				ChainSelector: selector,
				Grants: []grantrole.RoleGrant{{
					Role:      mcmssdk.TimelockRoleExecutor,
					Addresses: []string{grantee},
				}},
			},
		)
		require.NoError(t, err)
	})
}

func addValidateRef(
	t *testing.T,
	ds *datastore.MemoryDataStore,
	selector uint64,
	contractType cldf.ContractType,
	address string,
	version *semver.Version,
) {
	t.Helper()

	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       address,
		ChainSelector: selector,
		Type:          datastore.ContractType(contractType),
		Version:       version,
		Qualifier:     "",
	}))
}

func validateTestEnv(ds datastore.DataStore, selector uint64) cldf.Environment {
	return cldf.Environment{
		Logger:     logger.Nop(),
		DataStore:  ds,
		GetContext: context.Background,
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
	}
}
