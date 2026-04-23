package evm

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
)

// MCMSWithTimelockState holds the Go bindings
// for a MCMSWithTimelock contract deployment.
// It is public for use in product specific packages.
// Either all fields are nil or all fields are non-nil.
type MCMSWithTimelockState struct {
	CancellerMcm *bindings.ManyChainMultiSig
	BypasserMcm  *bindings.ManyChainMultiSig
	ProposerMcm  *bindings.ManyChainMultiSig
	Timelock     *bindings.RBACTimelock
	CallProxy    *bindings.CallProxy
}

// MaybeLoadMCMSWithTimelockStateWithQualifier loads the MCMSWithTimelockState state for each chain in the given environment,
// supporting qualifiers for filtering addresses from env.DataStore.
func MaybeLoadMCMSWithTimelockStateWithQualifier(env cldf.Environment, chainSelectors []uint64, qualifier string) (map[uint64]*MCMSWithTimelockState, error) {
	result := map[uint64]*MCMSWithTimelockState{}
	for _, chainSelector := range chainSelectors {
		chain, ok := env.BlockChains.EVMChains()[chainSelector]
		if !ok {
			return nil, fmt.Errorf("chain %d not found", chainSelector)
		}

		addressesChain, err := GetAddressTypeVersionByQualifier(env.DataStore.Addresses(), chainSelector, qualifier)
		if err != nil {
			return nil, err
		}

		state, err := MaybeLoadMCMSWithTimelockChainState(chain, addressesChain)
		if err != nil {
			return nil, err
		}
		result[chainSelector] = state
	}

	return result, nil
}

// should we allow user to pass version as a parameter?
var version1_0_0 = *semver.MustParse("1.0.0")

// MaybeLoadMCMSWithTimelockChainState looks for the addresses corresponding to
// contracts deployed with DeployMCMSWithTimelock and loads them into a
// MCMSWithTimelockState struct. If none of the contracts are found, it returns
// a non-nil state struct whose binding fields are all nil.
// An error indicates:
// - Found but was unable to load a contract
// - It only found part of the bundle of contracts
// - If found more than one instance of a contract (we expect one bundle in the given addresses)
func MaybeLoadMCMSWithTimelockChainState(chain cldf_evm.Chain, addresses map[string]cldf.TypeAndVersion) (*MCMSWithTimelockState, error) {
	state := MCMSWithTimelockState{}
	var (
		// We expect one of each contract on the chain.
		timelock  = cldf.NewTypeAndVersion(mcmscontracts.RBACTimelock, version1_0_0)
		callProxy = cldf.NewTypeAndVersion(mcmscontracts.CallProxy, version1_0_0)
		proposer  = cldf.NewTypeAndVersion(mcmscontracts.ProposerManyChainMultisig, version1_0_0)
		canceller = cldf.NewTypeAndVersion(mcmscontracts.CancellerManyChainMultisig, version1_0_0)
		bypasser  = cldf.NewTypeAndVersion(mcmscontracts.BypasserManyChainMultisig, version1_0_0)

		// the same contract can have different roles
		multichain    = cldf.NewTypeAndVersion(mcmscontracts.ManyChainMultisig, version1_0_0)
		proposerMCMS  = cldf.NewTypeAndVersion(mcmscontracts.ManyChainMultisig, version1_0_0)
		bypasserMCMS  = cldf.NewTypeAndVersion(mcmscontracts.ManyChainMultisig, version1_0_0)
		cancellerMCMS = cldf.NewTypeAndVersion(mcmscontracts.ManyChainMultisig, version1_0_0)
	)

	// Add role labels to the ManyChainMultisig variants and build the expected contract bundle.
	proposerMCMS.Labels.Add(mcmscontracts.ProposerRole.String())
	bypasserMCMS.Labels.Add(mcmscontracts.BypasserRole.String())
	cancellerMCMS.Labels.Add(mcmscontracts.CancellerRole.String())
	wantTypes := []cldf.TypeAndVersion{timelock, proposer, canceller, bypasser, callProxy,
		proposerMCMS, bypasserMCMS, cancellerMCMS,
	}

	// Ensure we either have the bundle or not.
	_, err := cldf.EnsureDeduped(addresses, wantTypes)
	if err != nil {
		return nil, fmt.Errorf("unable to check MCMS contracts on chain %s error: %w", chain.Name(), err)
	}

	for address, tv := range addresses {
		switch {
		case tv.Type == timelock.Type && tv.Version.String() == timelock.Version.String():
			addr, err := evmContractAddr(chain, address, tv)
			if err != nil {
				return nil, err
			}
			tl, err := bindings.NewRBACTimelock(addr, chain.Client)
			if err != nil {
				return nil, fmt.Errorf("failed to bind %s on chain %s (selector=%d) at address %q: %w",
					tv.Type, chain.Name(), chain.Selector, address, err)
			}
			state.Timelock = tl
		case tv.Type == callProxy.Type && tv.Version.String() == callProxy.Version.String():
			addr, err := evmContractAddr(chain, address, tv)
			if err != nil {
				return nil, err
			}
			cp, err := bindings.NewCallProxy(addr, chain.Client)
			if err != nil {
				return nil, fmt.Errorf("failed to bind %s on chain %s (selector=%d) at address %q: %w",
					tv.Type, chain.Name(), chain.Selector, address, err)
			}
			state.CallProxy = cp
		case tv.Type == proposer.Type && tv.Version.String() == proposer.Version.String():
			addr, err := evmContractAddr(chain, address, tv)
			if err != nil {
				return nil, err
			}
			mcms, err := bindings.NewManyChainMultiSig(addr, chain.Client)
			if err != nil {
				return nil, fmt.Errorf("failed to bind %s on chain %s (selector=%d) at address %q: %w",
					tv.Type, chain.Name(), chain.Selector, address, err)
			}
			state.ProposerMcm = mcms
		case tv.Type == bypasser.Type && tv.Version.String() == bypasser.Version.String():
			addr, err := evmContractAddr(chain, address, tv)
			if err != nil {
				return nil, err
			}
			mcms, err := bindings.NewManyChainMultiSig(addr, chain.Client)
			if err != nil {
				return nil, fmt.Errorf("failed to bind %s on chain %s (selector=%d) at address %q: %w",
					tv.Type, chain.Name(), chain.Selector, address, err)
			}
			state.BypasserMcm = mcms
		case tv.Type == canceller.Type && tv.Version.String() == canceller.Version.String():
			addr, err := evmContractAddr(chain, address, tv)
			if err != nil {
				return nil, err
			}
			mcms, err := bindings.NewManyChainMultiSig(addr, chain.Client)
			if err != nil {
				return nil, fmt.Errorf("failed to bind %s on chain %s (selector=%d) at address %q: %w",
					tv.Type, chain.Name(), chain.Selector, address, err)
			}
			state.CancellerMcm = mcms
		case tv.Type == multichain.Type && tv.Version.String() == multichain.Version.String():
			// Contract of type ManyChainMultiSig must be labeled to assign to the proper state
			// field.  If a specifically typed contract already occupies the field, then this
			// contract will be ignored.
			addr, err := evmContractAddr(chain, address, tv)
			if err != nil {
				return nil, err
			}
			mcms, err := bindings.NewManyChainMultiSig(addr, chain.Client)
			if err != nil {
				return nil, fmt.Errorf("failed to bind %s on chain %s (selector=%d) at address %q: %w",
					tv.Type, chain.Name(), chain.Selector, address, err)
			}
			if tv.Labels.Contains(mcmscontracts.ProposerRole.String()) && state.ProposerMcm == nil {
				state.ProposerMcm = mcms
			}
			if tv.Labels.Contains(mcmscontracts.BypasserRole.String()) && state.BypasserMcm == nil {
				state.BypasserMcm = mcms
			}
			if tv.Labels.Contains(mcmscontracts.CancellerRole.String()) && state.CancellerMcm == nil {
				state.CancellerMcm = mcms
			}
		}
	}

	return &state, nil
}

// GetAddressTypeVersionByQualifier loads addresses from DataStore for a specific chain and qualifier.
// It returns a map of address to TypeAndVersion. Refs with a nil Version are skipped; if none remain,
// it returns an error. Each address must be a non-zero hex-encoded EVM address (see common.IsHexAddress).
func GetAddressTypeVersionByQualifier(store datastore.AddressRefStore, chainSelector uint64, qualifier string) (map[string]cldf.TypeAndVersion, error) {
	addressesChain := make(map[string]cldf.TypeAndVersion)

	// Build filter list starting with chain selector
	filters := []datastore.FilterFunc[datastore.AddressRefKey, datastore.AddressRef]{datastore.AddressRefByChainSelector(chainSelector)}

	// Add qualifier filter if provided
	if qualifier != "" {
		filters = append(filters, datastore.AddressRefByQualifier(qualifier))
	}

	addresses := store.Filter(filters...)
	if len(addresses) == 0 {
		if qualifier != "" {
			return nil, fmt.Errorf("no addresses found for chain %d with qualifier %q", chainSelector, qualifier)
		}

		return nil, fmt.Errorf("no addresses found for chain %d", chainSelector)
	}

	for _, addressRef := range addresses {
		if addressRef.Version == nil {
			continue
		}
		if _, err := parseValidatedEVMAddress(addressRef.Address); err != nil {
			return nil, fmt.Errorf("datastore address ref for chain %d type=%s version=%s: %w",
				chainSelector, addressRef.Type, addressRef.Version.String(), err)
		}
		tv := cldf.TypeAndVersion{
			Type:    cldf.ContractType(addressRef.Type),
			Version: *addressRef.Version,
		}
		// Preserve labels from DataStore
		if !addressRef.Labels.IsEmpty() {
			tv.Labels = cldf.NewLabelSet(addressRef.Labels.List()...)
		}
		addressesChain[addressRef.Address] = tv
	}

	if len(addressesChain) == 0 {
		return nil, fmt.Errorf("no address refs with a non-nil contract version for chain %d", chainSelector)
	}

	return addressesChain, nil
}

func parseValidatedEVMAddress(raw string) (common.Address, error) {
	if !common.IsHexAddress(raw) {
		return common.Address{}, fmt.Errorf("not a valid hex-encoded EVM address: %q", raw)
	}
	addr := common.HexToAddress(raw)
	if addr == (common.Address{}) {
		return common.Address{}, fmt.Errorf("EVM address must not be the zero address: %q", raw)
	}

	return addr, nil
}

func evmContractAddr(chain cldf_evm.Chain, raw string, tv cldf.TypeAndVersion) (common.Address, error) {
	addr, err := parseValidatedEVMAddress(raw)
	if err != nil {
		return common.Address{}, fmt.Errorf("chain %s (selector=%d) contract %s %s address %q: %w",
			chain.Name(), chain.Selector, tv.Type, tv.Version.String(), raw, err)
	}

	return addr, nil
}
