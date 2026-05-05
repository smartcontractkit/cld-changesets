package soltestutils

import (
	"testing"

	cldfsolana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/stretchr/testify/require"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/pkg/common"
	solstate "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
	solutils "github.com/smartcontractkit/cld-changesets/pkg/family/solana/utils"
)

// PreloadAddressBookWithMCMSPrograms creates and returns an address book containing preloaded MCMS
// Solana program addresses for the specified selector.
func PreloadAddressBookWithMCMSPrograms(t *testing.T, selector uint64) *cldf.AddressBookMap {
	t.Helper()

	ab := cldf.NewMemoryAddressBook()

	tv := cldf.NewTypeAndVersion(mcmscontracts.ManyChainMultisigProgram, common.Version1_0_0)
	err := ab.Save(selector, solutils.GetProgramID(solutils.ProgMCM), tv)
	require.NoError(t, err)

	tv = cldf.NewTypeAndVersion(mcmscontracts.AccessControllerProgram, common.Version1_0_0)
	err = ab.Save(selector, solutils.GetProgramID(solutils.ProgAccessController), tv)
	require.NoError(t, err)

	tv = cldf.NewTypeAndVersion(mcmscontracts.RBACTimelockProgram, common.Version1_0_0)
	err = ab.Save(selector, solutils.GetProgramID(solutils.ProgTimelock), tv)
	require.NoError(t, err)

	return ab
}

// GetMCMSStateFromAddressBook retrieves the state of the Solana MCMS contracts on the given chain.
func GetMCMSStateFromAddressBook(
	t *testing.T, ab cldf.AddressBook, chain cldfsolana.Chain,
) *solstate.MCMSWithTimelockState {
	t.Helper()

	addresses, err := ab.AddressesForChain(chain.Selector)
	require.NoError(t, err)

	mcmState, err := solstate.MaybeLoadMCMSWithTimelockChainState(chain, addresses)
	require.NoError(t, err)

	return mcmState
}
