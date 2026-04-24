package solana

import (
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"
	timelockBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/timelock"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"

	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
)

type PDASeed [32]byte

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

func (s *MCMSWithTimelockPrograms) GetStateFromType(programType cldf.ContractType) (solana.PublicKey, PDASeed, error) {
	switch programType {
	case mcmscontracts.ManyChainMultisigProgram:
		return s.McmProgram, PDASeed{}, nil
	case mcmscontracts.ProposerManyChainMultisig:
		return s.McmProgram, s.ProposerMcmSeed, nil
	case mcmscontracts.BypasserManyChainMultisig:
		return s.McmProgram, s.BypasserMcmSeed, nil
	case mcmscontracts.CancellerManyChainMultisig:
		return s.McmProgram, s.CancellerMcmSeed, nil
	case mcmscontracts.RBACTimelockProgram:
		return s.TimelockProgram, PDASeed{}, nil
	case mcmscontracts.RBACTimelock:
		return s.TimelockProgram, s.TimelockSeed, nil
	case mcmscontracts.AccessControllerProgram:
		return s.AccessControllerProgram, PDASeed{}, nil
	case mcmscontracts.ProposerAccessControllerAccount:
		return s.AccessControllerProgram, PDASeed(s.ProposerAccessControllerAccount), nil
	case mcmscontracts.ExecutorAccessControllerAccount:
		return s.AccessControllerProgram, PDASeed(s.ExecutorAccessControllerAccount), nil
	case mcmscontracts.CancellerAccessControllerAccount:
		return s.AccessControllerProgram, PDASeed(s.CancellerAccessControllerAccount), nil
	case mcmscontracts.BypasserAccessControllerAccount:
		return s.AccessControllerProgram, PDASeed(s.BypasserAccessControllerAccount), nil
	default:
		return solana.PublicKey{}, PDASeed{}, fmt.Errorf("unknown program type: %s", programType)
	}
}

func (s *MCMSWithTimelockPrograms) SetState(contractType cldf.ContractType, program solana.PublicKey, seed PDASeed) error {
	switch contractType {
	case mcmscontracts.ManyChainMultisigProgram:
		s.McmProgram = program
	case mcmscontracts.ProposerManyChainMultisig:
		s.McmProgram = program
		s.ProposerMcmSeed = seed
	case mcmscontracts.BypasserManyChainMultisig:
		s.McmProgram = program
		s.BypasserMcmSeed = seed
	case mcmscontracts.CancellerManyChainMultisig:
		s.McmProgram = program
		s.CancellerMcmSeed = seed
	case mcmscontracts.RBACTimelockProgram:
		s.TimelockProgram = program
	case mcmscontracts.RBACTimelock:
		s.TimelockProgram = program
		s.TimelockSeed = seed
	case mcmscontracts.AccessControllerProgram:
		s.AccessControllerProgram = program
	case mcmscontracts.ProposerAccessControllerAccount:
		s.ProposerAccessControllerAccount = program
	case mcmscontracts.ExecutorAccessControllerAccount:
		s.ExecutorAccessControllerAccount = program
	case mcmscontracts.CancellerAccessControllerAccount:
		s.CancellerAccessControllerAccount = program
	case mcmscontracts.BypasserAccessControllerAccount:
		s.BypasserAccessControllerAccount = program
	default:
		return fmt.Errorf("unknown contract type: %s", contractType)
	}

	return nil
}

func (s *MCMSWithTimelockPrograms) RoleAccount(role timelockBindings.Role) solana.PublicKey {
	switch role {
	case timelockBindings.Admin_Role:
		return solana.PublicKey{}
	case timelockBindings.Proposer_Role:
		return s.ProposerAccessControllerAccount
	case timelockBindings.Executor_Role:
		return s.ExecutorAccessControllerAccount
	case timelockBindings.Canceller_Role:
		return s.CancellerAccessControllerAccount
	case timelockBindings.Bypasser_Role:
		return s.BypasserAccessControllerAccount
	}

	return solana.PublicKey{}
}

// GetState loads the MCMSWithTimelockState from the environment
func GetState(env cldf.Environment, selector uint64) (*MCMSWithTimelockState, error) {
	solanaChains := env.BlockChains.SolanaChains()
	chain, ok := solanaChains[selector]
	if !ok {
		return nil, fmt.Errorf("chain %d not found", selector)
	}

	var err1 error
	addresses, err := env.ExistingAddresses.AddressesForChain(selector) //nolint:staticcheck // SA1019: AddressBook deprecated; primary path still reads ExistingAddresses before DataStore fallback.
	if err == nil {
		solanaState, loadErr := MaybeLoadMCMSWithTimelockChainState(chain, addresses)
		if loadErr == nil {
			return solanaState, nil
		}
		err1 = loadErr
		env.Logger.Warnf("failed to load MCMSState from address book for selector %d: %v; falling back to DataStore", selector, loadErr)
	} else if !errors.Is(err, cldf.ErrChainNotFound) {
		return nil, fmt.Errorf("failed to get addresses for chain %d: %w", selector, err)
	} else {
		env.Logger.Warnf("no address book entry for chain selector %d; loading MCMS state from DataStore", selector)
	}

	if env.DataStore == nil {
		if err1 != nil {
			return nil, fmt.Errorf("failed to load solana state: %w", err1)
		}

		return nil, fmt.Errorf("no DataStore available for chain %d after address book miss", selector)
	}

	solanaState, err2 := MaybeLoadMCMSWithTimelockChainStateV2(env.DataStore.Addresses().Filter(datastore.AddressRefByChainSelector(selector)))
	if err2 != nil {
		if err1 != nil {
			return nil, fmt.Errorf("failed to load solana state: %w", errors.Join(err1, err2))
		}

		return nil, fmt.Errorf("failed to load solana state: %w", err2)
	}

	return solanaState, nil
}

// MCMSWithTimelockState holds the Go bindings for a MCMSWithTimelock contract deployment.
// It is public for use in product specific packages.
type MCMSWithTimelockState struct {
	*MCMSWithTimelockPrograms
}

// MaybeLoadMCMSWithTimelockState loads MCMSWithTimelockState for each provided chain selector from the environment's
// Solana chains and ExistingAddresses (address book).
func MaybeLoadMCMSWithTimelockState(env cldf.Environment, chainSelectors []uint64) (map[uint64]*MCMSWithTimelockState, error) {
	result := map[uint64]*MCMSWithTimelockState{}
	solChains := env.BlockChains.SolanaChains()
	for _, chainSelector := range chainSelectors {
		chain, ok := solChains[chainSelector]
		if !ok {
			return nil, fmt.Errorf("chain %d not found", chainSelector)
		}
		addressesChain, err := env.ExistingAddresses.AddressesForChain(chainSelector) //nolint:staticcheck // SA1019: AddressBook deprecated; Solana MCMS load merges with address book until full DataStore migration.
		if err != nil {
			if !errors.Is(err, cldf.ErrChainNotFound) {
				return nil, fmt.Errorf("unable to get addresses for chain %v: %w", chainSelector, err)
			}
			// chain not found in address book, initialize empty
			addressesChain = make(map[string]cldf.TypeAndVersion)
		}
		state, err := MaybeLoadMCMSWithTimelockChainState(chain, addressesChain)
		if err != nil {
			return nil, fmt.Errorf("unable to load mcms and timelock solana chain state: %w", err)
		}
		result[chainSelector] = state
	}

	return result, nil
}

// MaybeLoadMCMSWithTimelockChainState looks for the addresses corresponding to
// contracts deployed with DeployMCMSWithTimelock and loads them into a
// MCMSWithTimelockStateSolana struct. If none of the contracts are found, the
// state struct will be nil.
// An error indicates:
// - Found but was unable to load a contract
// - It only found part of the bundle of contracts
// - If found more than one instance of a contract (we expect one bundle in the given addresses)
func MaybeLoadMCMSWithTimelockChainState(chain cldf_solana.Chain, addresses map[string]cldf.TypeAndVersion) (*MCMSWithTimelockState, error) {
	state := MCMSWithTimelockState{MCMSWithTimelockPrograms: &MCMSWithTimelockPrograms{}}

	mcmProgram := cldf.NewTypeAndVersion(mcmscontracts.ManyChainMultisigProgram, cldchangesetscommon.Version1_0_0)
	timelockProgram := cldf.NewTypeAndVersion(mcmscontracts.RBACTimelockProgram, cldchangesetscommon.Version1_0_0)
	accessControllerProgram := cldf.NewTypeAndVersion(mcmscontracts.AccessControllerProgram, cldchangesetscommon.Version1_0_0)
	proposerMCM := cldf.NewTypeAndVersion(mcmscontracts.ProposerManyChainMultisig, cldchangesetscommon.Version1_0_0)
	cancellerMCM := cldf.NewTypeAndVersion(mcmscontracts.CancellerManyChainMultisig, cldchangesetscommon.Version1_0_0)
	bypasserMCM := cldf.NewTypeAndVersion(mcmscontracts.BypasserManyChainMultisig, cldchangesetscommon.Version1_0_0)
	timelock := cldf.NewTypeAndVersion(mcmscontracts.RBACTimelock, cldchangesetscommon.Version1_0_0)
	proposerAccessControllerAccount := cldf.NewTypeAndVersion(mcmscontracts.ProposerAccessControllerAccount, cldchangesetscommon.Version1_0_0)
	executorAccessControllerAccount := cldf.NewTypeAndVersion(mcmscontracts.ExecutorAccessControllerAccount, cldchangesetscommon.Version1_0_0)
	cancellerAccessControllerAccount := cldf.NewTypeAndVersion(mcmscontracts.CancellerAccessControllerAccount, cldchangesetscommon.Version1_0_0)
	bypasserAccessControllerAccount := cldf.NewTypeAndVersion(mcmscontracts.BypasserAccessControllerAccount, cldchangesetscommon.Version1_0_0)

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
			programID, seed, err := DecodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode timelock address (%s): %w", address, err)
			}
			state.TimelockProgram = programID
			state.TimelockSeed = seed

		case tvStr.Type == accessControllerProgram.Type && tvStr.Version.String() == accessControllerProgram.Version.String():
			programID, err := solana.PublicKeyFromBase58(address)
			if err != nil {
				return nil, fmt.Errorf("unable to parse public key from access controller address (%s): %w", address, err)
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
				return nil, fmt.Errorf("unable to parse public key from mcm address (%s): %w", address, err)
			}
			state.McmProgram = programID

		case tvStr.Type == proposerMCM.Type && tvStr.Version.String() == proposerMCM.Version.String():
			programID, seed, err := DecodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode proposer address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.ProposerMcmSeed = seed

		case tvStr.Type == bypasserMCM.Type && tvStr.Version.String() == bypasserMCM.Version.String():
			programID, seed, err := DecodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode bypasser address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.BypasserMcmSeed = seed

		case tvStr.Type == cancellerMCM.Type && tvStr.Version.String() == cancellerMCM.Version.String():
			programID, seed, err := DecodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode canceller address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.CancellerMcmSeed = seed
		}
	}

	return &state, nil
}

// Loads MCMSSolanaState from Datastore address refs
func MaybeLoadMCMSWithTimelockChainStateV2(refs []datastore.AddressRef) (*MCMSWithTimelockState, error) {
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
			programID, seed, err := DecodeAddressWithSeed(address)
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
			programID, seed, err := DecodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode proposer address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.ProposerMcmSeed = seed

		case bypasserMCM:
			programID, seed, err := DecodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode bypasser address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.BypasserMcmSeed = seed

		case cancellerMCM:
			programID, seed, err := DecodeAddressWithSeed(address)
			if err != nil {
				return nil, fmt.Errorf("unable to decode canceller address (%s): %w", address, err)
			}
			state.McmProgram = programID
			state.CancellerMcmSeed = seed
		}
	}

	return &state, nil
}

func EncodeAddressWithSeed(programID solana.PublicKey, seed PDASeed) string {
	return mcmssolanasdk.ContractAddress(programID, mcmssolanasdk.PDASeed(seed))
}

func DecodeAddressWithSeed(address string) (solana.PublicKey, PDASeed, error) {
	programID, seed, err := mcmssolanasdk.ParseContractAddress(address)
	if err != nil {
		return solana.PublicKey{}, PDASeed{}, fmt.Errorf("unable to parse address %q: %w", address, err)
	}

	return programID, PDASeed(seed), nil
}
