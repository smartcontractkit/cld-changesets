package solgrantrole

import (
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	solstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

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
	addValidateRef(t, ds, selector, mcmscontracts.AccessControllerProgram, programID, version)
	env := validateTestEnv(t, ds.Seal(), selector)

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
	addValidateRef(t, ds, selector, mcmscontracts.AccessControllerProgram, programID.String(), version)
	addValidateRef(t, ds, selector, mcmscontracts.ExecutorAccessControllerAccount, accountID.String(), version)
	env := validateTestEnv(t, ds.Seal(), selector)

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
	addValidateRef(t, ds, selector, mcmscontracts.RBACTimelock, timelockAddr, version)
	env := validateTestEnv(t, ds.Seal(), selector)

	got, err := timelockContractAddress(env, grantrole.SeqInput{ChainSelector: selector})
	require.NoError(t, err)
	require.Equal(t, timelockAddr, got)

	_, err = timelockContractAddress(validateTestEnv(t, datastore.NewMemoryDataStore().Seal(), selector), grantrole.SeqInput{ChainSelector: selector})
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
	addValidateRef(t, ds, selector, mcmscontracts.RBACTimelock, timelockAddr, version)
	env := validateTestEnv(t, ds.Seal(), selector)

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
