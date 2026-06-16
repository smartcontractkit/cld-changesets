package solreaders

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

func TestReaderGetRefs(t *testing.T) {
	t.Parallel()

	const selector uint64 = 12463857294658392847

	mcmProgram := solanago.NewWallet().PublicKey()
	timelockProgram := solanago.NewWallet().PublicKey()
	accessControllerProgram := solanago.NewWallet().PublicKey()
	proposerSeed := testPDASeed(1)
	cancellerSeed := testPDASeed(2)
	bypasserSeed := testPDASeed(3)
	timelockSeed := testPDASeed(4)

	ds := datastore.NewMemoryDataStore()
	addSolanaRef(t, ds, mcmscontracts.ManyChainMultisigProgram, mcmProgram.String())
	addSolanaRef(t, ds, mcmscontracts.RBACTimelockProgram, timelockProgram.String())
	addSolanaRef(t, ds, mcmscontracts.AccessControllerProgram, accessControllerProgram.String())
	addSolanaRef(t, ds, mcmscontracts.ProposerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, proposerSeed))
	addSolanaRef(t, ds, mcmscontracts.CancellerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, cancellerSeed))
	addSolanaRef(t, ds, mcmscontracts.BypasserManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, bypasserSeed))
	addSolanaRef(t, ds, mcmscontracts.RBACTimelock, mcmssolana.ContractAddress(timelockProgram, timelockSeed))
	addSolanaRef(t, ds, mcmscontracts.ProposerAccessControllerAccount, solanago.NewWallet().PublicKey().String())
	addSolanaRef(t, ds, mcmscontracts.ExecutorAccessControllerAccount, solanago.NewWallet().PublicKey().String())
	addSolanaRef(t, ds, mcmscontracts.CancellerAccessControllerAccount, solanago.NewWallet().PublicKey().String())
	addSolanaRef(t, ds, mcmscontracts.BypasserAccessControllerAccount, solanago.NewWallet().PublicKey().String())

	env := readerTestEnv(ds.Seal())
	env.BlockChains = chain.NewBlockChains(map[uint64]chain.BlockChain{
		selector: cldfsol.Chain{Selector: selector},
	})

	reader := Reader{}
	gotTimelock, err := reader.GetTimelockRef(env, selector, cldf.MCMSTimelockProposalInput{})
	require.NoError(t, err)
	require.Equal(t, mcmssolana.ContractAddress(timelockProgram, timelockSeed), gotTimelock.Address)
	require.Equal(t, datastore.ContractType(mcmscontracts.RBACTimelock), gotTimelock.Type)

	tests := []struct {
		name    string
		action  mcmstypes.TimelockAction
		want    string
		wantTyp datastore.ContractType
	}{
		{
			name:    "default action uses proposer",
			want:    mcmssolana.ContractAddress(mcmProgram, proposerSeed),
			wantTyp: datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		},
		{
			name:    "schedule uses proposer",
			action:  mcmstypes.TimelockActionSchedule,
			want:    mcmssolana.ContractAddress(mcmProgram, proposerSeed),
			wantTyp: datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		},
		{
			name:    "cancel uses canceller",
			action:  mcmstypes.TimelockActionCancel,
			want:    mcmssolana.ContractAddress(mcmProgram, cancellerSeed),
			wantTyp: datastore.ContractType(mcmscontracts.CancellerManyChainMultisig),
		},
		{
			name:    "bypass uses bypasser",
			action:  mcmstypes.TimelockActionBypass,
			want:    mcmssolana.ContractAddress(mcmProgram, bypasserSeed),
			wantTyp: datastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, refErr := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{TimelockAction: tt.action})
			require.NoError(t, refErr)
			require.Equal(t, tt.want, got.Address)
			require.Equal(t, tt.wantTyp, got.Type)
		})
	}
}

func TestReaderErrors(t *testing.T) {
	t.Parallel()

	const selector uint64 = 12463857294658392847

	reader := Reader{}

	_, err := reader.GetMCMSRef(readerTestEnv(datastore.NewMemoryDataStore().Seal()), 1, cldf.MCMSTimelockProposalInput{})
	require.EqualError(t, err, "chain 1 not found")

	ds := datastore.NewMemoryDataStore()
	env := readerTestEnv(ds.Seal())
	env.BlockChains = chain.NewBlockChains(map[uint64]chain.BlockChain{
		selector: cldfsol.Chain{Selector: selector},
	})
	_, err = reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockAction("bad"),
	})
	require.EqualError(t, err, `invalid MCMS action "bad"`)
}

func TestSolanaAccessControllerPubkey_errors(t *testing.T) {
	t.Parallel()

	const selector uint64 = 12463857294658392847
	env := readerTestEnv(datastore.NewMemoryDataStore().Seal())
	env.BlockChains = chain.NewBlockChains(map[uint64]chain.BlockChain{
		selector: cldfsol.Chain{Selector: selector},
	})

	_, err := solanaAccessControllerPubkey(env, selector, mcmscontracts.ProposerAccessControllerAccount, "")
	require.ErrorContains(t, err, "resolve ProposerAccessControllerAccount")

	ds := datastore.NewMemoryDataStore()
	addSolanaRef(t, ds, mcmscontracts.ProposerAccessControllerAccount, "not-a-valid-pubkey")
	env = readerTestEnv(ds.Seal())
	env.BlockChains = chain.NewBlockChains(map[uint64]chain.BlockChain{
		selector: cldfsol.Chain{Selector: selector},
	})

	_, err = solanaAccessControllerPubkey(env, selector, mcmscontracts.ProposerAccessControllerAccount, "")
	require.ErrorContains(t, err, "parse ProposerAccessControllerAccount address")
}

func TestReaderGetChainMetadata_errors(t *testing.T) {
	t.Parallel()

	const selector uint64 = 12463857294658392847
	reader := Reader{}

	_, err := reader.GetChainMetadata(readerTestEnv(datastore.NewMemoryDataStore().Seal()), 1, cldf.MCMSTimelockProposalInput{})
	require.EqualError(t, err, "chain 1 not found")

	ds := datastore.NewMemoryDataStore()
	addSolanaRef(t, ds, mcmscontracts.ProposerManyChainMultisig, "not-a-valid-mcm-address")
	env := readerTestEnv(ds.Seal())
	env.BlockChains = chain.NewBlockChains(map[uint64]chain.BlockChain{
		selector: cldfsol.Chain{Selector: selector},
	})

	_, err = reader.GetChainMetadata(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
	})
	require.ErrorContains(t, err, "parse MCMS address")

	missingAccessDS := datastore.NewMemoryDataStore()
	mcmProgram := solanago.NewWallet().PublicKey()
	addSolanaRef(t, missingAccessDS, mcmscontracts.ProposerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(1)))
	env = readerTestEnv(missingAccessDS.Seal())
	env.BlockChains = chain.NewBlockChains(map[uint64]chain.BlockChain{
		selector: cldfsol.Chain{Selector: selector},
	})

	_, err = reader.GetChainMetadata(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
	})
	require.ErrorContains(t, err, "resolve ProposerAccessControllerAccount")
}

func TestReaderGetTimelockRef_missingDatastore(t *testing.T) {
	t.Parallel()

	const selector uint64 = 12463857294658392847
	env := cldf.Environment{
		GetContext:  context.Background,
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{selector: cldfsol.Chain{Selector: selector}}),
	}

	_, err := Reader{}.GetTimelockRef(env, selector, cldf.MCMSTimelockProposalInput{})
	require.ErrorContains(t, err, "datastore not available")
}

func TestReaderGetChainMetadata_missingAccessControllers(t *testing.T) {
	t.Parallel()

	const selector uint64 = 12463857294658392847
	mcmProgram := solanago.NewWallet().PublicKey()

	tests := []struct {
		name    string
		setup   func(t *testing.T) *datastore.MemoryDataStore
		wantErr string
	}{
		{
			name: "missing canceller access controller",
			setup: func(t *testing.T) *datastore.MemoryDataStore {
				t.Helper()
				ds := datastore.NewMemoryDataStore()
				addSolanaRef(t, ds, mcmscontracts.ProposerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(1)))
				addSolanaRef(t, ds, mcmscontracts.ProposerAccessControllerAccount, solanago.NewWallet().PublicKey().String())
				addSolanaRef(t, ds, mcmscontracts.BypasserAccessControllerAccount, solanago.NewWallet().PublicKey().String())

				return ds
			},
			wantErr: "resolve CancellerAccessControllerAccount",
		},
		{
			name: "missing bypasser access controller",
			setup: func(t *testing.T) *datastore.MemoryDataStore {
				t.Helper()
				ds := datastore.NewMemoryDataStore()
				addSolanaRef(t, ds, mcmscontracts.ProposerManyChainMultisig, mcmssolana.ContractAddress(mcmProgram, testPDASeed(1)))
				addSolanaRef(t, ds, mcmscontracts.ProposerAccessControllerAccount, solanago.NewWallet().PublicKey().String())
				addSolanaRef(t, ds, mcmscontracts.CancellerAccessControllerAccount, solanago.NewWallet().PublicKey().String())

				return ds
			},
			wantErr: "resolve BypasserAccessControllerAccount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := readerTestEnv(tt.setup(t).Seal())
			env.BlockChains = chain.NewBlockChains(map[uint64]chain.BlockChain{
				selector: cldfsol.Chain{Selector: selector},
			})

			_, err := Reader{}.GetChainMetadata(env, selector, cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionSchedule,
			})
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestSolanaAddressRef_missingDatastore(t *testing.T) {
	t.Parallel()

	_, err := solanaAddressRef(cldf.Environment{}, 1, mcmscontracts.RBACTimelock, "")
	require.EqualError(t, err, "datastore not available for chain 1")
}

func addSolanaRef(t *testing.T, ds *datastore.MemoryDataStore, typ cldf.ContractType, address string) {
	t.Helper()

	const selector uint64 = 12463857294658392847
	version := semver.MustParse("1.0.0")
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       address,
		ChainSelector: selector,
		Type:          datastore.ContractType(typ),
		Version:       version,
	}))
}

func testPDASeed(v byte) mcmssolana.PDASeed {
	var seed mcmssolana.PDASeed
	for i := range seed {
		seed[i] = v
	}

	return seed
}

func readerTestEnv(ds datastore.DataStore) cldf.Environment {
	return cldf.Environment{
		Logger:            logger.Nop(),
		ExistingAddresses: cldf.NewMemoryAddressBook(),
		DataStore:         ds,
		GetContext:        context.Background,
	}
}
