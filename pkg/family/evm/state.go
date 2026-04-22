package state

import (
	"errors"
	"fmt"
	"maps"

	"github.com/ethereum/go-ethereum/common"
	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	cldflink "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/link"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/generated/link_token_interface"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"

	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	linkview "github.com/smartcontractkit/cld-changesets/link/view"

	"github.com/smartcontractkit/cld-changesets/mcms/common/view/v1_0"
	common2 "github.com/smartcontractkit/cld-changesets/pkg/common"
)

// MCMSWithTimelockState holds the Go bindings
// for a MCMSWithTimelock contract mcms.
// It is public for use in product specific packages.
// Either all fields are nil or all fields are non-nil.
type MCMSWithTimelockState struct {
	CancellerMcm *bindings.ManyChainMultiSig
	BypasserMcm  *bindings.ManyChainMultiSig
	ProposerMcm  *bindings.ManyChainMultiSig
	Timelock     *bindings.RBACTimelock
	CallProxy    *bindings.CallProxy
}

// Validate checks that all fields are non-nil, ensuring it's ready
// for use generating views or interactions.
func (state MCMSWithTimelockState) Validate() error {
	if state.Timelock == nil {
		return errors.New("timelock not found")
	}
	if state.CancellerMcm == nil {
		return errors.New("canceller not found")
	}
	if state.ProposerMcm == nil {
		return errors.New("proposer not found")
	}
	if state.BypasserMcm == nil {
		return errors.New("bypasser not found")
	}
	if state.CallProxy == nil {
		return errors.New("call proxy not found")
	}
	return nil
}

func (state MCMSWithTimelockState) GenerateMCMSWithTimelockView() (v1_0.MCMSWithTimelockView, error) {
	if err := state.Validate(); err != nil {
		return v1_0.MCMSWithTimelockView{}, fmt.Errorf("unable to validate McmsWithTimelock state: %w", err)
	}

	return v1_0.GenerateMCMSWithTimelockView(*state.BypasserMcm, *state.CancellerMcm, *state.ProposerMcm,
		*state.Timelock, *state.CallProxy)
}

// AddressesForChain combines addresses from both DataStore and AddressBook making it backward compatible.
// This version supports qualifiers for filtering DataStore addresses.
// When a qualifier is specified, only DataStore addresses with that qualifier are returned (no AddressBook merge)
// to ensure isolation between different deployments.
func AddressesForChain(env cldf.Environment, chainSelector uint64, qualifier string) (map[string]cldf.TypeAndVersion, error) {
	// If a qualifier is specified, only use DataStore to ensure isolation between deployments
	if qualifier != "" {
		if env.DataStore != nil {
			return LoadAddressesFromDataStore(env.DataStore, chainSelector, qualifier)
		}
		return nil, fmt.Errorf("DataStore not available but qualifier %s specified", qualifier)
	}

	// For backward compatibility without qualifier, merge both sources
	// Start with addresses from AddressBook
	addressBookAddresses := make(map[string]cldf.TypeAndVersion)
	if addresses, err := env.ExistingAddresses.AddressesForChain(chainSelector); err == nil {
		addressBookAddresses = addresses
	} else if !errors.Is(err, cldf.ErrChainNotFound) {
		return nil, fmt.Errorf("failed to load addresses from AddressBook: %w", err)
	}

	// If no DataStore, just return AddressBook addresses
	if env.DataStore == nil {
		return addressBookAddresses, nil
	}

	// Try to load addresses from DataStore (without qualifier for general case)
	dataStoreAddresses, err := LoadAddressesFromDataStore(env.DataStore, chainSelector, "")
	if err != nil {
		// If DataStore has no addresses or returns an error, fall back to AddressBook addresses only
		return addressBookAddresses, nil
	}

	// Merge the two maps - DataStore addresses take precedence
	mergedAddresses := make(map[string]cldf.TypeAndVersion)

	// First add all AddressBook addresses
	maps.Copy(mergedAddresses, addressBookAddresses)

	// Then add DataStore addresses (overwriting any conflicts)
	maps.Copy(mergedAddresses, dataStoreAddresses)

	return mergedAddresses, nil
}

// MaybeLoadMCMSWithTimelockStateDataStore loads the MCMSWithTimelockState state for each chain in the given environment from the DataStore.
func MaybeLoadMCMSWithTimelockStateDataStore(env cldf.Environment, chainSelectors []uint64) (map[uint64]*MCMSWithTimelockState, error) {
	return MaybeLoadMCMSWithTimelockStateDataStoreWithQualifier(env, chainSelectors, "")
}

func MaybeLoadMCMSWithTimelockStateDataStoreWithQualifier(env cldf.Environment, chainSelectors []uint64, qualifier string) (map[uint64]*MCMSWithTimelockState, error) {
	result := map[uint64]*MCMSWithTimelockState{}
	ds := env.DataStore
	if ds == nil {
		return nil, fmt.Errorf("datastore not available")
	}
	for _, chainSelector := range chainSelectors {
		chain, ok := env.BlockChains.EVMChains()[chainSelector]
		if !ok {
			return nil, fmt.Errorf("chain %d not found", chainSelector)
		}
		state, err := GetMCMSWithTimelockState(ds.Addresses(), chain, qualifier)
		if err != nil {
			return nil, fmt.Errorf("failed to get MCMSWithTimelock state for chain %d, qualifier %s: %w", chainSelector, qualifier, err)
		}
		result[chainSelector] = state
	}
	return result, nil
}

// GetMCMSWithTimelockState loads the MCMSWithTimelockState for a specific chain and qualifier from the DataStore.
// It filters AddressRefs to avoid key collisions that occur when multiple contract types share the same address (e.g. bypasser and canceller).
func GetMCMSWithTimelockState(store datastore.AddressRefStore, chain cldf_evm.Chain, qualifier string) (*MCMSWithTimelockState, error) {
	filters := []datastore.FilterFunc[datastore.AddressRefKey, datastore.AddressRef]{datastore.AddressRefByChainSelector(chain.Selector)}
	if qualifier != "" {
		filters = append(filters, datastore.AddressRefByQualifier(qualifier))
	}

	refs := store.Filter(filters...)
	if len(refs) == 0 {
		return nil, fmt.Errorf("no addresses found for chain %d", chain.Selector)
	}

	return MaybeLoadMCMSWithTimelockChainStateFromRefs(chain, refs)
}

// LoadAddressesFromDataStore loads addresses from DataStore with optional qualifier.
// This is a public utility function that can be used by other packages to avoid duplication.
//
// Deprecated: Use GetAddressTypeVersionByQualifier instead.
func LoadAddressesFromDataStore(ds datastore.DataStore, chainSelector uint64, qualifier string) (map[string]cldf.TypeAndVersion, error) {
	addressesChain, err := GetAddressTypeVersionByQualifier(ds.Addresses(), chainSelector, qualifier)
	if err != nil {
		return nil, err
	}
	return addressesChain, nil
}

// MaybeLoadMCMSWithTimelockChainStateFromRefs is the DataStore-native equivalent of MaybeLoadMCMSWithTimelockChainState.
// It accepts []datastore.AddressRef directly, to preserve entries when multiple contract types share the same address (e.g. bypasser and canceller).
func MaybeLoadMCMSWithTimelockChainStateFromRefs(chain cldf_evm.Chain, refs []datastore.AddressRef) (*MCMSWithTimelockState, error) {
	state := MCMSWithTimelockState{}
	var (
		// We expect one of each contract on the chain.
		timelock  = cldf.NewTypeAndVersion(proposalutils.RBACTimelock, common2.Version1_0_0)
		callProxy = cldf.NewTypeAndVersion(proposalutils.CallProxy, common2.Version1_0_0)
		proposer  = cldf.NewTypeAndVersion(proposalutils.ProposerManyChainMultisig, common2.Version1_0_0)
		canceller = cldf.NewTypeAndVersion(proposalutils.CancellerManyChainMultisig, common2.Version1_0_0)
		bypasser  = cldf.NewTypeAndVersion(proposalutils.BypasserManyChainMultisig, common2.Version1_0_0)
	)

	wantTypes := []cldf.TypeAndVersion{timelock, proposer, canceller, bypasser, callProxy}

	dedupMap := make(map[string]cldf.TypeAndVersion, len(refs))
	for _, ref := range refs {
		tv := cldf.TypeAndVersion{
			Type:    cldf.ContractType(ref.Type),
			Version: *ref.Version,
		}
		if !ref.Labels.IsEmpty() {
			tv.Labels = cldf.NewLabelSet(ref.Labels.List()...)
		}
		dedupMap[ref.Key().String()] = tv
	}

	// Ensure we either have the bundle or not.
	_, err := cldf.EnsureDeduped(dedupMap, wantTypes)
	if err != nil {
		return nil, fmt.Errorf("unable to check MCMS contracts on chain %s error: %w", chain.Name(), err)
	}

	for _, ref := range refs {
		addr := common.HexToAddress(ref.Address)
		tv := cldf.TypeAndVersion{
			Type:    cldf.ContractType(ref.Type),
			Version: *ref.Version,
		}

		switch {
		case tv.Type == timelock.Type && tv.Version.String() == timelock.Version.String():
			tl, err := bindings.NewRBACTimelock(addr, chain.Client)
			if err != nil {
				return nil, err
			}
			state.Timelock = tl
		case tv.Type == callProxy.Type && tv.Version.String() == callProxy.Version.String():
			cp, err := bindings.NewCallProxy(addr, chain.Client)
			if err != nil {
				return nil, err
			}
			state.CallProxy = cp
		case tv.Type == proposer.Type && tv.Version.String() == proposer.Version.String():
			mcms, err := bindings.NewManyChainMultiSig(addr, chain.Client)
			if err != nil {
				return nil, err
			}
			state.ProposerMcm = mcms
		case tv.Type == bypasser.Type && tv.Version.String() == bypasser.Version.String():
			mcms, err := bindings.NewManyChainMultiSig(addr, chain.Client)
			if err != nil {
				return nil, err
			}
			state.BypasserMcm = mcms
		case tv.Type == canceller.Type && tv.Version.String() == canceller.Version.String():
			mcms, err := bindings.NewManyChainMultiSig(addr, chain.Client)
			if err != nil {
				return nil, err
			}
			state.CancellerMcm = mcms
		}
	}
	return &state, nil
}

type LinkTokenState struct {
	LinkToken *link_token.LinkToken
}

func (s LinkTokenState) GenerateLinkView() (linkview.LinkTokenView, error) {
	if s.LinkToken == nil {
		return linkview.LinkTokenView{}, errors.New("link token not found")
	}
	return linkview.GenerateLinkTokenView(s.LinkToken)
}

func MaybeLoadLinkTokenChainState(chain cldf_evm.Chain, addresses map[string]cldf.TypeAndVersion) (*LinkTokenState, error) {
	state := LinkTokenState{}
	linkToken := cldf.NewTypeAndVersion(cldflink.LinkToken, common2.Version1_0_0)

	// Convert map keys to a slice
	wantTypes := []cldf.TypeAndVersion{linkToken}

	// Ensure we either have the bundle or not.
	_, err := cldf.EnsureDeduped(addresses, wantTypes)
	if err != nil {
		return nil, fmt.Errorf("unable to check link token on chain %s error: %w", chain.Name(), err)
	}

	for address, tvStr := range addresses {
		if tvStr.Type == linkToken.Type && tvStr.Version.String() == linkToken.Version.String() {
			lt, err := link_token.NewLinkToken(common.HexToAddress(address), chain.Client)
			if err != nil {
				return nil, err
			}
			state.LinkToken = lt
		}
	}
	return &state, nil
}

type StaticLinkTokenState struct {
	StaticLinkToken *link_token_interface.LinkToken
}

func (s StaticLinkTokenState) GenerateStaticLinkView() (linkview.StaticLinkTokenView, error) {
	if s.StaticLinkToken == nil {
		return linkview.StaticLinkTokenView{}, errors.New("static link token not found")
	}
	return linkview.GenerateStaticLinkTokenView(s.StaticLinkToken)
}

func MaybeLoadStaticLinkTokenState(chain cldf_evm.Chain, addresses map[string]cldf.TypeAndVersion) (*StaticLinkTokenState, error) {
	state := StaticLinkTokenState{}
	staticLinkToken := cldf.NewTypeAndVersion(cldflink.StaticLinkToken, common2.Version1_0_0)

	// Convert map keys to a slice
	wantTypes := []cldf.TypeAndVersion{staticLinkToken}

	// Ensure we either have the bundle or not.
	_, err := cldf.EnsureDeduped(addresses, wantTypes)
	if err != nil {
		return nil, fmt.Errorf("unable to check static link token on chain %s error: %w", chain.Name(), err)
	}

	for address, tvStr := range addresses {
		if tvStr.Type == staticLinkToken.Type && tvStr.Version.String() == staticLinkToken.Version.String() {
			lt, err := link_token_interface.NewLinkToken(common.HexToAddress(address), chain.Client)
			if err != nil {
				return nil, err
			}
			state.StaticLinkToken = lt
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
