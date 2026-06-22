package evmfiredrill

import (
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
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	firedrill "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
)

func testMCMSInput() firedrill.ChainInput {
	return firedrill.ChainInput{
		MCMS: cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			TimelockDelay:  mcmstypes.NewDuration(time.Second),
		},
	}
}

func testFireDrillDeps(t *testing.T, selector uint64, timelockAddress string) firedrill.Deps {
	t.Helper()

	ds := datastore.NewMemoryDataStore()
	version := semver.MustParse("1.0.0")
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       timelockAddress,
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.RBACTimelock),
		Version:       version,
	}))
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "0x0000000000000000000000000000000000000200",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       version,
	}))

	return firedrill.Deps{
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldf_evm.Chain{Selector: selector},
		}),
		DataStore: ds.Seal(),
	}
}

func TestRunEVMFireDrill_errors(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	input := testMCMSInput()
	input.ChainSelector = selector

	t.Run("chain not in environment", func(t *testing.T) {
		t.Parallel()

		_, err := runEVMFireDrill(
			optest.NewBundle(t),
			firedrill.Deps{BlockChains: chain.NewBlockChains(nil)},
			input,
		)
		require.ErrorContains(t, err, "EVM chain")
	})

	t.Run("invalid timelock address", func(t *testing.T) {
		t.Parallel()

		_, err := runEVMFireDrill(
			optest.NewBundle(t),
			testFireDrillDeps(t, selector, "not-an-address"),
			input,
		)
		require.ErrorContains(t, err, `invalid timelock address "not-an-address"`)
	})

	t.Run("zero timelock address", func(t *testing.T) {
		t.Parallel()

		_, err := runEVMFireDrill(
			optest.NewBundle(t),
			testFireDrillDeps(t, selector, "0x0000000000000000000000000000000000000000"),
			input,
		)
		require.ErrorContains(t, err, "timelock address is zero")
	})
}

func TestRunEVMFireDrill_success(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	timelock := "0x0000000000000000000000000000000000000100"
	input := testMCMSInput()
	input.ChainSelector = selector

	out, err := runEVMFireDrill(
		optest.NewBundle(t),
		testFireDrillDeps(t, selector, timelock),
		input,
	)
	require.NoError(t, err)
	require.Len(t, out.BatchOps, 1)
	require.Equal(t, mcmstypes.ChainSelector(selector), out.BatchOps[0].ChainSelector)
	require.Len(t, out.BatchOps[0].Transactions, 1)
	require.Equal(t, timelock, out.BatchOps[0].Transactions[0].To)
}
