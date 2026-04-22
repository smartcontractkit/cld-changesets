package solana

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	cldfsolana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	contractsmcms "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"

	"github.com/smartcontractkit/cld-changesets/pkg/common"
)

// GetState loads the MCMSWithTimelockState from the environment
func GetState(env cldf.Environment, selector uint64) (*MCMSWithTimelockState, error) {
	solanaState, err := MaybeLoadMCMSWithTimelockChainState(env.DataStore.Addresses().Filter(datastore.AddressRefByChainSelector(selector)))
	if err != nil {
		return nil, fmt.Errorf("failed to load solana state: %w", err)
	}

	return solanaState, nil
}

// MCMSWithTimelockState holds the Go bindings for a MCMSWithTimelock contract deployment.
// It is public for use in product specific packages.
type MCMSWithTimelockState struct {
	*MCMSWithTimelockPrograms
}

// MCMSWithTimelockPrograms holds the Solana public keys
// and seeds for the MCM, AccessController and Timelock programs.
// It is public for use in product specific packages.
type MCMSWithTimelockPrograms struct {
	McmProgram                       solana.PublicKey
	ProposerMcmSeed                  PDASeed
	CancellerMcmSeed                 PDASeed
	BypasserMcmSeed                  PDASeed
	TimelockProgram                  solana.PublicKey
	TimelockSeed                     PDASeed
	AccessControllerProgram          solana.PublicKey
	ProposerAccessControllerAccount  solana.PublicKey
	ExecutorAccessControllerAccount  solana.PublicKey
	CancellerAccessControllerAccount solana.PublicKey
	BypasserAccessControllerAccount  solana.PublicKey
}

type PDASeed [32]byte

// MaybeLoadMCMSWithTimelockChainState loads MCMSWithTimelockState from Datastore address refs.
func MaybeLoadMCMSWithTimelockChainState(refs []datastore.AddressRef) (*MCMSWithTimelockState, error) {
	state := MCMSWithTimelockState{MCMSWithTimelockPrograms: &MCMSWithTimelockPrograms{}}

	mcmProgram := datastore.ContractType(contractsmcms.ManyChainMultisigProgram)
	timelockProgram := datastore.ContractType(contractsmcms.RBACTimelockProgram)
	accessControllerProgram := datastore.ContractType(contractsmcms.AccessControllerProgram)
	proposerMCM := datastore.ContractType(contractsmcms.ProposerManyChainMultisig)
	cancellerMCM := datastore.ContractType(contractsmcms.CancellerManyChainMultisig)
	bypasserMCM := datastore.ContractType(contractsmcms.BypasserManyChainMultisig)
	timelock := datastore.ContractType(contractsmcms.RBACTimelock)
	proposerAccessControllerAccount := datastore.ContractType(contractsmcms.ProposerAccessControllerAccount)
	executorAccessControllerAccount := datastore.ContractType(contractsmcms.ExecutorAccessControllerAccount)
	cancellerAccessControllerAccount := datastore.ContractType(contractsmcms.CancellerAccessControllerAccount)
	bypasserAccessControllerAccount := datastore.ContractType(contractsmcms.BypasserAccessControllerAccount)

	for _, ref := range refs {
		address := ref.Address
		switch ref.Type {
		case timelockProgram:
			programID, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode timelock program address (%s): %w", address, err)
			}
			state.TimelockProgram = programID

		case timelock:
			programID, seed, err := decodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode timelock address (%s): %w", address, err)
			}
			state.TimelockProgram = programID
			state.TimelockSeed = seed

		case accessControllerProgram:
			programID, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to parse public key from access controller address (%s): %w", address, err)
			}
			state.AccessControllerProgram = programID

		case proposerAccessControllerAccount:
			account, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode proposer access controller address (%s): %w", address, err)
			}
			state.ProposerAccessControllerAccount = account

		case executorAccessControllerAccount:
			account, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode executor access controller address (%s): %w", address, err)
			}
			state.ExecutorAccessControllerAccount = account

		case cancellerAccessControllerAccount:
			account, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode canceller access controller address (%s): %w", address, err)
			}
			state.CancellerAccessControllerAccount = account

		case bypasserAccessControllerAccount:
			account, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode bypasser access controller address (%s): %w", address, err)
			}
			state.BypasserAccessControllerAccount = account

		case mcmProgram:
			programID, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to parse public key from mcm address (%s): %w", address, err)
			}
			state.McmProgram = programID

		case proposerMCM:
			programID, seed, err := decodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode proposer address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.ProposerMcmSeed = seed

		case bypasserMCM:
			programID, seed, err := decodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode bypasser address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.BypasserMcmSeed = seed

		case cancellerMCM:
			programID, seed, err := decodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode canceller address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.CancellerMcmSeed = seed
		}
	}

	return &state, nil
}

// MaybeLoadMCMSWithTimelockChainStateLegacyAddrBook loads MCMSWithTimelockState from a map of addresses, typically loaded from the environment.
// Deprecated: use MaybeLoadMCMSWithTimelockChainState instead, which loads from Datastore address refs.
func MaybeLoadMCMSWithTimelockChainStateLegacyAddrBook(chain cldfsolana.Chain, addresses map[string]cldf.TypeAndVersion) (*MCMSWithTimelockState, error) {
	state := MCMSWithTimelockState{MCMSWithTimelockPrograms: &MCMSWithTimelockPrograms{}}

	mcmProgram := cldf.NewTypeAndVersion(contractsmcms.ManyChainMultisigProgram, common.Version1_0_0)
	timelockProgram := cldf.NewTypeAndVersion(contractsmcms.RBACTimelockProgram, common.Version1_0_0)
	accessControllerProgram := cldf.NewTypeAndVersion(contractsmcms.AccessControllerProgram, common.Version1_0_0)
	proposerMCM := cldf.NewTypeAndVersion(contractsmcms.ProposerManyChainMultisig, common.Version1_0_0)
	cancellerMCM := cldf.NewTypeAndVersion(contractsmcms.CancellerManyChainMultisig, common.Version1_0_0)
	bypasserMCM := cldf.NewTypeAndVersion(contractsmcms.BypasserManyChainMultisig, common.Version1_0_0)
	timelock := cldf.NewTypeAndVersion(contractsmcms.RBACTimelock, common.Version1_0_0)
	proposerAccessControllerAccount := cldf.NewTypeAndVersion(contractsmcms.ProposerAccessControllerAccount, common.Version1_0_0)
	executorAccessControllerAccount := cldf.NewTypeAndVersion(contractsmcms.ExecutorAccessControllerAccount, common.Version1_0_0)
	cancellerAccessControllerAccount := cldf.NewTypeAndVersion(contractsmcms.CancellerAccessControllerAccount, common.Version1_0_0)
	bypasserAccessControllerAccount := cldf.NewTypeAndVersion(contractsmcms.BypasserAccessControllerAccount, common.Version1_0_0)

	// Convert map keys to a slice
	wantTypes := []cldf.TypeAndVersion{
		mcmProgram, timelockProgram, accessControllerProgram, proposerMCM, cancellerMCM, bypasserMCM, timelock,
		proposerAccessControllerAccount, executorAccessControllerAccount, cancellerAccessControllerAccount,
		bypasserAccessControllerAccount,
	}

	// Ensure we either have the bundle or not.
	_, err := cldf.EnsureDeduped(addresses, wantTypes)
	if err != nil {
		return nil, fmt.Errorf("unable to check MCMS contracts on chain %s error: %w", chain.Name(), err)
	}

	for address, tvStr := range addresses {
		switch {
		case tvStr.Type == timelockProgram.Type && tvStr.Version.String() == timelockProgram.Version.String():
			programID, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode timelock program address (%s): %w", address, err)
			}
			state.TimelockProgram = programID

		case tvStr.Type == timelock.Type && tvStr.Version.String() == timelock.Version.String():
			programID, seed, err := decodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode timelock address (%s): %w", address, err)
			}
			state.TimelockProgram = programID
			state.TimelockSeed = seed

		case tvStr.Type == accessControllerProgram.Type && tvStr.Version.String() == accessControllerProgram.Version.String():
			programID, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to parse public key from access controller address: %s", address)
			}
			state.AccessControllerProgram = programID

		case tvStr.Type == proposerAccessControllerAccount.Type && tvStr.Version.String() == proposerAccessControllerAccount.Version.String():
			account, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode proposer access controller address (%s): %w", address, err)
			}
			state.ProposerAccessControllerAccount = account

		case tvStr.Type == executorAccessControllerAccount.Type && tvStr.Version.String() == executorAccessControllerAccount.Version.String():
			account, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode executor access controller address (%s): %w", address, err)
			}
			state.ExecutorAccessControllerAccount = account

		case tvStr.Type == cancellerAccessControllerAccount.Type && tvStr.Version.String() == cancellerAccessControllerAccount.Version.String():
			account, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode canceller access controller address (%s): %w", address, err)
			}
			state.CancellerAccessControllerAccount = account

		case tvStr.Type == bypasserAccessControllerAccount.Type && tvStr.Version.String() == bypasserAccessControllerAccount.Version.String():
			account, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode bypasser access controller address (%s): %w", address, err)
			}
			state.BypasserAccessControllerAccount = account

		case tvStr.Type == mcmProgram.Type && tvStr.Version.String() == mcmProgram.Version.String():
			programID, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to parse public key from mcm address: %s", address)
			}
			state.McmProgram = programID

		case tvStr.Type == proposerMCM.Type && tvStr.Version.String() == proposerMCM.Version.String():
			programID, seed, err := decodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode proposer address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.ProposerMcmSeed = seed

		case tvStr.Type == bypasserMCM.Type && tvStr.Version.String() == bypasserMCM.Version.String():
			programID, seed, err := decodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode bypasser address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.BypasserMcmSeed = seed

		case tvStr.Type == cancellerMCM.Type && tvStr.Version.String() == cancellerMCM.Version.String():
			programID, seed, err := decodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode canceller address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.CancellerMcmSeed = seed
		}
	}
	return &state, nil
}

func decodeAddressWithSeed(address string) (solana.PublicKey, PDASeed, error) {
	programID, seed, err := mcmssolanasdk.ParseContractAddress(address)
	if err != nil {
		return solana.PublicKey{}, PDASeed{}, fmt.Errorf("unable to parse address %q: %w", address, err)
	}

	return programID, PDASeed(seed), nil
}
