package stellarsetconfig

import (
	"context"
	"testing"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
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

	selector := chainselectors.STELLAR_LOCALNET.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)
	validRef := refkey.New(
		selector,
		datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		&semvers.V1_0_0,
		"",
	)

	address := testStellarContractID(t, 1)
	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       address,
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       &semvers.V1_0_0,
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
	require.Equal(t, address, got[0].Address)
	require.Equal(t, cfg, got[0].Config)
	require.Equal(t, string(mcmscontracts.ProposerManyChainMultisig), got[0].ContractType)

	_, err = setConfigTargets(env, []setconfig.ContractSetConfig{
		{
			Ref: refkey.New(
				selector,
				datastore.ContractType(mcmscontracts.CancellerManyChainMultisig),
				&semvers.V1_0_0,
				"",
			),
			Config: cfg,
		},
	})
	require.ErrorContains(t, err, "targets[0]:")

	invalidDS := datastore.NewMemoryDataStore()
	require.NoError(t, invalidDS.Addresses().Add(datastore.AddressRef{
		Address:       "not-a-stellar-address",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       &semvers.V1_0_0,
	}))

	_, err = setConfigTargets(
		cldf.Environment{
			DataStore:  invalidDS.Seal(),
			GetContext: context.Background,
		},
		[]setconfig.ContractSetConfig{
			{Ref: validRef, Config: cfg},
		},
	)
	require.ErrorContains(t, err, `invalid Stellar address "not-a-stellar-address"`)
}

func TestRunStellarSetConfig_Errors(t *testing.T) {
	t.Parallel()

	selector := chainselectors.STELLAR_LOCALNET.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)

	_, err := runStellarSetConfig(
		optest.NewBundle(t),
		setconfig.Deps{
			BlockChains: chain.NewBlockChains(nil),
		},
		setconfig.ChainInput{
			ChainSelector: selector,
		},
	)
	require.ErrorContains(t, err, "stellar chain")

	deps := setconfig.Deps{
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldfstellar.Chain{
				ChainMetadata: cldfstellar.ChainMetadata{Selector: selector},
			},
		}),
		DataStore: datastore.NewMemoryDataStore().Seal(),
	}

	_, err = runStellarSetConfig(
		optest.NewBundle(t),
		deps,
		setconfig.ChainInput{
			ChainSelector: selector,
			Targets: []setconfig.ContractSetConfig{
				{
					Ref: refkey.New(
						selector,
						datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
						&semvers.V1_0_0,
						"",
					),
					Config: cfg,
				},
			},
		},
	)
	require.ErrorContains(t, err, "targets[0]:")
}

func testStellarContractID(t *testing.T, fill byte) string {
	t.Helper()

	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = fill
	}

	address, err := strkey.Encode(strkey.VersionByteContract, raw)
	require.NoError(t, err)

	return address
}
