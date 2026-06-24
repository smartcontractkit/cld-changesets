package solsetconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
)

func TestRegistration(t *testing.T) {
	t.Parallel()

	reg := Registration()
	require.Equal(t, chainselectors.FamilySolana, reg.Family)
	require.Equal(t, seqSetConfig, reg.Sequence)
	require.NotNil(t, reg.Verify)
}

func TestInitRegistersSolana(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "sequence for family",
			run: func(t *testing.T) {
				t.Helper()
				seq, err := setconfig.Registry.SequenceForFamily(chainselectors.FamilySolana)
				require.NoError(t, err)
				require.Equal(t, seqSetConfig, seq)
			},
		},
		{
			name: "sequence for chain selector",
			run: func(t *testing.T) {
				t.Helper()
				got, err := setconfig.Registry.SequenceForChainSelector(
					chainselectors.TEST_22222222222222222222222222222222222222222222.Selector,
				)
				require.NoError(t, err)
				require.Equal(t, seqSetConfig, got)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestVerifySolanaChains(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	reg := Registration()

	tests := []struct {
		name    string
		env     cldf.Environment
		wantErr string
	}{
		{
			name:    "chain missing from environment",
			env:     cldf.Environment{},
			wantErr: fmt.Sprintf("solana chain %d not found in environment", selector),
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

			err := reg.Verify(tt.env, []setconfig.ChainInput{{ChainSelector: selector}})
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
