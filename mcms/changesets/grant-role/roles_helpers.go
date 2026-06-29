package grantrole

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
)

// AddressesForRole returns accounts that currently hold role on the timelock.
// Admin returns nil without querying the chain.
func AddressesForRole(
	ctx context.Context,
	inspector mcmssdk.TimelockInspector,
	timelockAddress string,
	role mcmssdk.TimelockRole,
) ([]string, error) {
	switch role {
	case mcmssdk.TimelockRoleProposer:
		return inspector.GetProposers(ctx, timelockAddress)
	case mcmssdk.TimelockRoleCanceller:
		return inspector.GetCancellers(ctx, timelockAddress)
	case mcmssdk.TimelockRoleBypasser:
		return inspector.GetBypassers(ctx, timelockAddress)
	case mcmssdk.TimelockRoleExecutor:
		return inspector.GetExecutors(ctx, timelockAddress)
	case mcmssdk.TimelockRoleAdmin:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported timelock role %s", role.String())
	}
}

// AddressesNeedingGrant returns grant addresses that do not yet hold the role.
func AddressesNeedingGrant(
	ctx context.Context,
	inspector mcmssdk.TimelockInspector,
	timelockAddress string,
	grant RoleGrant,
) ([]string, error) {
	addressesWithRole, err := AddressesForRole(ctx, inspector, timelockAddress, grant.Role)
	if err != nil {
		return nil, err
	}
	needed, _ := lo.Difference(grant.Addresses, addressesWithRole)
	return needed, nil
}
