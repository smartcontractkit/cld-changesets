package changesets

import (
	"fmt"
	"testing"

	"github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	timelockbindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/timelock"
)

func TestGrantRoleTimelockSolana_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	sel := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	missingSel := uint64(999991)
	acct := solana.NewWallet().PublicKey()

	tests := []struct {
		name          string
		env           func(t *testing.T) cldf.Environment
		config        GrantRoleTimelockSolanaConfig
		expectedError string
	}{
		{
			name: "All preconditions satisfied",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()
				ab := cldf.NewMemoryAddressBook()
				saveMCMSAddresses(t, ab, sel, true)

				return testEnvironment(t, ab, cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
					sel: cldf_solana.Chain{Selector: sel},
				}))
			},
			config: GrantRoleTimelockSolanaConfig{
				Accounts: map[uint64][]solana.PublicKey{sel: {acct}},
				Role:     timelockbindings.Executor_Role,
				MCMS:     &cldfproposalutils.TimelockConfig{MCMSAction: mcmstypes.TimelockActionSchedule},
			},
			expectedError: "",
		},
		{
			name: "Nil MCMS allowed",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()
				ab := cldf.NewMemoryAddressBook()
				saveMCMSAddresses(t, ab, sel, true)

				return testEnvironment(t, ab, cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
					sel: cldf_solana.Chain{Selector: sel},
				}))
			},
			config: GrantRoleTimelockSolanaConfig{
				Accounts: map[uint64][]solana.PublicKey{sel: {acct}},
				Role:     timelockbindings.Bypasser_Role,
				MCMS:     nil,
			},
			expectedError: "",
		},
		{
			name: "No Solana chains in environment",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()

				return testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(nil))
			},
			config: GrantRoleTimelockSolanaConfig{
				Accounts: map[uint64][]solana.PublicKey{sel: {acct}},
				Role:     timelockbindings.Proposer_Role,
			},
			expectedError: "no solana chains provided",
		},
		{
			name: "Chain selector not found in environment",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()

				return testEnvironment(t, cldf.NewMemoryAddressBook(), cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
					sel: cldf_solana.Chain{Selector: sel},
				}))
			},
			config: GrantRoleTimelockSolanaConfig{
				Accounts: map[uint64][]solana.PublicKey{missingSel: {acct}},
				Role:     timelockbindings.Proposer_Role,
			},
			expectedError: fmt.Sprintf("solana chain not found for selector %d", missingSel),
		},
		{
			name: "Invalid MCMS action",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()
				ab := cldf.NewMemoryAddressBook()
				saveMCMSAddresses(t, ab, sel, true)

				return testEnvironment(t, ab, cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
					sel: cldf_solana.Chain{Selector: sel},
				}))
			},
			config: GrantRoleTimelockSolanaConfig{
				Accounts: map[uint64][]solana.PublicKey{sel: {acct}},
				Role:     timelockbindings.Proposer_Role,
				MCMS: &cldfproposalutils.TimelockConfig{
					MCMSAction: mcmstypes.TimelockAction("unsupported-action"),
				},
			},
			expectedError: "invalid mcms action",
		},
		{
			name: "Incomplete MCMS address fixture",
			env: func(t *testing.T) cldf.Environment {
				t.Helper()
				ab := cldf.NewMemoryAddressBook()
				saveMCMSAddresses(t, ab, sel, false)

				return testEnvironment(t, ab, cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
					sel: cldf_solana.Chain{Selector: sel},
				}))
			},
			config: GrantRoleTimelockSolanaConfig{
				Accounts: map[uint64][]solana.PublicKey{sel: {acct}},
				Role:     timelockbindings.Proposer_Role,
			},
			expectedError: "timelock program not deployed for chain",
		},
	}

	cs := GrantRoleTimelockSolana{}

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
