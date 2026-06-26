package fundmcmpdas

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	"github.com/stretchr/testify/require"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

func TestValidateMCMSRefs(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	version := semver.MustParse("1.0.0")

	tests := []struct {
		name    string
		refs    []validateRefSpec
		wantErr string
	}{
		{
			name:    "missing timelock ref",
			wantErr: "resolve timelock ref",
		},
		{
			name: "missing proposer ref",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, mcmssolana.ContractAddress(solana.NewWallet().PublicKey(), testPDASeed(4))},
			},
			wantErr: "resolve proposer ref",
		},
		{
			name: "invalid timelock address",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, "timelock"},
				{mcmscontracts.ProposerManyChainMultisig, mcmssolana.ContractAddress(solana.NewWallet().PublicKey(), testPDASeed(1))},
				{mcmscontracts.CancellerManyChainMultisig, mcmssolana.ContractAddress(solana.NewWallet().PublicKey(), testPDASeed(2))},
				{mcmscontracts.BypasserManyChainMultisig, mcmssolana.ContractAddress(solana.NewWallet().PublicKey(), testPDASeed(3))},
			},
			wantErr: "parse timelock signer PDA",
		},
		{
			name: "success",
			refs: completeMCMSRefs(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ds := datastore.NewMemoryDataStore()
			for _, ref := range tt.refs {
				addValidateRef(t, ds, selector, ref.contractType, ref.address, version, "")
			}

			err := validateMCMSRefs(
				validateTestEnv(ds.Seal(), selector),
				selector,
				FundingConfig{},
			)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestValidateDeployerBalance(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	deployerKey := solana.NewWallet().PrivateKey

	tests := []struct {
		name    string
		chain   cldfsol.Chain
		cfg     FundingConfig
		wantErr string
	}{
		{
			name:    "missing client",
			chain:   cldfsol.Chain{Selector: selector, DeployerKey: &deployerKey},
			wantErr: "solana client missing",
		},
		{
			name:    "missing deployer key",
			chain:   cldfsol.Chain{Selector: selector, Client: rpcWithBalance(t, 1000)},
			wantErr: "deployer key missing",
		},
		{
			name: "insufficient balance",
			chain: cldfsol.Chain{
				Selector:    selector,
				Client:      rpcWithBalance(t, 10),
				DeployerKey: &deployerKey,
			},
			cfg: FundingConfig{
				ProposeMCM:   100,
				CancellerMCM: 100,
				BypasserMCM:  100,
				Timelock:     100,
			},
			wantErr: "deployer balance is insufficient",
		},
		{
			name: "success",
			chain: cldfsol.Chain{
				Selector:    selector,
				Client:      rpcWithBalance(t, 1000),
				DeployerKey: &deployerKey,
			},
			cfg: FundingConfig{
				ProposeMCM:   100,
				CancellerMCM: 100,
				BypasserMCM:  100,
				Timelock:     100,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateDeployerBalance(
				validateTestEnv(datastore.NewMemoryDataStore().Seal(), selector, tt.chain),
				selector,
				tt.cfg,
			)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

type validateRefSpec struct {
	contractType cldf.ContractType
	address      string
}

func completeMCMSRefs() []validateRefSpec {
	mcmProgram := solana.NewWallet().PublicKey()
	timelockProgram := solana.NewWallet().PublicKey()

	return []validateRefSpec{
		{mcmscontracts.RBACTimelock, mcmssolana.ContractAddress(timelockProgram, testPDASeed(4))},
		{mcmscontracts.ProposerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(1))},
		{mcmscontracts.CancellerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(2))},
		{mcmscontracts.BypasserManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(3))},
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

func validateTestEnv(ds datastore.DataStore, selector uint64, chains ...cldfsol.Chain) cldf.Environment {
	blockChains := chain.NewBlockChains(map[uint64]chain.BlockChain{
		selector: cldfsol.Chain{Selector: selector},
	})
	if len(chains) > 0 {
		blockChains = chain.NewBlockChains(map[uint64]chain.BlockChain{selector: chains[0]})
	}

	return cldf.Environment{
		Logger:      logger.Nop(),
		DataStore:   ds,
		GetContext:  context.Background,
		BlockChains: blockChains,
	}
}
