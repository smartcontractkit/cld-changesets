package changeset

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
)

func MustFundAddressWithLink(t *testing.T, e cldf.Environment, chain cldf_evm.Chain, to common.Address, amount int64) {
	t.Helper()

	addresses, err := e.ExistingAddresses.AddressesForChain(chain.Selector) //nolint:staticcheck // legacy helper still loads LINK from AddressBook
	require.NoError(t, err)

	linkState, err := evmstate.MaybeLoadLinkTokenChainState(chain, addresses)
	require.NoError(t, err)
	require.NotNil(t, linkState.LinkToken)

	// grant minter permissions - only owner can call this function
	e.Logger.Info("granting minter permissions for chain", chain.DeployerKey)
	tx, err := linkState.LinkToken.GrantMintRole(chain.DeployerKey, chain.DeployerKey.From)
	require.NoError(t, err)
	_, err = cldf.ConfirmIfNoError(chain, tx, err)
	require.NoError(t, err)

	// Mint 'To' address some tokens
	tx, err = linkState.LinkToken.Mint(chain.DeployerKey, to, big.NewInt(amount))
	require.NoError(t, err)
	_, err = cldf.ConfirmIfNoError(chain, tx, err)
	require.NoError(t, err)

	// 'To' address should have the tokens
	ctx := e.GetContext()
	endBalance, err := linkState.LinkToken.BalanceOf(&bind.CallOpts{Context: ctx}, to)
	require.NoError(t, err)
	expectedBalance := big.NewInt(amount)
	require.Equal(t, expectedBalance, endBalance)
}

// MaybeGetLinkBalance returns the LINK balance of the given address on the given chain.
func MaybeGetLinkBalance(t *testing.T, e cldf.Environment, chain cldf_evm.Chain, linkAddr common.Address) *big.Int {
	t.Helper()

	addresses, err := e.ExistingAddresses.AddressesForChain(chain.Selector) //nolint:staticcheck // legacy helper still loads LINK from AddressBook
	require.NoError(t, err)
	linkState, err := evmstate.MaybeLoadLinkTokenChainState(chain, addresses)
	require.NoError(t, err)
	endBalance, err := linkState.LinkToken.BalanceOf(&bind.CallOpts{Context: chain.DeployerKey.Context}, linkAddr)
	require.NoError(t, err)

	return endBalance
}
