package changesets

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	solCommonUtil "github.com/smartcontractkit/chainlink-ccip/chains/solana/utils/common"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/stretchr/testify/require"

	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
	solanastate "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

func TestFundMCMSignersChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	selector1 := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	selector2 := chainselectors.TEST_33333333333333333333333333333333333333333333.Selector

	tests := []struct {
		name          string
		env           func(t *testing.T) cldf.Environment
		config        FundMCMSignerConfig
		expectedError string
	}{
		{
			name: "All preconditions satisfied",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()
				return testFundMCMSignersEnv(t, selector1, rpcWithBalance(t, 1_000), true)
			},
			config: FundMCMSignerConfig{
				AmountsPerChain: map[uint64]AmountsToTransfer{selector1: {
					ProposeMCM:   100,
					CancellerMCM: 100,
					BypasserMCM:  100,
					Timelock:     100,
				}},
			},
			expectedError: "",
		},
		{
			name: "No Solana chains found in environment",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()

				return testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(nil))
			},
			config: FundMCMSignerConfig{
				AmountsPerChain: map[uint64]AmountsToTransfer{selector1: {
					ProposeMCM:   100,
					CancellerMCM: 100,
					BypasserMCM:  100,
					Timelock:     100,
				}},
			},
			expectedError: fmt.Sprintf("solana chain not found for selector %d", selector1),
		},
		{
			name: "Chain selector not found in environment",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()
				return testFundMCMSignersEnv(t, selector1, rpcWithBalance(t, 1_000), true)
			},
			config: FundMCMSignerConfig{AmountsPerChain: map[uint64]AmountsToTransfer{99999: {
				ProposeMCM:   100,
				CancellerMCM: 100,
				BypasserMCM:  100,
				Timelock:     100,
			}}},
			expectedError: "solana chain not found for selector 99999",
		},
		{
			name: "MCMS contracts not deployed (empty seeds)",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()
				return testFundMCMSignersEnv(t, selector2, rpcWithBalance(t, 1_000), false)
			},
			config: FundMCMSignerConfig{
				AmountsPerChain: map[uint64]AmountsToTransfer{selector2: {
					ProposeMCM:   100,
					CancellerMCM: 100,
					BypasserMCM:  100,
					Timelock:     100,
				}},
			},
			expectedError: "mcm/timelock seeds are empty, please deploy MCMS contracts first",
		},
		{
			name: "Insufficient deployer balance",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()
				return testFundMCMSignersEnv(t, selector1, rpcWithBalance(t, 1), true)
			},
			config: FundMCMSignerConfig{
				AmountsPerChain: map[uint64]AmountsToTransfer{selector1: {
					ProposeMCM:   100,
					CancellerMCM: 100,
					BypasserMCM:  100,
					Timelock:     100,
				}},
			},
			expectedError: "deployer balance is insufficient",
		},
		{
			name: "Invalid Solana chain in environment",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()

				return testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
					selector1: cldf_solana.Chain{}, // Empty chain is invalid
				}))
			},
			config: FundMCMSignerConfig{
				AmountsPerChain: map[uint64]AmountsToTransfer{selector1: {
					ProposeMCM:   100,
					CancellerMCM: 100,
					BypasserMCM:  100,
					Timelock:     100,
				}},
			},
			expectedError: "failed to get existing addresses",
		},
	}

	cs := FundMCMSignersChangeset{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := cs.VerifyPreconditions(tt.env(t), tt.config)
			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expectedError)
			}
		})
	}
}

func TestFundMCMSignersChangeset_Apply(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	deployerKey := solana.NewWallet().PrivateKey
	var confirmed [][]solana.Instruction
	chain := cldf_solana.Chain{
		Selector:    selector,
		DeployerKey: &deployerKey,
		Confirm: func(instructions []solana.Instruction, opts ...solCommonUtil.TxModifier) error {
			confirmed = append(confirmed, instructions)
			return nil
		},
	}
	addressBook := cldf.NewMemoryAddressBook()
	mcmsState := saveMCMSAddresses(t, addressBook, selector, true)
	env := testEnvironment(t, addressBook, cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		selector: chain,
	}))
	cfgAmounts := AmountsToTransfer{
		ProposeMCM:   100 * solana.LAMPORTS_PER_SOL,
		CancellerMCM: 350 * solana.LAMPORTS_PER_SOL,
		BypasserMCM:  75 * solana.LAMPORTS_PER_SOL,
		Timelock:     83 * solana.LAMPORTS_PER_SOL,
	}

	_, err := FundMCMSignersChangeset{}.Apply(env, FundMCMSignerConfig{
		AmountsPerChain: map[uint64]AmountsToTransfer{selector: cfgAmounts},
	})
	require.NoError(t, err)
	require.Len(t, confirmed, 4)

	gotBalances := map[solana.PublicKey]uint64{}
	for _, instructionSet := range confirmed {
		require.Len(t, instructionSet, 1)
		ix := instructionSet[0]
		require.True(t, ix.ProgramID().Equals(system.ProgramID))
		accounts := ix.Accounts()
		require.Len(t, accounts, 2)
		require.True(t, accounts[0].PublicKey.Equals(deployerKey.PublicKey()))
		data, err := ix.Data()
		require.NoError(t, err)
		decoded, err := system.DecodeInstruction(accounts, data)
		require.NoError(t, err)
		transfer, ok := decoded.Impl.(*system.Transfer)
		require.True(t, ok)
		require.NotNil(t, transfer.Lamports)
		gotBalances[accounts[1].PublicKey] = *transfer.Lamports
	}

	require.Equal(t, cfgAmounts.Timelock, gotBalances[solanastate.GetTimelockSignerPDA(mcmsState.TimelockProgram, mcmsState.TimelockSeed)])
	require.Equal(t, cfgAmounts.ProposeMCM, gotBalances[solanastate.GetMCMSignerPDA(mcmsState.McmProgram, mcmsState.ProposerMcmSeed)])
	require.Equal(t, cfgAmounts.CancellerMCM, gotBalances[solanastate.GetMCMSignerPDA(mcmsState.McmProgram, mcmsState.CancellerMcmSeed)])
	require.Equal(t, cfgAmounts.BypasserMCM, gotBalances[solanastate.GetMCMSignerPDA(mcmsState.McmProgram, mcmsState.BypasserMcmSeed)])
}

func testFundMCMSignersEnv(t *testing.T, selector uint64, client *rpc.Client, completeState bool) cldf.Environment {
	t.Helper()

	deployerKey := solana.NewWallet().PrivateKey
	addressBook := cldf.NewMemoryAddressBook()
	saveMCMSAddresses(t, addressBook, selector, completeState)
	chain := cldf_solana.Chain{
		Selector:    selector,
		Client:      client,
		DeployerKey: &deployerKey,
	}

	return testEnvironment(t, addressBook, cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
		selector: chain,
	}))
}

func saveMCMSAddresses(t *testing.T, addressBook cldf.AddressBook, selector uint64, completeState bool) *solanastate.MCMSWithTimelockState {
	t.Helper()

	mcmDummyProgram := solana.NewWallet().PublicKey()
	timelockProgram := solana.NewWallet().PublicKey()
	state := &solanastate.MCMSWithTimelockState{
		MCMSWithTimelockPrograms: &solanastate.MCMSWithTimelockPrograms{
			McmProgram:       mcmDummyProgram,
			TimelockProgram:  timelockProgram,
			ProposerMcmSeed:  solanastate.PDASeed{'t', 'e', 's', 't', '1'},
			CancellerMcmSeed: solanastate.PDASeed{'t', 'e', 's', 't', '2'},
			BypasserMcmSeed:  solanastate.PDASeed{'t', 'e', 's', 't', '3'},
			TimelockSeed:     solanastate.PDASeed{'t', 'e', 's', 't'},
		},
	}

	if !completeState {
		state.ProposerMcmSeed = solanastate.PDASeed{}
		state.CancellerMcmSeed = solanastate.PDASeed{}
		state.BypasserMcmSeed = solanastate.PDASeed{}
		state.TimelockSeed = solanastate.PDASeed{}

		require.NoError(t, addressBook.Save(selector, solanastate.EncodeAddressWithSeed(state.McmProgram, state.BypasserMcmSeed), cldf.NewTypeAndVersion(
			mcmscontracts.BypasserManyChainMultisig,
			cldchangesetscommon.Version1_0_0,
		)))

		return state
	}

	require.NoError(t, addressBook.Save(selector, solanastate.EncodeAddressWithSeed(state.TimelockProgram, state.TimelockSeed), cldf.NewTypeAndVersion(
		mcmscontracts.RBACTimelock,
		cldchangesetscommon.Version1_0_0,
	)))
	require.NoError(t, addressBook.Save(selector, solanastate.EncodeAddressWithSeed(state.McmProgram, state.ProposerMcmSeed), cldf.NewTypeAndVersion(
		mcmscontracts.ProposerManyChainMultisig,
		cldchangesetscommon.Version1_0_0,
	)))
	require.NoError(t, addressBook.Save(selector, solanastate.EncodeAddressWithSeed(state.McmProgram, state.CancellerMcmSeed), cldf.NewTypeAndVersion(
		mcmscontracts.CancellerManyChainMultisig,
		cldchangesetscommon.Version1_0_0,
	)))
	require.NoError(t, addressBook.Save(selector, solanastate.EncodeAddressWithSeed(state.McmProgram, state.BypasserMcmSeed), cldf.NewTypeAndVersion(
		mcmscontracts.BypasserManyChainMultisig,
		cldchangesetscommon.Version1_0_0,
	)))

	return state
}

func testEnvironment(t *testing.T, addressBook cldf.AddressBook, chains cldf_chain.BlockChains) cldf.Environment {
	t.Helper()

	return *cldf.NewEnvironment(
		"test",
		logger.Nop(),
		addressBook,
		datastore.NewMemoryDataStore().Seal(),
		nil,
		nil,
		func() context.Context { return t.Context() },
		ocr.OCRSecrets{},
		chains,
	)
}

func rpcWithBalance(t *testing.T, balance uint64) *rpc.Client {
	t.Helper()

	response := fmt.Sprintf(`{"jsonrpc":"2.0","result":{"context":{"slot":1},"value":%d},"id":1}`, balance)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)

	return rpc.New(server.URL)
}
