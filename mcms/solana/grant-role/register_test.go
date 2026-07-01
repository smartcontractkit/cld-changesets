package solgrantrole

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
)

func TestRegistration(t *testing.T) {
	t.Parallel()

	reg := Registration()
	require.Equal(t, chainselectors.FamilySolana, reg.Family)
	require.Equal(t, seqGrantRole, reg.Sequence)
	require.NotNil(t, reg.Verify)
}

func TestInitRegistersSolana(t *testing.T) {
	t.Parallel()

	seq, err := grantrole.Registry.SequenceForFamily(chainselectors.FamilySolana)
	require.NoError(t, err)
	require.Equal(t, seqGrantRole, seq)

	got, err := grantrole.Registry.SequenceForChainSelector(
		chainselectors.TEST_22222222222222222222222222222222222222222222.Selector,
	)
	require.NoError(t, err)
	require.Equal(t, seqGrantRole, got)
}

func TestVerifySolanaChains(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	reg := Registration()

	tests := []struct {
		name    string
		env     cldf.Environment
		chains  []grantrole.SeqInput
		wantErr string
	}{
		{
			name:    "chain missing from environment",
			env:     cldf.Environment{},
			wantErr: fmt.Sprintf("solana chain %d not found in environment", selector),
		},
		{
			name: "invalid grant role",
			env: cldf.Environment{
				BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
					selector: cldfsol.Chain{Selector: selector},
				}),
			},
			chains: []grantrole.SeqInput{{
				ChainSelector: selector,
				Grants: []grantrole.RoleGrant{{
					Role:      mcmssdk.TimelockRoleAdmin,
					Addresses: []string{"11111111111111111111111111111112"},
				}},
			}},
			wantErr: fmt.Sprintf("chain %d: grants[0]: admin role not supported on solana", selector),
		},
		{
			name: "chain present",
			env: cldf.Environment{
				BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
					selector: cldfsol.Chain{Selector: selector},
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chains := tt.chains
			if chains == nil {
				chains = []grantrole.SeqInput{{ChainSelector: selector}}
			}

			err := reg.Verify(tt.env, chains)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, tt.wantErr)
		})
	}
}
