package evmgrantrole

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	mcmsevm "github.com/smartcontractkit/mcms/sdk/evm"
	"github.com/stretchr/testify/require"

	mcmssdk "github.com/smartcontractkit/mcms/sdk"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
)

func TestAddressesNeedingGrant(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	rt := newEVMGrantRoleRuntime(t, selector)
	refs := grantRoleRefsFromEnv(t, rt.Environment(), selector)
	bundle := rt.Environment().OperationsBundle
	chain := rt.Environment().BlockChains.EVMChains()[selector]
	deps := grantrole.Deps{
		BlockChains: rt.Environment().BlockChains,
		DataStore:   rt.Environment().DataStore,
	}

	grantee := "0x00000000000000000000000000000000000000dd"
	pending := "0x00000000000000000000000000000000000000ee"
	_, err := runEVMGrantRole(bundle, deps, grantrole.SeqInput{
		ChainSelector: selector,
		Grants: []grantrole.RoleGrant{{
			Role:      mcmssdk.TimelockRoleCanceller,
			Addresses: []string{grantee},
		}},
	})
	require.NoError(t, err)

	needed, err := AddressesNeedingGrant(
		t.Context(),
		mcmsevm.NewTimelockInspector(chain.Client),
		refs.Timelock,
		grantrole.RoleGrant{
			Role:      mcmssdk.TimelockRoleCanceller,
			Addresses: []string{grantee, pending},
		},
	)
	require.NoError(t, err)
	require.Equal(t, []common.Address{common.HexToAddress(pending)}, needed)

	adminNeeded, err := AddressesNeedingGrant(
		t.Context(),
		mcmsevm.NewTimelockInspector(chain.Client),
		refs.Timelock,
		grantrole.RoleGrant{
			Role:      mcmssdk.TimelockRoleAdmin,
			Addresses: []string{grantee},
		},
	)
	require.NoError(t, err)
	require.Equal(t, []common.Address{common.HexToAddress(grantee)}, adminNeeded)
}
