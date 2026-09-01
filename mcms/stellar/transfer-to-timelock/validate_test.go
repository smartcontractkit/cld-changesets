package stellartransfertotimelock

import (
	"context"
	"testing"
	"time"

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
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"

	_ "github.com/smartcontractkit/cld-changesets/mcms/stellar/readers"
)

type transferValidateRef struct {
	contractType cldf.ContractType
	address      string
	qualifier    string
}

func TestVerifyStellarChains(t *testing.T) {
	selector := chainselectors.STELLAR_LOCALNET.Selector
	qualifier := "qualified"

	tests := []struct {
		name         string
		chainPresent bool
		useMCMS      bool
		action       mcmstypes.TimelockAction
		refs         []transferValidateRef
		wantErr      string
	}{
		{
			name:    "chain missing from environment",
			wantErr: "not found in environment",
		},
		{
			name:         "chain present without MCMS",
			chainPresent: true,
		},
		{
			name:         "missing timelock ref",
			chainPresent: true,
			useMCMS:      true,
			action:       mcmstypes.TimelockActionSchedule,
			wantErr:      "resolve Stellar timelock",
		},
		{
			name:         "missing schedule MCMS ref",
			chainPresent: true,
			useMCMS:      true,
			action:       mcmstypes.TimelockActionSchedule,
			refs: []transferValidateRef{
				{
					contractType: mcmscontracts.RBACTimelock,
					address:      "timelock",
					qualifier:    qualifier,
				},
			},
			wantErr: "resolve Stellar MCMS",
		},
		{
			name:         "schedule success",
			chainPresent: true,
			useMCMS:      true,
			action:       mcmstypes.TimelockActionSchedule,
			refs: []transferValidateRef{
				{
					contractType: mcmscontracts.RBACTimelock,
					address:      "timelock",
					qualifier:    qualifier,
				},
				{
					contractType: mcmscontracts.ProposerManyChainMultisig,
					address:      "proposer",
					qualifier:    qualifier,
				},
			},
		},
		{
			name:         "bypass does not use proposer",
			chainPresent: true,
			useMCMS:      true,
			action:       mcmstypes.TimelockActionBypass,
			refs: []transferValidateRef{
				{
					contractType: mcmscontracts.RBACTimelock,
					address:      "timelock",
					qualifier:    qualifier,
				},
				{
					contractType: mcmscontracts.ProposerManyChainMultisig,
					address:      "proposer",
					qualifier:    qualifier,
				},
			},
			wantErr: "resolve Stellar MCMS",
		},
		{
			name:         "bypass success",
			chainPresent: true,
			useMCMS:      true,
			action:       mcmstypes.TimelockActionBypass,
			refs: []transferValidateRef{
				{
					contractType: mcmscontracts.RBACTimelock,
					address:      "timelock",
					qualifier:    qualifier,
				},
				{
					contractType: mcmscontracts.BypasserManyChainMultisig,
					address:      "bypasser",
					qualifier:    qualifier,
				},
			},
		},
		{
			name:         "qualifier is part of identity",
			chainPresent: true,
			useMCMS:      true,
			action:       mcmstypes.TimelockActionSchedule,
			refs: []transferValidateRef{
				{
					contractType: mcmscontracts.RBACTimelock,
					address:      "timelock",
				},
				{
					contractType: mcmscontracts.ProposerManyChainMultisig,
					address:      "proposer",
				},
			},
			wantErr: "resolve Stellar timelock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := datastore.NewMemoryDataStore()

			for _, ref := range tt.refs {
				require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
					Address:       ref.address,
					ChainSelector: selector,
					Type:          datastore.ContractType(ref.contractType),
					Version:       &semvers.V1_0_0,
					Qualifier:     ref.qualifier,
				}))
			}

			env := transferValidateEnvironment(
				ds.Seal(),
				selector,
				tt.chainPresent,
			)

			input := transfertotimelock.ChainInput{
				ChainSelector: selector,
			}

			if tt.useMCMS {
				input.MCMS = &cldf.MCMSTimelockProposalInput{
					TimelockAction: tt.action,
					ValidUntil: uint32(
						time.Now().
							Add(2 * time.Hour).
							UTC().
							Unix(),
					), //nolint:gosec // test timestamp
					TimelockDelay: mcmstypes.NewDuration(0),
					Qualifier:     qualifier,
				}
			}

			err := verifyStellarChains(
				env,
				[]transfertotimelock.ChainInput{input},
			)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func transferValidateEnvironment(
	ds datastore.DataStore,
	selector uint64,
	chainPresent bool,
) cldf.Environment {
	blockChains := chain.NewBlockChains(nil)

	if chainPresent {
		blockChains = chain.NewBlockChains(
			map[uint64]chain.BlockChain{
				selector: cldfstellar.Chain{
					ChainMetadata: cldfstellar.ChainMetadata{Selector: selector},
				},
			},
		)
	}

	return cldf.Environment{
		Logger:      logger.Nop(),
		DataStore:   ds,
		GetContext:  context.Background,
		BlockChains: blockChains,
	}
}
