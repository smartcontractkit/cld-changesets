package grantrole

import (
	"context"
	"fmt"

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
