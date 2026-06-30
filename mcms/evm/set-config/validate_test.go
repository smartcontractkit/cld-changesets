package evmsetconfig_test

import (
	"context"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	evmsetconfig "github.com/smartcontractkit/cld-changesets/mcms/evm/set-config"

	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
)

type validateRefSpec struct {
	contractType cldf.ContractType
	address      string
}

func TestValidateMCMSIfPresent(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	version := semver.MustParse("1.0.0")
	mcmsInput := cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
	}
	reg := evmsetconfig.Registration()

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
			refs:    nil,
			mcms:    &mcmsInput,
			wantErr: "validate timelock ref",
		},
		{
			name: "missing call proxy ref",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, "0x100"},
				{mcmscontracts.ProposerManyChainMultisig, "0x200"},
			},
			mcms:    &mcmsInput,
			wantErr: "validate call proxy ref",
		},
		{
			name: "success",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, "0x100"},
				{mcmscontracts.ProposerManyChainMultisig, "0x200"},
				{mcmscontracts.CallProxy, "0x300"},
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

			err := reg.Verify(
				validateTestEnv(ds.Seal(), selector),
				[]setconfig.ChainInput{{
					ChainSelector: selector,
					MCMS:          tt.mcms,
				}},
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
		Logger:    logger.Nop(),
		DataStore: ds,
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldf_evm.Chain{Selector: selector},
		}),
		GetContext: context.Background,
	}
}
