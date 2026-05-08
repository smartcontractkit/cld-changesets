package changesets

import (
	"encoding/json"
	"testing"

	chainsel "github.com/smartcontractkit/chain-selectors"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/stretchr/testify/require"

	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"

	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"

	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/onchain"
)

func TestDeployMCMSWithConfig(t *testing.T) {
	t.Parallel()
	lggr := logger.Test(t)

	selector := chainsel.TEST_90000001.Selector
	blockchains, err := onchain.NewEVMSimLoader().Load(t, []uint64{selector})
	require.NoError(t, err)
	require.Len(t, blockchains, 1)

	// Convert the blockchains to concrete EVM chains.
	chain, ok := blockchains[0].(cldf_evm.Chain)
	require.True(t, ok)

	ab := cldf.NewMemoryAddressBook()

	// 1) Test WITHOUT a label
	mcmNoLabel, err := DeployMCMSWithConfigEVM(
		mcmscontracts.ProposerManyChainMultisig,
		lggr,
		chain,
		ab,
		cldftesthelpers.SingleGroupMCMS(t),
	)
	require.NoError(t, err)
	require.Empty(t, mcmNoLabel.Tv.Labels, "expected no label to be set")

	// 2) Test WITH a label
	label := "SA"
	mcmWithLabel, err := DeployMCMSWithConfigEVM(
		mcmscontracts.ProposerManyChainMultisig,
		lggr,
		chain,
		ab,
		cldftesthelpers.SingleGroupMCMS(t),
		WithLabel(label),
	)
	require.NoError(t, err)
	require.NotNil(t, mcmWithLabel.Tv.Labels, "expected labels to be set")
	require.Contains(t, mcmWithLabel.Tv.Labels, label, "label mismatch")
}

func TestDeployMCMSWithTimelockContracts(t *testing.T) {
	t.Parallel()
	selector := chainsel.TEST_90000001.Selector
	env, err := environment.New(t.Context(),
		environment.WithEVMSimulated(t, []uint64{selector}),
	)
	require.NoError(t, err)

	chain := env.BlockChains.EVMChains()[selector]

	ab := cldf.NewMemoryAddressBook()

	_, err = DeployMCMSWithTimelockContractsEVM(*env,
		chain,
		ab,
		cldftesthelpers.SingleGroupTimelockConfig(t),
		nil,
	)
	require.NoError(t, err)

	addresses, err := ab.AddressesForChain(chainsel.TEST_90000001.Selector)
	require.NoError(t, err)
	require.Len(t, addresses, 5)

	mcmsState, err := evmstate.MaybeLoadMCMSWithTimelockChainState(chain, addresses)
	require.NoError(t, err)

	v, err := mcmsState.GenerateMCMSWithTimelockView()
	require.NoError(t, err)

	_, err = json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
}
