package solsetconfig_test

import (
	"context"
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/internal/testutil/solanatest"

	solstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	mcmsdeploy "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"

	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config/all"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/deploy"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

func TestChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_22222222222222222222222222222222222222222222.Selector
	env := newSolanaVerifyPreconditionsEnv(t, selector)

	validCfg := cldftesthelpers.SingleGroupMCMS(t)
	validTargets := mcmsTargets(selector, validCfg, validCfg, validCfg)

	cs := setconfig.Changeset{}
	for _, tt := range []struct {
		name  string
		input setconfig.Input
	}{
		{name: "valid direct-send input", input: setConfigInput(validTargets, nil)},
		{name: "valid MCMS input", input: setConfigInput(validTargets, newMCMSInput(mcmstypes.TimelockActionSchedule, "valid solana proposal", ""))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, cs.VerifyPreconditions(env, tt.input))
		})
	}
}

//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func TestChangeset(t *testing.T) {
	selector := chain_selectors.TEST_22222222222222222222222222222222222222222222.Selector
	rt := newSolanaSetConfigRuntime(t, selector)
	chain := rt.Environment().BlockChains.SolanaChains()[selector]

	refs := rt.Environment().DataStore.Addresses().Filter(cldfdatastore.AddressRefByChainSelector(selector))
	mcmsState, err := solstate.MaybeLoadMCMSWithTimelockChainStateV2(refs)
	require.NoError(t, err)
	soltestutils.FundSignerPDAs(t, chain, mcmsState)

	inspector := mcmssolana.NewInspector(chain.Client)
	_, signer1Addr := createSolSigner(t)

	newCfgProposer := cldftesthelpers.SingleGroupMCMS(t)
	newCfgProposer.Signers = append(newCfgProposer.Signers, signer1Addr)
	newCfgProposer.Quorum = 2
	newCfgCanceller := cldftesthelpers.SingleGroupMCMS(t)
	newCfgBypasser := cldftesthelpers.SingleGroupMCMS(t)
	newCfgBypasser.Signers = append(newCfgBypasser.Signers, signer1Addr)
	newCfgBypasser.Quorum = 2

	t.Run("direct send", func(t *testing.T) { //nolint:paralleltest // shared runtime state
		err = rt.Exec(
			runtime.ChangesetTask(setconfig.Changeset{}, setConfigInput(
				mcmsTargets(selector, newCfgProposer, newCfgCanceller, newCfgBypasser),
				nil,
			)),
		)
		require.NoError(t, err)

		assertSolConfigEquals(t, inspector, mcmsState.McmProgram, mcmsState.ProposerMcmSeed, newCfgProposer)
		assertSolConfigEquals(t, inspector, mcmsState.McmProgram, mcmsState.BypasserMcmSeed, newCfgBypasser)
		assertSolConfigEquals(t, inspector, mcmsState.McmProgram, mcmsState.CancellerMcmSeed, newCfgCanceller)
	})

	t.Run("builds MCMS proposal without execute", func(t *testing.T) { //nolint:paralleltest // shared runtime state
		cfg := cldftesthelpers.SingleGroupMCMS(t)
		taskID, err := runtime.ExecChangeset(rt, setconfig.Changeset{}, setConfigInput(
			mcmsTargets(selector, cfg, cfg, cfg),
			newMCMSInput(mcmstypes.TimelockActionBypass, "proposal only", ""),
		))
		require.NoError(t, err)

		output, ok := rt.State().Outputs[taskID]
		require.True(t, ok)
		require.Len(t, output.MCMSTimelockProposals, 1)
		solanatest.AssertMCMSSetConfigProposal(
			t, selector, mcmsState, output.MCMSTimelockProposals[0],
			mcmstypes.TimelockActionBypass, mcmstypes.NewDuration(0), "proposal only",
		)
	})
}

func newSolanaVerifyPreconditionsEnv(t *testing.T, selector uint64) cldf.Environment {
	t.Helper()

	ds := cldfdatastore.NewMemoryDataStore()
	version := semver.MustParse("1.0.0")
	for _, ref := range []struct {
		contractType cldf.ContractType
		address      string
	}{
		{mcmscontracts.RBACTimelock, "timelock-address"},
		{mcmscontracts.ProposerManyChainMultisig, "proposer-address"},
		{mcmscontracts.CancellerManyChainMultisig, "canceller-address"},
		{mcmscontracts.BypasserManyChainMultisig, "bypasser-address"},
	} {
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			Address:       ref.address,
			ChainSelector: selector,
			Type:          cldfdatastore.ContractType(ref.contractType),
			Version:       version,
		}))
	}

	return cldf.Environment{
		Logger:    logger.Test(t),
		DataStore: ds.Seal(),
		GetContext: func() context.Context {
			return t.Context()
		},
		BlockChains: cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
	}
}

func newSolanaSetConfigRuntime(t *testing.T, selector uint64) *runtime.Runtime {
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
		runtime.ChangesetTask(mcmsdeploy.Changeset{}, mcmsdeploy.Input{
			ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
				selector: cldftesthelpers.SingleGroupTimelockConfig(t),
			},
		}),
	)
	require.NoError(t, err)

	return rt
}

func assertSolConfigEquals(
	t *testing.T, inspector *mcmssolana.Inspector, programID solanago.PublicKey, seed solstate.PDASeed, want mcmstypes.Config,
) {
	t.Helper()

	cfg, err := inspector.GetConfig(t.Context(), mcmssolana.ContractAddress(programID, mcmssolana.PDASeed(seed)))
	require.NoError(t, err)
	require.ElementsMatch(t, want.Signers, cfg.Signers)
	require.Equal(t, want.Quorum, cfg.Quorum)
}

func createSolSigner(t *testing.T) (*ecdsa.PrivateKey, common.Address) {
	t.Helper()

	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	publicKey := key.Public().(*ecdsa.PublicKey)

	return key, crypto.PubkeyToAddress(*publicKey)
}

func contractRef(chainSelector uint64, contractType cldf.ContractType, qualifier string) refkey.RefKey {
	return refkey.New(chainSelector, cldfdatastore.ContractType(contractType), &semvers.V1_0_0, qualifier)
}

func mcmsTargets(
	chainSelector uint64,
	proposer, canceller, bypasser mcmstypes.Config,
) []setconfig.ContractSetConfig {
	return []setconfig.ContractSetConfig{
		{Ref: contractRef(chainSelector, mcmscontracts.ProposerManyChainMultisig, ""), Config: proposer},
		{Ref: contractRef(chainSelector, mcmscontracts.CancellerManyChainMultisig, ""), Config: canceller},
		{Ref: contractRef(chainSelector, mcmscontracts.BypasserManyChainMultisig, ""), Config: bypasser},
	}
}

func newMCMSInput(action mcmstypes.TimelockAction, description, qualifier string) *cldf.MCMSTimelockProposalInput {
	delay := mcmstypes.NewDuration(time.Second)
	if action == mcmstypes.TimelockActionBypass {
		delay = mcmstypes.NewDuration(0)
	}

	return &cldf.MCMSTimelockProposalInput{
		TimelockAction: action,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  delay,
		Qualifier:      qualifier,
		Description:    description,
	}
}

func setConfigInput(targets []setconfig.ContractSetConfig, mcms *cldf.MCMSTimelockProposalInput) setconfig.Input {
	return setconfig.Input{
		Cfg:  setconfig.Config{Targets: targets},
		MCMS: mcms,
	}
}
