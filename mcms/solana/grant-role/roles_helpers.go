package solgrantrole

import (
	"context"
	"fmt"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/samber/lo"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
)

// AddressesNeedingGrant returns grant addresses that do not yet hold the role.
func AddressesNeedingGrant(
	ctx context.Context,
	inspector mcmssdk.TimelockInspector,
	timelockAddress string,
	grant grantrole.RoleGrant,
) ([]solanago.PublicKey, error) {
	addressesWithRole, err := grantrole.AddressesForRole(ctx, inspector, timelockAddress, grant.Role)
	if err != nil {
		return nil, err
	}

	existing := make([]solanago.PublicKey, len(addressesWithRole))
	for i, address := range addressesWithRole {
		pubkey, err := parseSolanaAddress(address)
		if err != nil {
			return nil, fmt.Errorf("parse role holder address %q: %w", address, err)
		}
		existing[i] = pubkey
	}

	grantees := make([]solanago.PublicKey, len(grant.Addresses))
	for i, address := range grant.Addresses {
		grantee, err := parseSolanaAddress(address)
		if err != nil {
			return nil, fmt.Errorf("parse grantee address %q: %w", address, err)
		}
		grantees[i] = grantee
	}

	needed, _ := lo.Difference(grantees, existing)

	return needed, nil
}
