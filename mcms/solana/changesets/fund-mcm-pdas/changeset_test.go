package fundmcmpdas

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	solCommonUtil "github.com/smartcontractkit/chainlink-ccip/chains/solana/utils/common"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	"github.com/stretchr/testify/require"

	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

//nolint:paralleltest // global mcm.SetProgramID state; shared Solana CTF container
func TestChangeset(t *testing.T) {
	selector1 := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	selector2 := chainselectors.TEST_33333333333333333333333333333333333333333333.Selector

	rt1 := testRuntime(t, selector1)
	env1 := configureFundMCMSignersEnv(t, rt1.Environment(), selector1, rpcWithBalance(t, 1_000))
	cs := Changeset{}

	t.Run("VerifyPreconditions", func(t *testing.T) {
		tests := []struct {
			name          string
			env           cldf.Environment
			config        Config
			expectedError string
		}{
			{
				name: "all preconditions satisfied",
				env:  env1,
				config: Config{
					FundingPerChain: map[uint64]FundingConfig{selector1: {
						ProposeMCM:   100,
						CancellerMCM: 100,
						BypasserMCM:  100,
						Timelock:     100,
					}},
				},
			},
			{
				name:          "no funding config",
				env:           env1,
				config:        Config{},
				expectedError: "no funding config provided",
			},
			{
				name: "no solana chains found in environment",
				env: func() cldf.Environment {
					env := rt1.Environment()
					env.BlockChains = cldf_chain.NewBlockChains(nil)

					return env
				}(),
				config: Config{
					FundingPerChain: map[uint64]FundingConfig{selector1: {
						ProposeMCM:   100,
						CancellerMCM: 100,
						BypasserMCM:  100,
						Timelock:     100,
					}},
				},
				expectedError: fmt.Sprintf("solana chain %d not found in environment", selector1),
			},
			{
				name: "chain selector not found in environment",
				env:  env1,
				config: Config{FundingPerChain: map[uint64]FundingConfig{99999: {
					ProposeMCM:   100,
					CancellerMCM: 100,
					BypasserMCM:  100,
					Timelock:     100,
				}}},
				expectedError: "solana chain 99999 not found in environment",
			},
			{
				name: "insufficient deployer balance",
				env:  configureFundMCMSignersEnv(t, rt1.Environment(), selector1, rpcWithBalance(t, 1)),
				config: Config{
					FundingPerChain: map[uint64]FundingConfig{selector1: {
						ProposeMCM:   100,
						CancellerMCM: 100,
						BypasserMCM:  100,
						Timelock:     100,
					}},
				},
				expectedError: "deployer balance is insufficient",
			},
			{
				name: "missing deployer key",
				env: func() cldf.Environment {
					env := configureFundMCMSignersEnv(t, rt1.Environment(), selector1, rpcWithBalance(t, 1_000))
					chain := env.BlockChains.SolanaChains()[selector1]
					chain.DeployerKey = nil
					env.BlockChains = cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{selector1: chain})

					return env
				}(),
				config: Config{
					FundingPerChain: map[uint64]FundingConfig{selector1: {
						ProposeMCM:   100,
						CancellerMCM: 100,
						BypasserMCM:  100,
						Timelock:     100,
					}},
				},
				expectedError: "deployer key missing",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := cs.VerifyPreconditions(tt.env, tt.config)
				if tt.expectedError == "" {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.ErrorContains(t, err, tt.expectedError)
				}
			})
		}

		t.Run("mcms contracts not deployed", func(t *testing.T) {
			chain := rt1.Environment().BlockChains.SolanaChains()[selector1]
			chain.Client = rpcWithBalance(t, 1_000)
			env := cldf.Environment{DataStore: newMCMSDataStore(t, selector2, false)}
			env.BlockChains = cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{selector2: chain})

			err := cs.VerifyPreconditions(env, Config{
				FundingPerChain: map[uint64]FundingConfig{selector2: {
					ProposeMCM:   100,
					CancellerMCM: 100,
					BypasserMCM:  100,
					Timelock:     100,
				}},
			})
			require.ErrorContains(t, err, "resolve timelock ref")
		})
	})

	t.Run("Apply", func(t *testing.T) {
		var confirmed [][]solana.Instruction

		env := configureFundMCMSignersEnv(t, rt1.Environment(), selector1, nil)
		chain := env.BlockChains.SolanaChains()[selector1]
		require.NotNil(t, chain.DeployerKey)
		deployerKey := *chain.DeployerKey
		chain.Confirm = func(instructions []solana.Instruction, _ ...solCommonUtil.TxModifier) error {
			confirmed = append(confirmed, instructions)
			return nil
		}
		env.BlockChains = cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{selector1: chain})
		env.OperationsBundle = optest.NewBundle(t)

		cfgAmounts := FundingConfig{
			ProposeMCM:   100 * solana.LAMPORTS_PER_SOL,
			CancellerMCM: 350 * solana.LAMPORTS_PER_SOL,
			BypasserMCM:  75 * solana.LAMPORTS_PER_SOL,
			Timelock:     83 * solana.LAMPORTS_PER_SOL,
		}

		refs := testFundingRefs(t, env, selector1, "")
		_, err := cs.Apply(env, Config{
			FundingPerChain: map[uint64]FundingConfig{selector1: cfgAmounts},
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
			data, dataErr := ix.Data()
			require.NoError(t, dataErr)
			decoded, decodeErr := system.DecodeInstruction(accounts, data)
			require.NoError(t, decodeErr)
			transfer, ok := decoded.Impl.(*system.Transfer)
			require.True(t, ok)
			require.NotNil(t, transfer.Lamports)
			gotBalances[accounts[1].PublicKey] = *transfer.Lamports
		}

		require.Equal(t, cfgAmounts.Timelock, gotBalances[refs.TimelockSigner])
		require.Equal(t, cfgAmounts.ProposeMCM, gotBalances[refs.ProposerSigner])
		require.Equal(t, cfgAmounts.CancellerMCM, gotBalances[refs.CancellerSigner])
		require.Equal(t, cfgAmounts.BypasserMCM, gotBalances[refs.BypasserSigner])
	})

	t.Run("Apply_sequenceError", func(t *testing.T) {
		selector := uint64(99999)
		env := rt1.Environment()
		env.BlockChains = cldf_chain.NewBlockChains(nil)
		env.OperationsBundle = optest.NewBundle(t)

		_, err := cs.Apply(env, Config{
			FundingPerChain: map[uint64]FundingConfig{
				selector: {ProposeMCM: 1},
			},
		})
		require.Error(t, err)
	})
}

type fundingRefSet struct {
	TimelockSigner  solana.PublicKey
	ProposerSigner  solana.PublicKey
	CancellerSigner solana.PublicKey
	BypasserSigner  solana.PublicKey
}

func testFundingRefs(t *testing.T, env cldf.Environment, selector uint64, qualifier string) fundingRefSet {
	t.Helper()

	targets, err := ResolveFundingTargets(env, selector, FundingConfig{Qualifier: qualifier})
	require.NoError(t, err)
	require.Len(t, targets, 4)

	return fundingRefSet{
		TimelockSigner:  targets[0].Address,
		ProposerSigner:  targets[1].Address,
		CancellerSigner: targets[2].Address,
		BypasserSigner:  targets[3].Address,
	}
}

func configureFundMCMSignersEnv(
	t *testing.T,
	base cldf.Environment,
	selector uint64,
	client *rpc.Client,
) cldf.Environment {
	t.Helper()

	env := base
	env.DataStore = newMCMSDataStore(t, selector, true)

	chain := env.BlockChains.SolanaChains()[selector]
	if client != nil {
		chain.Client = client
	}
	env.BlockChains = cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{selector: chain})

	return env
}

func newMCMSDataStore(t *testing.T, selector uint64, completeState bool) datastore.DataStore {
	t.Helper()

	mcmProgram := solana.NewWallet().PublicKey()
	timelockProgram := solana.NewWallet().PublicKey()
	proposerSeed := testPDASeed(1)
	cancellerSeed := testPDASeed(2)
	bypasserSeed := testPDASeed(3)
	timelockSeed := testPDASeed(4)

	ds := datastore.NewMemoryDataStore()
	version := semver.MustParse("1.0.0")

	if !completeState {
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       mcmssolana.ContractAddress(mcmProgram, bypasserSeed),
			ChainSelector: selector,
			Type:          datastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
			Version:       version,
		}))

		return ds.Seal()
	}

	for _, ref := range []struct {
		typ     cldf.ContractType
		address string
	}{
		{mcmscontracts.RBACTimelock, mcmssolana.ContractAddress(timelockProgram, timelockSeed)},
		{mcmscontracts.ProposerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, proposerSeed)},
		{mcmscontracts.CancellerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, cancellerSeed)},
		{mcmscontracts.BypasserManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, bypasserSeed)},
	} {
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       ref.address,
			ChainSelector: selector,
			Type:          datastore.ContractType(ref.typ),
			Version:       version,
		}))
	}

	return ds.Seal()
}

func testRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	programsPath, programIDs, ab := soltestutils.PreloadMCMS(t, selector)
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{selector}, programsPath, programIDs),
		environment.WithAddressBook(ab),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)
	require.Contains(t, rt.Environment().BlockChains.SolanaChains(), selector)

	return rt
}

func rpcWithBalance(t *testing.T, balance uint64) *rpc.Client {
	t.Helper()

	response := fmt.Sprintf(`{"jsonrpc":"2.0","result":{"context":{"slot":1},"value":%d},"id":1}`, balance)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)

	return rpc.New(server.URL)
}
