package evmgrantrole

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/samber/lo"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
)

// AddressesNeedingGrant returns grant addresses that do not yet hold the role.
func AddressesNeedingGrant(
	ctx context.Context,
	inspector mcmssdk.TimelockInspector,
	timelock common.Address,
	grant grantrole.RoleGrant,
) ([]common.Address, error) {
	addressesWithRole, err := grantrole.AddressesForRole(ctx, inspector, timelock.Hex(), grant.Role)
	if err != nil {
		return nil, err
	}

	existing := lo.Map(addressesWithRole, func(address string, _ int) common.Address {
		return common.HexToAddress(address)
	})

	grantees := make([]common.Address, len(grant.Addresses))
	for i, address := range grant.Addresses {
		grantee, err := parseEVMAddress(address)
		if err != nil {
			return nil, fmt.Errorf("parse grantee address %q: %w", address, err)
		}
		grantees[i] = grantee
	}

	needed, _ := lo.Difference(grantees, existing)

	return needed, nil
}
