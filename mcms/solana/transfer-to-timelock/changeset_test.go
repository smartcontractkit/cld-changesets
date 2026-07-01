package soltransfertotimelock_test

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	mcmBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/mcm"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/internal/testutil/solanatest"
	solstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	mcmsdeploy "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
	pdasol "github.com/smartcontractkit/cld-changesets/pkg/family/solana"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/deploy"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/transfer-to-timelock"
)

// TestChangeset shares one Solana container across the transfer and idempotent subtests to
// avoid paying the ~30 s container-startup cost multiple times. OnlyAcceptOwnership keeps its
// own environment because it requires a pending-ownership state that conflicts with the other
// flows.
//
//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func TestChangeset(t *testing.T) {
	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector

	t.Run("OnlyAcceptOwnership", func(t *testing.T) { //nolint:paralleltest
		rt, chain, mcmsState := newTransferToTimelockTestEnv(t, selector)

		timelockSignerPDA := pdasol.GetTimelockSignerPDA(mcmsState.TimelockProgram, mcmsState.TimelockSeed)
		proposerConfigPDA := pdasol.GetMCMConfigPDA(mcmsState.McmProgram, mcmsState.ProposerMcmSeed)
		deployer := chain.DeployerKey.PublicKey()

		require.NoError(t, transferProposerMCMOnChain(t, chain, mcmsState, timelockSignerPDA))

		assertMCMOwner(t, deployer, proposerConfigPDA, chain)

		err := rt.Exec(
			runtime.ChangesetTask(transfertotimelock.Changeset{}, acceptOwnershipInput(selector)),
		)
		require.NoError(t, err)

		err = rt.Exec(runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}))
		require.NoError(t, err)
		require.Len(t, rt.State().Proposals, 1)
		require.True(t, rt.State().Proposals[0].IsExecuted)

		assertMCMOwner(t, timelockSignerPDA, proposerConfigPDA, chain)
	})

	rt, chain, mcmsState := newTransferToTimelockTestEnv(t, selector)
	timelockSignerPDA := pdasol.GetTimelockSignerPDA(mcmsState.TimelockProgram, mcmsState.TimelockSeed)
	proposerConfigPDA := pdasol.GetMCMConfigPDA(mcmsState.McmProgram, mcmsState.ProposerMcmSeed)
	transferInput := transferToTimelockInput(selector)

	t.Run("TransferOwnershipToTimelock", func(t *testing.T) { //nolint:paralleltest
		deployer := chain.DeployerKey.PublicKey()
		assertMCMOwner(t, deployer, proposerConfigPDA, chain)

		execTransferToTimelock(t, rt, transferInput)
		require.Len(t, rt.State().Proposals, 1)
		require.True(t, rt.State().Proposals[0].IsExecuted)

		assertMCMOwner(t, timelockSignerPDA, proposerConfigPDA, chain)
	})

	t.Run("IdempotentWhenAlreadyOwnedByTimelock", func(t *testing.T) { //nolint:paralleltest
		ensureProposerOwnedByTimelock(t, rt, chain, timelockSignerPDA, proposerConfigPDA, transferInput)

		taskID, err := runtime.ExecChangeset(rt, transfertotimelock.Changeset{}, transferInput)
		require.NoError(t, err)

		output, ok := rt.State().Outputs[taskID]
		require.True(t, ok)
		require.Empty(t, output.MCMSTimelockProposals, "expected no proposal when contract already owned by timelock")
	})
}

func transferToTimelockInput(selector uint64) transfertotimelock.Input {
	return transfertotimelock.Input{
		Cfg: transfertotimelock.Config{
			ContractsByChain: map[uint64][]refkey.RefKey{
				selector: {contractRef(selector, mcmscontracts.ProposerManyChainMultisig, "")},
			},
		},
		MCMS: &cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			Description:    "Transfer proposer MCM ownership to timelock",
			TimelockDelay:  mcmstypes.NewDuration(time.Second),
		},
	}
}

func acceptOwnershipInput(selector uint64) transfertotimelock.Input {
	return transfertotimelock.Input{
		Cfg: transfertotimelock.Config{
			OnlyAcceptOwnership: true,
			ContractsByChain: map[uint64][]refkey.RefKey{
				selector: {contractRef(selector, mcmscontracts.ProposerManyChainMultisig, "")},
			},
		},
		MCMS: &cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			Description:    "Accept proposer MCM ownership on timelock",
			TimelockDelay:  mcmstypes.NewDuration(time.Second),
		},
	}
}

func execTransferToTimelock(t *testing.T, rt *runtime.Runtime, input transfertotimelock.Input) {
	t.Helper()

	err := rt.Exec(
		runtime.ChangesetTask(transfertotimelock.Changeset{}, input),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)
}

func ensureProposerOwnedByTimelock(
	t *testing.T,
	rt *runtime.Runtime,
	chain cldfsol.Chain,
	timelockSignerPDA, proposerConfigPDA solana.PublicKey,
	input transfertotimelock.Input,
) {
	t.Helper()

	if mcmOwner(t, proposerConfigPDA, chain) == timelockSignerPDA {
		return
	}

	execTransferToTimelock(t, rt, input)
	assertMCMOwner(t, timelockSignerPDA, proposerConfigPDA, chain)
}

func transferProposerMCMOnChain(
	t *testing.T,
	chain cldfsol.Chain,
	mcmsState *solstate.MCMSWithTimelockState,
	timelockSignerPDA solana.PublicKey,
) error {
	t.Helper()

	configPDA := pdasol.GetMCMConfigPDA(mcmsState.McmProgram, mcmsState.ProposerMcmSeed)
	mcmBindings.SetProgramID(mcmsState.McmProgram)

	ix, err := mcmBindings.NewTransferOwnershipInstruction(
		mcmsState.ProposerMcmSeed,
		timelockSignerPDA,
		configPDA,
		chain.DeployerKey.PublicKey(),
	).ValidateAndBuild()
	if err != nil {
		return err
	}

	return chain.Confirm([]solana.Instruction{&seededInstruction{Instruction: ix, programID: mcmsState.McmProgram}})
}

type seededInstruction struct {
	*mcmBindings.Instruction
	programID solana.PublicKey
}

func (s *seededInstruction) ProgramID() solana.PublicKey {
	return s.programID
}

func newTransferToTimelockTestEnv(
	t *testing.T,
	selector uint64,
) (*runtime.Runtime, cldfsol.Chain, *solstate.MCMSWithTimelockState) {
	t.Helper()

	programsPath, programIDs, _ := soltestutils.PreloadMCMS(t, selector)
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{selector}, programsPath, programIDs),
		environment.WithDatastore(solanatest.NewDataStoreWithMCMSPrograms(t, selector)),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	chain := rt.Environment().BlockChains.SolanaChains()[selector]

	err = rt.Exec(
		runtime.ChangesetTask(mcmsdeploy.Changeset{}, mcmsdeploy.Input{
			ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
				selector: cldftesthelpers.SingleGroupTimelockConfig(t),
			},
		}),
	)
	require.NoError(t, err)

	refs := rt.Environment().DataStore.Addresses().Filter(cldfdatastore.AddressRefByChainSelector(selector))
	mcmsState, err := solstate.MaybeLoadMCMSWithTimelockChainStateV2(refs)
	require.NoError(t, err)
	soltestutils.FundSignerPDAs(t, chain, mcmsState)

	return rt, chain, mcmsState
}

func contractRef(chainSelector uint64, contractType cldf.ContractType, qualifier string) refkey.RefKey {
	return refkey.New(chainSelector, cldfdatastore.ContractType(contractType), &semvers.V1_0_0, qualifier)
}

func mcmOwner(t *testing.T, configPDA solana.PublicKey, chain cldfsol.Chain) solana.PublicKey {
	t.Helper()

	var config mcmBindings.MultisigConfig
	err := chain.GetAccountDataBorshInto(t.Context(), configPDA, &config)
	require.NoError(t, err)

	return config.Owner
}

func assertMCMOwner(
	t *testing.T,
	want solana.PublicKey,
	configPDA solana.PublicKey,
	chain cldfsol.Chain,
) {
	t.Helper()

	require.Equal(t, want, mcmOwner(t, configPDA, chain))
}
