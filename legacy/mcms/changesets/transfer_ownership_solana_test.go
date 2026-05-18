package changesets_test

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/quarantine"
	"github.com/stretchr/testify/require"

	mcmschangesets "github.com/smartcontractkit/cld-changesets/legacy/mcms/changesets"
	solstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	pdasol "github.com/smartcontractkit/cld-changesets/pkg/family/solana"

	accessControllerBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/access_controller"
	mcmBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/mcm"
	timelockBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/timelock"

	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
)

func setupTransferOwnershipSolanaTest(t *testing.T) (*runtime.Runtime, uint64) {
	t.Helper()
	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	programsPath, programIDs, ab := soltestutils.PreloadMCMS(t, selector)
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{selector}, programsPath, programIDs),
		environment.WithAddressBook(ab),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(mcmschangesets.DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	return rt, selector
}

func TestTransferToMCMSToTimelockSolana(t *testing.T) {
	t.Parallel()

	quarantine.Flaky(t, "DX-1773")

	rt, selector := setupTransferOwnershipSolanaTest(t)

	addresses, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)

	chain := rt.Environment().BlockChains.SolanaChains()[selector]

	mcmsState, err := solstate.MaybeLoadMCMSWithTimelockChainState(chain, addresses)
	require.NoError(t, err)

	soltestutils.FundSignerPDAs(t, chain, mcmsState)

	deployer := rt.Environment().BlockChains.SolanaChains()[selector].DeployerKey.PublicKey()
	assertTransferOwnershipSolanaOwner(t, chain, mcmsState, deployer)

	err = rt.Exec(
		runtime.ChangesetTask(&mcmschangesets.TransferMCMSToTimelockSolana{}, mcmschangesets.TransferMCMSToTimelockSolanaConfig{
			Chains:  []uint64{selector},
			MCMSCfg: cldfproposalutils.TimelockConfig{MinDelay: 1 * time.Second},
		}),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)

	timelockSignerPDA := pdasol.GetTimelockSignerPDA(mcmsState.TimelockProgram, mcmsState.TimelockSeed)
	assertTransferOwnershipSolanaOwner(t, chain, mcmsState, timelockSignerPDA)
}

func assertTransferOwnershipSolanaOwner(
	t *testing.T, chain cldf_solana.Chain, mcmsState *solstate.MCMSWithTimelockState, owner solana.PublicKey,
) {
	t.Helper()

	assertMCMOwner(t, owner, pdasol.GetMCMConfigPDA(mcmsState.McmProgram, mcmsState.ProposerMcmSeed), chain)
	assertMCMOwner(t, owner, pdasol.GetMCMConfigPDA(mcmsState.McmProgram, mcmsState.CancellerMcmSeed), chain)
	assertMCMOwner(t, owner, pdasol.GetMCMConfigPDA(mcmsState.McmProgram, mcmsState.BypasserMcmSeed), chain)
	assertTimelockOwner(t, owner, pdasol.GetTimelockConfigPDA(mcmsState.TimelockProgram, mcmsState.TimelockSeed), chain)
	assertAccessControllerOwner(t, owner, mcmsState.ProposerAccessControllerAccount, chain)
	assertAccessControllerOwner(t, owner, mcmsState.ExecutorAccessControllerAccount, chain)
	assertAccessControllerOwner(t, owner, mcmsState.CancellerAccessControllerAccount, chain)
	assertAccessControllerOwner(t, owner, mcmsState.BypasserAccessControllerAccount, chain)
}

func assertMCMOwner(
	t *testing.T, want solana.PublicKey, configPDA solana.PublicKey, chain cldf_solana.Chain,
) {
	t.Helper()

	var config mcmBindings.MultisigConfig
	err := chain.GetAccountDataBorshInto(t.Context(), configPDA, &config)
	require.NoError(t, err)
	require.Equal(t, want, config.Owner)
}

func assertTimelockOwner(
	t *testing.T, want solana.PublicKey, configPDA solana.PublicKey, chain cldf_solana.Chain,
) {
	t.Helper()

	var config timelockBindings.Config
	err := chain.GetAccountDataBorshInto(t.Context(), configPDA, &config)
	require.NoError(t, err)
	require.Equal(t, want, config.Owner)
}

func assertAccessControllerOwner(
	t *testing.T, want solana.PublicKey, account solana.PublicKey, chain cldf_solana.Chain,
) {
	t.Helper()

	var config accessControllerBindings.AccessController
	err := chain.GetAccountDataBorshInto(t.Context(), account, &config)
	require.NoError(t, err)
	require.Equal(t, want, config.Owner)
}
