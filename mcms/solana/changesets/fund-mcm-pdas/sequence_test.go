package fundmcmpdas

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	solCommonUtil "github.com/smartcontractkit/chainlink-ccip/chains/solana/utils/common"
	cldfchain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	"github.com/stretchr/testify/require"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

func TestSeqFundSolanaMCMPDAs(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	deployerKey := solana.NewWallet().PrivateKey
	var confirmed int
	solChain := cldfsol.Chain{
		Selector:    selector,
		DeployerKey: &deployerKey,
		Confirm: func(_ []solana.Instruction, _ ...solCommonUtil.TxModifier) error {
			confirmed++
			return nil
		},
	}

	targets := []FundingTarget{
		{Address: solana.NewWallet().PublicKey(), Amount: 1},
		{Address: solana.NewWallet().PublicKey(), Amount: 0},
		{Address: solana.NewWallet().PublicKey(), Amount: 2},
	}

	report, err := operations.ExecuteSequence(
		optest.NewBundle(t),
		SeqFundSolanaMCMPDAs,
		solChain,
		SeqFundSolanaMCMPDAsInput{
			ChainSelector: selector,
			Targets:       targets,
		},
	)
	require.NoError(t, err)
	require.Empty(t, report.Output.BatchOps)
	require.Equal(t, 2, confirmed)
}

func TestSeqFundSolanaMCMPDAs_chainMismatch(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	deployerKey := solana.NewWallet().PrivateKey

	_, err := operations.ExecuteSequence(
		optest.NewBundle(t),
		SeqFundSolanaMCMPDAs,
		cldfsol.Chain{Selector: selector, DeployerKey: &deployerKey},
		SeqFundSolanaMCMPDAsInput{ChainSelector: selector + 1},
	)
	require.ErrorContains(t, err, "mismatch between input chain selector")
}

func TestSeqFundMCMPDAs_errors(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector

	_, err := operations.ExecuteSequence(
		optest.NewBundle(t),
		SeqFundMCMPDAs,
		Deps{BlockChains: cldfchain.NewBlockChains(nil)},
		ChainInput{ChainSelector: selector},
	)
	require.ErrorContains(t, err, "solana chain")

	deps := Deps{
		BlockChains: cldfchain.NewBlockChains(map[uint64]cldfchain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
		DataStore: datastore.NewMemoryDataStore().Seal(),
	}
	_, err = operations.ExecuteSequence(
		optest.NewBundle(t),
		SeqFundMCMPDAs,
		deps,
		ChainInput{ChainSelector: selector},
	)
	require.ErrorContains(t, err, "resolve timelock ref")
}

func TestSeqFundMCMPDAs_success(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	deployerKey := solana.NewWallet().PrivateKey
	solChain := cldfsol.Chain{
		Selector:    selector,
		DeployerKey: &deployerKey,
		Confirm: func(_ []solana.Instruction, _ ...solCommonUtil.TxModifier) error {
			return nil
		},
	}

	ds := newSequenceDataStore(t, selector)
	deps := Deps{
		BlockChains: cldfchain.NewBlockChains(map[uint64]cldfchain.BlockChain{selector: solChain}),
		DataStore:   ds,
	}

	_, err := operations.ExecuteSequence(
		optest.NewBundle(t),
		SeqFundMCMPDAs,
		deps,
		ChainInput{
			ChainSelector: selector,
			FundingConfig: FundingConfig{
				ProposeMCM:   1,
				CancellerMCM: 2,
				BypasserMCM:  3,
				Timelock:     4,
			},
		},
	)
	require.NoError(t, err)
}

func newSequenceDataStore(t *testing.T, selector uint64) datastore.DataStore {
	t.Helper()

	mcmProgram := solana.NewWallet().PublicKey()
	timelockProgram := solana.NewWallet().PublicKey()
	version := semver.MustParse("1.0.0")
	ds := datastore.NewMemoryDataStore()

	for _, ref := range []struct {
		typ     cldf.ContractType
		address string
	}{
		{mcmscontracts.RBACTimelock, mcmssolana.ContractAddress(timelockProgram, testPDASeed(4))},
		{mcmscontracts.ProposerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(1))},
		{mcmscontracts.CancellerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(2))},
		{mcmscontracts.BypasserManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(3))},
	} {
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       ref.address,
			ChainSelector: selector,
			Type:          datastore.ContractType(ref.typ),
			Version:       version,
		}))
	}

	return ds.Seal()
}
