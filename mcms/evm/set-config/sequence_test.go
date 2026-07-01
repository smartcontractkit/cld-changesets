package evmsetconfig

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
)

func TestSetConfigTargets(t *testing.T) {
	t.Parallel()

	const selector uint64 = 90000001
	cfg := cldftesthelpers.SingleGroupMCMS(t)
	validRef := refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, "")

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "0x0000000000000000000000000000000000000100",
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
	require.Equal(t, "0x0000000000000000000000000000000000000100", got[0].Address.Hex())
	require.Equal(t, cfg, got[0].Config)

	_, err = setConfigTargets(env, []setconfig.ContractSetConfig{
		{Ref: refkey.New(selector, datastore.ContractType(mcmscontracts.CancellerManyChainMultisig), &semvers.V1_0_0, ""), Config: cfg},
	})
	require.ErrorContains(t, err, "targets[0]:")

	invalidDS := datastore.NewMemoryDataStore()
	require.NoError(t, invalidDS.Addresses().Add(datastore.AddressRef{
		Address:       "not-an-address",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       semver.MustParse("1.0.0"),
	}))
	_, err = setConfigTargets(cldf.Environment{DataStore: invalidDS.Seal(), GetContext: context.Background}, []setconfig.ContractSetConfig{
		{Ref: validRef, Config: cfg},
	})
	require.ErrorContains(t, err, `invalid EVM address "not-an-address"`)
}

func TestRunEVMSetConfig_errors(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)

	_, err := runEVMSetConfig(
		optest.NewBundle(t),
		setconfig.Deps{BlockChains: chain.NewBlockChains(nil)},
		setconfig.ChainInput{ChainSelector: selector},
	)
	require.ErrorContains(t, err, "EVM chain")

	deps := setconfig.Deps{
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldf_evm.Chain{Selector: selector},
		}),
		DataStore: datastore.NewMemoryDataStore().Seal(),
	}
	_, err = runEVMSetConfig(
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
}
