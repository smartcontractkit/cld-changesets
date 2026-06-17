package solsetconfig

import (
	"context"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

type validateRefSpec struct {
	contractType cldf.ContractType
	address      string
}

func TestValidateMCMSIfPresent(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	version := semver.MustParse("1.0.0")
	mcmsInput := cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
	}

	tests := []struct {
		name    string
		refs    []validateRefSpec
		mcms    *cldf.MCMSTimelockProposalInput
		wantErr string
	}{
		{
			name: "nil MCMS",
		},
		{
			name:    "missing timelock ref",
			mcms:    &mcmsInput,
			wantErr: "validate timelock ref",
		},
		{
			name: "missing mcms ref",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, "timelock"},
			},
			mcms:    &mcmsInput,
			wantErr: "validate mcms ref",
		},
		{
			name: "success",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, "timelock"},
				{mcmscontracts.ProposerManyChainMultisig, "proposer"},
			},
			mcms: &mcmsInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ds := datastore.NewMemoryDataStore()
			for _, ref := range tt.refs {
				addValidateRef(t, ds, selector, ref.contractType, ref.address, version, "")
			}

			err := validateMCMSIfPresent(
				validateTestEnv(ds.Seal(), selector),
				setconfig.ChainInput{
					ChainSelector: selector,
					MCMS:          tt.mcms,
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

func addValidateRef(
	t *testing.T,
	ds *datastore.MemoryDataStore,
	selector uint64,
	contractType cldf.ContractType,
	address string,
	version *semver.Version,
	qualifier string,
) {
	t.Helper()

	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       address,
		ChainSelector: selector,
		Type:          datastore.ContractType(contractType),
		Version:       version,
		Qualifier:     qualifier,
	}))
}

func validateTestEnv(ds datastore.DataStore, selector uint64) cldf.Environment {
	return cldf.Environment{
		Logger:     logger.Nop(),
		DataStore:  ds,
		GetContext: context.Background,
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
	}
}
