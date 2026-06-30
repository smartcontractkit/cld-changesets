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
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

func TestSetConfigTargets(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)
	validRef := refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, "")

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "proposer-account",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       semver.MustParse("1.0.0"),
	}))
	env := cldf.Environment{
		DataStore:  ds.Seal(),
		GetContext: context.Background,
	}

	got, err := setConfigTargets(env, []setconfig.ContractSetConfig{
		{Ref: validRef, Config: cfg},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "proposer-account", got[0].Address)
	require.Equal(t, cfg, got[0].Config)

	_, err = setConfigTargets(env, []setconfig.ContractSetConfig{
		{Ref: refkey.New(selector, datastore.ContractType(mcmscontracts.CancellerManyChainMultisig), &semvers.V1_0_0, ""), Config: cfg},
	})
	require.ErrorContains(t, err, "targets[0]:")
}

func TestRunSolanaSetConfig_errors(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)

	_, err := runSolanaSetConfig(
		optest.NewBundle(t),
		setconfig.Deps{BlockChains: chain.NewBlockChains(nil)},
		setconfig.ChainInput{ChainSelector: selector},
	)
	require.ErrorContains(t, err, "solana chain")

	deps := setconfig.Deps{
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
		DataStore: datastore.NewMemoryDataStore().Seal(),
	}
	_, err = runSolanaSetConfig(
		optest.NewBundle(t),
		deps,
		setconfig.ChainInput{
			ChainSelector: selector,
			Targets: []setconfig.ContractSetConfig{
				{
					Ref:    refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, ""),
					Config: cfg,
				},
			},
		},
	)
	require.ErrorContains(t, err, "targets[0]:")

	mcmsInput := &cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
	}
	_, err = runSolanaSetConfig(
		optest.NewBundle(t),
		deps,
		setconfig.ChainInput{
			ChainSelector: selector,
			Targets: []setconfig.ContractSetConfig{
				{
					Ref:    refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, ""),
					Config: cfg,
				},
			},
			MCMS: mcmsInput,
		},
	)
	require.ErrorContains(t, err, "resolve timelock ref")
}

func TestRunSolanaSetConfig_invalidTimelockAddress(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)
	version := semver.MustParse("1.0.0")

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "timelock-account",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.RBACTimelock),
		Version:       version,
	}))
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "proposer-account",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       version,
	}))

	deps := setconfig.Deps{
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
		DataStore: ds.Seal(),
	}
	_, err := runSolanaSetConfig(
		optest.NewBundle(t),
		deps,
		setconfig.ChainInput{
			ChainSelector: selector,
			Targets: []setconfig.ContractSetConfig{
				{
					Ref:    refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, ""),
					Config: cfg,
				},
			},
			MCMS: &cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionSchedule,
				ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
				TimelockDelay:  mcmstypes.NewDuration(time.Second),
			},
		},
	)
	require.ErrorContains(t, err, "parse timelock ref address")
}
