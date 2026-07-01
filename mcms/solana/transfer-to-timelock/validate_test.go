package soltransfertotimelock

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfchain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

func TestValidateContracts_duplicateResolvedContract(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	version1 := &semvers.V1_0_0
	version2 := semver.MustParse("1.1.0")

	mcmProgram := solanago.NewWallet().PublicKey()
	timelockProgram := solanago.NewWallet().PublicKey()
	accessControllerProgram := solanago.NewWallet().PublicKey()
	proposerSeed := testPDASeed(1)
	timelockSeed := testPDASeed(4)
	sharedAccount := solanago.NewWallet().PublicKey()

	ds := datastore.NewMemoryDataStore()
	addSolanaValidateRef(t, ds, selector, mcmscontracts.ManyChainMultisigProgram, version1, mcmProgram.String())
	addSolanaValidateRef(t, ds, selector, mcmscontracts.RBACTimelockProgram, version1, timelockProgram.String())
	addSolanaValidateRef(t, ds, selector, mcmscontracts.AccessControllerProgram, version1, accessControllerProgram.String())
	addSolanaValidateRef(t, ds, selector, mcmscontracts.ProposerManyChainMultisig, version1, mcmssolana.ContractAddress(mcmProgram, proposerSeed))
	addSolanaValidateRef(t, ds, selector, mcmscontracts.RBACTimelock, version1, mcmssolana.ContractAddress(timelockProgram, timelockSeed))
	addSolanaValidateRef(t, ds, selector, mcmscontracts.ProposerAccessControllerAccount, version1, sharedAccount.String())
	addSolanaValidateRef(t, ds, selector, mcmscontracts.ExecutorAccessControllerAccount, version2, sharedAccount.String())

	ref1 := refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerAccessControllerAccount), version1, "")
	ref2 := refkey.New(selector, datastore.ContractType(mcmscontracts.ExecutorAccessControllerAccount), version2, "")

	deployerKey := solanago.NewWallet().PrivateKey
	env := cldf.Environment{
		Logger:    logger.Nop(),
		DataStore: ds.Seal(),
		GetContext: func() context.Context {
			return t.Context()
		},
		BlockChains: cldfchain.NewBlockChains(map[uint64]cldfchain.BlockChain{
			selector: cldfsol.Chain{
				Selector:    selector,
				DeployerKey: &deployerKey,
			},
		}),
	}

	err := validateContracts(env, transfertotimelock.ChainInput{
		ChainSelector: selector,
		Contracts:     []refkey.RefKey{ref1, ref2},
		MCMS:          &cldf.MCMSTimelockProposalInput{},
	})
	require.ErrorContains(t, err, "duplicate contract "+sharedAccount.String())
}

func TestResolveOwnableContract_fillsChainSelector(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	version := &semvers.V1_0_0
	accessControllerProgram := solanago.NewWallet().PublicKey()
	account := solanago.NewWallet().PublicKey()

	ds := datastore.NewMemoryDataStore()
	addSolanaValidateRef(t, ds, selector, mcmscontracts.AccessControllerProgram, version, accessControllerProgram.String())
	addSolanaValidateRef(t, ds, selector, mcmscontracts.ProposerAccessControllerAccount, version, account.String())

	env := cldf.Environment{
		Logger:    logger.Nop(),
		DataStore: ds.Seal(),
	}

	ref := refkey.RefKey{
		Type:    datastore.ContractType(mcmscontracts.ProposerAccessControllerAccount),
		Version: version,
	}

	got, err := resolveOwnableContract(env, selector, ref)
	require.NoError(t, err)
	require.Equal(t, account, got.OwnerPDA)
}

func addSolanaValidateRef(
	t *testing.T,
	ds *datastore.MemoryDataStore,
	selector uint64,
	contractType cldf.ContractType,
	version *semver.Version,
	address string,
) {
	t.Helper()

	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		ChainSelector: selector,
		Type:          datastore.ContractType(contractType),
		Version:       version,
		Address:       address,
	}))
}

func testPDASeed(v byte) mcmssolana.PDASeed {
	var seed mcmssolana.PDASeed
	seed[0] = v

	return seed
}

func TestValidateContractOwner(t *testing.T) {
	t.Parallel()

	contract := OwnableContract{Type: mcmscontracts.ProposerManyChainMultisig}
	deployer := solanago.NewWallet().PublicKey()
	timelock := solanago.NewWallet().PublicKey()
	other := solanago.NewWallet().PublicKey()

	tests := []struct {
		name       string
		owner      solanago.PublicKey
		onlyAccept bool
		wantErr    string
	}{
		{
			name:  "timelock already owns",
			owner: timelock,
		},
		{
			name:       "only accept with deployer owner",
			owner:      deployer,
			onlyAccept: true,
		},
		{
			name:       "only accept rejects third party",
			owner:      other,
			onlyAccept: true,
			wantErr:    "only accept ownership requires current owner to be deployer or timelock",
		},
		{
			name:  "full transfer requires deployer owner",
			owner: deployer,
		},
		{
			name:    "full transfer rejects third party",
			owner:   other,
			wantErr: "not owned by the deployer key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateContractOwner(contract, tt.owner, deployer, timelock, tt.onlyAccept)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
