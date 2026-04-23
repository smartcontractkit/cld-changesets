package evm

import (
	"fmt"
	"testing"

	chainsel "github.com/smartcontractkit/chain-selectors"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	"github.com/stretchr/testify/require"

	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
)

func TestLinkTokenState_GenerateLinkView(t *testing.T) {
	t.Parallel()
	t.Run("nil binding", func(t *testing.T) {
		t.Parallel()
		_, err := LinkTokenState{}.GenerateLinkView()
		require.ErrorContains(t, err, "link token not found")
	})
}

func TestStaticLinkTokenState_GenerateStaticLinkView(t *testing.T) {
	t.Parallel()
	t.Run("nil binding", func(t *testing.T) {
		t.Parallel()
		_, err := StaticLinkTokenState{}.GenerateStaticLinkView()
		require.ErrorContains(t, err, "static link token not found")
	})
}

func TestMaybeLoadLinkTokenChainState(t *testing.T) {
	t.Parallel()
	chain := testSepoliaChain(t)
	linkTV := cldf.NewTypeAndVersion(linkcontracts.LinkToken, cldchangesetscommon.Version1_0_0)

	t.Run("empty addresses map returns non-nil state with nil LinkToken", func(t *testing.T) {
		t.Parallel()
		got, err := MaybeLoadLinkTokenChainState(chain, map[string]cldf.TypeAndVersion{})
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Nil(t, got.LinkToken)
	})

	t.Run("nil addresses map returns non-nil state with nil LinkToken", func(t *testing.T) {
		t.Parallel()
		got, err := MaybeLoadLinkTokenChainState(chain, nil)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Nil(t, got.LinkToken)
	})

	t.Run("duplicate link token addresses returns wrapped error", func(t *testing.T) {
		t.Parallel()
		addrs := map[string]cldf.TypeAndVersion{
			"0x0000000000000000000000000000000000000001": linkTV,
			"0x0000000000000000000000000000000000000002": linkTV,
		}
		_, err := MaybeLoadLinkTokenChainState(chain, addrs)
		require.ErrorContains(t, err, fmt.Sprintf(
			"unable to check link token on chain %s error: found more than one instance of contract",
			chain.Name()))
	})

	t.Run("no matching link token version leaves binding nil", func(t *testing.T) {
		t.Parallel()
		addrs := map[string]cldf.TypeAndVersion{
			"0x0000000000000000000000000000000000000001": cldf.NewTypeAndVersion(linkcontracts.LinkToken, cldchangesetscommon.Version1_1_0),
		}
		got, err := MaybeLoadLinkTokenChainState(chain, addrs)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Nil(t, got.LinkToken)
	})

	t.Run("invalid link token address returns error", func(t *testing.T) {
		t.Parallel()
		addrs := map[string]cldf.TypeAndVersion{
			"not-a-valid-hex-addr": linkTV,
		}
		_, err := MaybeLoadLinkTokenChainState(chain, addrs)
		require.ErrorContains(t, err, fmt.Sprintf(
			"chain %s (selector=%d) contract %s %s address \"not-a-valid-hex-addr\": not a valid hex-encoded EVM address",
			chain.Name(), chain.Selector, linkTV.Type, linkTV.Version.String()))
	})

	t.Run("zero link token address returns error", func(t *testing.T) {
		t.Parallel()
		addrs := map[string]cldf.TypeAndVersion{
			"0x0000000000000000000000000000000000000000": linkTV,
		}
		_, err := MaybeLoadLinkTokenChainState(chain, addrs)
		require.ErrorContains(t, err, fmt.Sprintf(
			"chain %s (selector=%d) contract %s %s address \"0x0000000000000000000000000000000000000000\": EVM address must not be the zero address",
			chain.Name(), chain.Selector, linkTV.Type, linkTV.Version.String()))
	})
}

func TestMaybeLoadStaticLinkTokenState(t *testing.T) {
	t.Parallel()
	chain := testSepoliaChain(t)
	staticTV := cldf.NewTypeAndVersion(linkcontracts.StaticLinkToken, cldchangesetscommon.Version1_0_0)

	t.Run("empty addresses map returns non-nil state with nil StaticLinkToken", func(t *testing.T) {
		t.Parallel()
		got, err := MaybeLoadStaticLinkTokenState(chain, map[string]cldf.TypeAndVersion{})
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Nil(t, got.StaticLinkToken)
	})

	t.Run("nil addresses map returns non-nil state with nil StaticLinkToken", func(t *testing.T) {
		t.Parallel()
		got, err := MaybeLoadStaticLinkTokenState(chain, nil)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Nil(t, got.StaticLinkToken)
	})

	t.Run("duplicate static link token addresses returns wrapped error", func(t *testing.T) {
		t.Parallel()
		addrs := map[string]cldf.TypeAndVersion{
			"0x0000000000000000000000000000000000000001": staticTV,
			"0x0000000000000000000000000000000000000002": staticTV,
		}
		_, err := MaybeLoadStaticLinkTokenState(chain, addrs)
		require.ErrorContains(t, err, fmt.Sprintf(
			"unable to check static link token on chain %s error: found more than one instance of contract",
			chain.Name()))
	})

	t.Run("no matching static link token version leaves binding nil", func(t *testing.T) {
		t.Parallel()
		addrs := map[string]cldf.TypeAndVersion{
			"0x0000000000000000000000000000000000000001": cldf.NewTypeAndVersion(linkcontracts.StaticLinkToken, cldchangesetscommon.Version1_1_0),
		}
		got, err := MaybeLoadStaticLinkTokenState(chain, addrs)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Nil(t, got.StaticLinkToken)
	})

	t.Run("invalid static link token address returns error", func(t *testing.T) {
		t.Parallel()
		addrs := map[string]cldf.TypeAndVersion{
			"not-a-valid-hex-addr": staticTV,
		}
		_, err := MaybeLoadStaticLinkTokenState(chain, addrs)
		require.ErrorContains(t, err, fmt.Sprintf(
			"chain %s (selector=%d) contract %s %s address \"not-a-valid-hex-addr\": not a valid hex-encoded EVM address",
			chain.Name(), chain.Selector, staticTV.Type, staticTV.Version.String()))
	})

	t.Run("zero static link token address returns error", func(t *testing.T) {
		t.Parallel()
		addrs := map[string]cldf.TypeAndVersion{
			"0x0000000000000000000000000000000000000000": staticTV,
		}
		_, err := MaybeLoadStaticLinkTokenState(chain, addrs)
		require.ErrorContains(t, err, fmt.Sprintf(
			"chain %s (selector=%d) contract %s %s address \"0x0000000000000000000000000000000000000000\": EVM address must not be the zero address",
			chain.Name(), chain.Selector, staticTV.Type, staticTV.Version.String()))
	})
}

func testSepoliaChain(t *testing.T) cldf_evm.Chain {
	t.Helper()
	return cldf_evm.Chain{Selector: chainsel.ETHEREUM_TESTNET_SEPOLIA.Selector}
}
