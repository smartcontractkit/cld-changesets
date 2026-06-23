package soldeploy_test

import (
	"fmt"
	"testing"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	soldeploy "github.com/smartcontractkit/cld-changesets/mcms/solana/deploy"
)

func TestRegistration(t *testing.T) {
	t.Parallel()

	reg := soldeploy.Registration()
	require.Equal(t, chainselectors.FamilySolana, reg.Family)
	require.NotNil(t, reg.Sequence)
	require.NotNil(t, reg.Verify)
}

func TestVerifySolanaChains(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	reg := soldeploy.Registration()

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

			err := reg.Verify(tt.env, []deploy.ChainInput{{ChainSelector: selector}})
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
