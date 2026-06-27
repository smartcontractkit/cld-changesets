package grantrole

import (
	"context"
	"fmt"
	"slices"

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
// normalize canonicalizes addresses for comparison; pass nil to compare raw strings.
func AddressesNeedingGrant(
	ctx context.Context,
	inspector mcmssdk.TimelockInspector,
	timelockAddress string,
	grant RoleGrant,
	normalize func(string) string,
) ([]string, error) {
	addressesWithRole, err := AddressesForRole(ctx, inspector, timelockAddress, grant.Role)
	if err != nil {
		return nil, err
	}
	if len(addressesWithRole) == 0 {
		return grant.Addresses, nil
	}

	if normalize == nil {
		normalize = func(address string) string { return address }
	}

	normalizedExisting := make([]string, len(addressesWithRole))
	for i, address := range addressesWithRole {
		normalizedExisting[i] = normalize(address)
	}

	out := make([]string, 0, len(grant.Addresses))
	for _, address := range grant.Addresses {
		if slices.Contains(normalizedExisting, normalize(address)) {
			continue
		}
		out = append(out, address)
	}

	return out, nil
}
