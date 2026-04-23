package solana

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
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

// MaybeLoadMCMSWithTimelockState loads MCMSWithTimelockState for each provided chain selector from env.DataStore address refs.
func MaybeLoadMCMSWithTimelockState(env cldf.Environment, chainSelectors []uint64) (map[uint64]*MCMSWithTimelockState, error) {
	result := map[uint64]*MCMSWithTimelockState{}
	for _, chainSelector := range chainSelectors {
		state, err := MaybeLoadMCMSWithTimelockChainState(env.DataStore.Addresses().Filter(datastore.AddressRefByChainSelector(chainSelector)))
		if err != nil {
			return nil, fmt.Errorf("unable to load mcms and timelock solana chain state for chain selector %d: %w", chainSelector, err)
		}
		result[chainSelector] = state
	}

	return result, nil
}

// MaybeLoadMCMSWithTimelockChainState loads MCMSWithTimelockState from Datastore address refs.
func MaybeLoadMCMSWithTimelockChainState(refs []datastore.AddressRef) (*MCMSWithTimelockState, error) {
	state := MCMSWithTimelockState{MCMSWithTimelockPrograms: &MCMSWithTimelockPrograms{}}

	mcmProgram := datastore.ContractType(mcmscontracts.ManyChainMultisigProgram)
	timelockProgram := datastore.ContractType(mcmscontracts.RBACTimelockProgram)
	accessControllerProgram := datastore.ContractType(mcmscontracts.AccessControllerProgram)
	proposerMCM := datastore.ContractType(mcmscontracts.ProposerManyChainMultisig)
	cancellerMCM := datastore.ContractType(mcmscontracts.CancellerManyChainMultisig)
	bypasserMCM := datastore.ContractType(mcmscontracts.BypasserManyChainMultisig)
	timelock := datastore.ContractType(mcmscontracts.RBACTimelock)
	proposerAccessControllerAccount := datastore.ContractType(mcmscontracts.ProposerAccessControllerAccount)
	executorAccessControllerAccount := datastore.ContractType(mcmscontracts.ExecutorAccessControllerAccount)
	cancellerAccessControllerAccount := datastore.ContractType(mcmscontracts.CancellerAccessControllerAccount)
	bypasserAccessControllerAccount := datastore.ContractType(mcmscontracts.BypasserAccessControllerAccount)

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

func decodeAddressWithSeed(address string) (solana.PublicKey, PDASeed, error) {
	programID, seed, err := mcmssolanasdk.ParseContractAddress(address)
	if err != nil {
		return solana.PublicKey{}, PDASeed{}, fmt.Errorf("unable to parse address %q: %w", address, err)
	}

	return programID, PDASeed(seed), nil
}
