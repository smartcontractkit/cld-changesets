package solsetconfig

import (
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func TestSolanaSetConfig(t *testing.T) {
	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector

	// Direct-send cases must not mutate MCMS quorum before transfer runs in the MCMS group.
	t.Run("direct send", func(t *testing.T) { //nolint:paralleltest // shared runtime state
		rt := newSolanaSetConfigRuntime(t, selector)
		chain := rt.Environment().BlockChains.SolanaChains()[selector]
		refs := solanaSetConfigRefs(t, rt.Environment(), selector)
		fundSolanaSignerPDAs(t, chain, refs)

		t.Run("operation", func(t *testing.T) { //nolint:paralleltest // shared runtime state
			testOpSolanaSetConfigDirectSend(t, rt, chain, refs)
		})
		t.Run("sequence", func(t *testing.T) { //nolint:paralleltest // shared runtime state
			testRunSolanaSetConfigDirectSend(t, rt, chain, refs, selector)
		})
	})

	// MCMS cases share one fresh deploy; transfer runs once before any timelock proposals.
	t.Run("MCMS proposal", func(t *testing.T) { //nolint:paralleltest // shared runtime state
		rt := newSolanaSetConfigRuntime(t, selector)
		chain := rt.Environment().BlockChains.SolanaChains()[selector]
		refs := solanaSetConfigRefs(t, rt.Environment(), selector)
		fundSolanaSignerPDAs(t, chain, refs)

		t.Run("operation", func(t *testing.T) { //nolint:paralleltest // shared runtime state
			testOpSolanaSetConfigMCMSProposal(t, rt, chain, refs, selector)
		})
		t.Run("sequence", func(t *testing.T) { //nolint:paralleltest // shared runtime state
			testRunSolanaSetConfigMCMSProposal(t, rt, chain, refs, selector)
		})
	})
}

func testOpSolanaSetConfigDirectSend(t *testing.T, rt *runtime.Runtime, chain cldfsol.Chain, refs solanaMCMSRefs) {
	t.Helper()

	cfg := cldftesthelpers.SingleGroupMCMS(t)
	cfg.Signers = append(cfg.Signers, common.HexToAddress("0x0000000000000000000000000000000000000909"))
	cfg.Quorum = 2

	report, err := operations.ExecuteOperation(
		rt.Environment().OperationsBundle,
		OpSolanaSetConfigMCM,
		chain,
		OpSolanaSetConfigInput{
			Target: MCMSetConfigTarget{
				Address:      refs.Canceller,
				Config:       cfg,
				ContractType: string(mcmscontracts.CancellerManyChainMultisig),
			},
			NoSend: false,
		},
	)
	require.NoError(t, err)
	require.True(t, report.Output.Confirmed)
	assertSolanaConfigEquals(t, mcmssolana.NewInspector(chain.Client), refs.Canceller, cfg)
}

func testOpSolanaSetConfigMCMSProposal(t *testing.T, rt *runtime.Runtime, chain cldfsol.Chain, refs solanaMCMSRefs, selector uint64) {
	t.Helper()

	transferSolanaMCMSToTimelock(t, rt, selector)
	fundSolanaSignerPDAs(t, chain, refs)

	cfg := cldftesthelpers.SingleGroupMCMS(t)
	cfg.Signers = append(cfg.Signers, common.HexToAddress("0x0000000000000000000000000000000000000909"))
	cfg.Quorum = 2

	report, err := operations.ExecuteOperation(
		rt.Environment().OperationsBundle,
		OpSolanaSetConfigMCM,
		chain,
		OpSolanaSetConfigInput{
			Target: MCMSetConfigTarget{
				Address:      refs.Canceller,
				Config:       cfg,
				ContractType: string(mcmscontracts.CancellerManyChainMultisig),
			},
			NoSend:           true,
			AuthorityAccount: refs.TimelockSigner,
		},
	)
	require.NoError(t, err)
	require.False(t, report.Output.Confirmed)
	require.Equal(t, mcmstypes.ChainSelector(selector), report.Output.BatchOperation.ChainSelector)
	require.NotEmpty(t, report.Output.BatchOperation.Transactions)
	require.NoError(t, rt.Exec(
		newTimelockProposalTask([]mcmstypes.BatchOperation{report.Output.BatchOperation}, "solana set config operation test"),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	))
	assertSolanaConfigEquals(t, mcmssolana.NewInspector(chain.Client), refs.Canceller, cfg)
}
