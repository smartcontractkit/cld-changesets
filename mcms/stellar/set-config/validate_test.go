package stellarsetconfig

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/readers"
)

type validateRefSpec struct {
	contractType cldf.ContractType
	address      string
}

func TestVerifyStellarChains_MCMSRefs(t *testing.T) {
	t.Parallel()

	selector := chainselectors.STELLAR_LOCALNET.Selector
	mcmsInput := cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
	}

	tests := []struct {
		name    string
		refs    []validateRefSpec
		mcms    *cldf.MCMSTimelockProposalInput
		wantErr string
	}{
		{
			name: "direct mode does not require MCMS refs",
		},
		{
			name:    "missing timelock ref",
			mcms:    &mcmsInput,
			wantErr: "resolve Stellar timelock",
		},
		{
			name: "missing MCMS ref",
			refs: []validateRefSpec{
				{
					contractType: mcmscontracts.RBACTimelock,
					address:      testStellarContractID(t, 1),
				},
			},
			mcms:    &mcmsInput,
			wantErr: "resolve Stellar MCMS",
		},
		{
			name: "success",
			refs: []validateRefSpec{
				{
					contractType: mcmscontracts.RBACTimelock,
					address:      testStellarContractID(t, 1),
				},
				{
					contractType: mcmscontracts.ProposerManyChainMultisig,
					address:      testStellarContractID(t, 2),
				},
			},
			mcms: &mcmsInput,
		},
	}

	reg := Registration()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ds := datastore.NewMemoryDataStore()
			for _, ref := range tt.refs {
				require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
					Address:       ref.address,
					ChainSelector: selector,
					Type:          datastore.ContractType(ref.contractType),
					Version:       &semvers.V1_0_0,
				}))
			}

			env := stellarSetConfigValidateEnv(ds.Seal(), selector)

			err := reg.Verify(
				env,
				[]setconfig.ChainInput{
					{
						ChainSelector: selector,
						MCMS:          tt.mcms,
					},
				},
			)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func stellarSetConfigValidateEnv(
	ds datastore.DataStore,
	selector uint64,
) cldf.Environment {
	return cldf.Environment{
		Logger:     logger.Nop(),
		DataStore:  ds,
		GetContext: context.Background,
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldfstellar.Chain{
				ChainMetadata: cldfstellar.ChainMetadata{Selector: selector},
			},
		}),
	}
}
