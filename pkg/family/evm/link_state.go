package evm

import (
	"errors"
	"fmt"

	"github.com/smartcontractkit/chainlink-evm/gethwrappers/generated/link_token_interface"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"

	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"

	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
	v1_0 "github.com/smartcontractkit/cld-changesets/pkg/contract/link/view/v1_0"
)

type LinkTokenState struct {
	LinkToken *link_token.LinkToken
}

// GenerateLinkView generates the LinkTokenView for the LinkTokenState.
func (s LinkTokenState) GenerateLinkView() (v1_0.LinkTokenView, error) {
	if s.LinkToken == nil {
		return v1_0.LinkTokenView{}, errors.New("link token not found")
	}

	return v1_0.GenerateLinkTokenView(s.LinkToken)
}

// MaybeLoadLinkTokenChainState loads the LinkTokenState for the given chain and addresses.
func MaybeLoadLinkTokenChainState(chain cldf_evm.Chain, addresses map[string]cldf.TypeAndVersion) (*LinkTokenState, error) {
	state := LinkTokenState{}
	// todo(ggoh): version should be configurable?
	linkToken := cldf.NewTypeAndVersion(linkcontracts.LinkToken, cldchangesetscommon.Version1_0_0)

	wantTypes := []cldf.TypeAndVersion{linkToken}

	// Ensure we either have the bundle or not.
	_, err := cldf.EnsureDeduped(addresses, wantTypes)
	if err != nil {
		return nil, fmt.Errorf("unable to check link token on chain %s error: %w", chain.Name(), err)
	}

	for address, tv := range addresses {
		if tv.Type == linkToken.Type && tv.Version.String() == linkToken.Version.String() {
			addr, err := evmContractAddr(chain, address, tv)
			if err != nil {
				return nil, err
			}
			lt, err := link_token.NewLinkToken(addr, chain.Client)
			if err != nil {
				return nil, fmt.Errorf("failed to bind %s on chain %s (selector=%d) at address %q: %w",
					tv.Type, chain.Name(), chain.Selector, address, err)
			}
			state.LinkToken = lt
		}
	}

	// todo(ggoh): should return error when link token is not found?
	return &state, nil
}

type StaticLinkTokenState struct {
	StaticLinkToken *link_token_interface.LinkToken
}

// GenerateStaticLinkView generates the StaticLinkTokenView for the StaticLinkTokenState.
func (s StaticLinkTokenState) GenerateStaticLinkView() (v1_0.StaticLinkTokenView, error) {
	if s.StaticLinkToken == nil {
		return v1_0.StaticLinkTokenView{}, errors.New("static link token not found")
	}

	return v1_0.GenerateStaticLinkTokenView(s.StaticLinkToken)
}

// MaybeLoadStaticLinkTokenState loads the StaticLinkTokenState for the given chain and addresses.
func MaybeLoadStaticLinkTokenState(chain cldf_evm.Chain, addresses map[string]cldf.TypeAndVersion) (*StaticLinkTokenState, error) {
	state := StaticLinkTokenState{}
	// todo(ggoh): version should be configurable?
	staticLinkToken := cldf.NewTypeAndVersion(linkcontracts.StaticLinkToken, cldchangesetscommon.Version1_0_0)

	wantTypes := []cldf.TypeAndVersion{staticLinkToken}

	// Ensure we either have the bundle or not.
	_, err := cldf.EnsureDeduped(addresses, wantTypes)
	if err != nil {
		return nil, fmt.Errorf("unable to check static link token on chain %s error: %w", chain.Name(), err)
	}

	for address, tv := range addresses {
		if tv.Type == staticLinkToken.Type && tv.Version.String() == staticLinkToken.Version.String() {
			addr, err := evmContractAddr(chain, address, tv)
			if err != nil {
				return nil, err
			}
			lt, err := link_token_interface.NewLinkToken(addr, chain.Client)
			if err != nil {
				return nil, fmt.Errorf("failed to bind %s on chain %s (selector=%d) at address %q: %w",
					tv.Type, chain.Name(), chain.Selector, address, err)
			}
			state.StaticLinkToken = lt
		}
	}

	// todo(ggoh): should return error when link token is not found?
	return &state, nil
}
