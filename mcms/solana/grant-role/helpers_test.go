package solgrantrole

import (
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

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

func TestProgramRef(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	programID := solanago.NewWallet().PublicKey().String()

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       programID,
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.AccessControllerProgram),
		Version:       semver.MustParse("1.0.0"),
	}))
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

	ds := datastore.NewMemoryDataStore()
	version := semver.MustParse("1.0.0")
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       programID.String(),
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.AccessControllerProgram),
		Version:       version,
	}))
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       accountID.String(),
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ExecutorAccessControllerAccount),
		Version:       version,
	}))
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

	ds := datastore.NewMemoryDataStore()
	addHelperRef(t, ds, selector, mcmscontracts.RBACTimelock, timelockAddr)
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

	ds := datastore.NewMemoryDataStore()
	addHelperRef(t, ds, selector, mcmscontracts.RBACTimelock, timelockAddr)
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
	var legacySeed legacysolana.PDASeed
	copy(legacySeed[:], parsedSeed[:])
	require.Equal(t, familysolana.GetTimelockSignerPDA(parsedProgram, legacySeed), got)
}

func addHelperRef(t *testing.T, ds *datastore.MemoryDataStore, selector uint64, contractType cldf.ContractType, address string) {
	t.Helper()

	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       address,
		ChainSelector: selector,
		Type:          datastore.ContractType(contractType),
		Version:       semver.MustParse("1.0.0"),
	}))
}

func testPDASeed(v byte) mcmssolana.PDASeed {
	var seed mcmssolana.PDASeed
	seed[31] = v

	return seed
}
