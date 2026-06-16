package solsetconfig

import (
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func TestSolanaSetConfig(t *testing.T) {
	t.Run("operation", testOpSolanaSetConfigMCM)
	t.Run("sequence", testSeqSolanaSetConfig)
}

func testOpSolanaSetConfigMCM(t *testing.T) {
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
			rt := newSolanaSetConfigRuntime(t, selector)
			chain := rt.Environment().BlockChains.SolanaChains()[selector]
			refs := solanaSetConfigRefs(t, rt.Environment(), selector)
			fundSolanaSignerPDAs(t, chain, refs)

			authorityAccount := solanago.PublicKey{}
			if tt.noSend {
				transferSolanaMCMSToTimelock(t, rt, selector)
				fundSolanaSignerPDAs(t, chain, refs)
				authorityAccount = refs.TimelockSigner
			}

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
					NoSend:           tt.noSend,
					AuthorityAccount: authorityAccount,
				},
			)
			require.NoError(t, err)
			require.Equal(t, !tt.noSend, report.Output.Confirmed)

			if tt.noSend {
				require.Equal(t, mcmstypes.ChainSelector(selector), report.Output.BatchOperation.ChainSelector)
				require.NotEmpty(t, report.Output.BatchOperation.Transactions)
				require.NoError(t, rt.Exec(
					newTimelockProposalTask([]mcmstypes.BatchOperation{report.Output.BatchOperation}, "solana set config operation test"),
					runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
				))
			}

			assertSolanaConfigEquals(t, mcmssolana.NewInspector(chain.Client), refs.Canceller, cfg)
		})
	}
}
