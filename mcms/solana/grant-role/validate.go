package solgrantrole

import (
	"errors"
	"fmt"

	solanago "github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
)

func validateMCMSIfPresent(e cldf.Environment, in grantrole.SeqInput) error {
	if in.MCMS == nil {
		return nil
	}

	input := *in.MCMS
	chainSelector := in.ChainSelector

	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilySolana)
	if !ok {
		return fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilySolana)
	}

	if _, err := reader.GetTimelockRef(e, chainSelector, input); err != nil {
		return fmt.Errorf("validate timelock ref for chain %d: %w", chainSelector, err)
	}
	if _, err := reader.GetMCMSRef(e, chainSelector, input); err != nil {
		return fmt.Errorf("validate mcms ref for chain %d: %w", chainSelector, err)
	}

	return nil
}

func validateRoles(in grantrole.SeqInput) error {
	for i, grant := range in.Grants {
		if !grant.Role.Valid() {
			return fmt.Errorf("grants[%d]: unsupported timelock role %s", i, grant.Role.String())
		}
		if grant.Role == mcmssdk.TimelockRoleAdmin {
			return fmt.Errorf("grants[%d]: admin role not supported on solana", i)
		}
	}

	return nil
}

func validateGrantAddresses(env cldf.Environment, in grantrole.SeqInput) error {
	for i, grant := range in.Grants {
		contractType, err := accessControllerContractType(grant.Role)
		if err != nil {
			return fmt.Errorf("grants[%d]: %w", i, err)
		}
		if _, err := accessControllerRef(env, in.ChainSelector, contractType); err != nil {
			return fmt.Errorf("grants[%d]: %w", i, err)
		}

		for j, addr := range grant.Addresses {
			if _, err := parseSolanaAddress(addr); err != nil {
				return fmt.Errorf("grants[%d].addresses[%d]: %w", i, j, err)
			}
		}
	}

	return nil
}

func parseSolanaAddress(raw string) (solanago.PublicKey, error) {
	pubkey, err := solanago.PublicKeyFromBase58(raw)
	if err != nil {
		return solanago.PublicKey{}, fmt.Errorf("address %q is not a valid solana address: %w", raw, err)
	}
	if pubkey == (solanago.PublicKey{}) {
		return solanago.PublicKey{}, errors.New("address must not be zero")
	}

	return pubkey, nil
}

func accessControllerContractType(role mcmssdk.TimelockRole) (cldf.ContractType, error) {
	switch role {
	case mcmssdk.TimelockRoleProposer:
		return mcmscontracts.ProposerAccessControllerAccount, nil
	case mcmssdk.TimelockRoleExecutor:
		return mcmscontracts.ExecutorAccessControllerAccount, nil
	case mcmssdk.TimelockRoleCanceller:
		return mcmscontracts.CancellerAccessControllerAccount, nil
	case mcmssdk.TimelockRoleBypasser:
		return mcmscontracts.BypasserAccessControllerAccount, nil
	case mcmssdk.TimelockRoleAdmin:
		return "", errors.New("admin role not supported on solana")
	default:
		return "", fmt.Errorf("unsupported timelock role %s", role.String())
	}
}

func accessControllerRef(
	env cldf.Environment,
	chainSelector uint64,
	contractType cldf.ContractType,
) (string, error) {
	if env.DataStore == nil {
		return "", fmt.Errorf("datastore not available for chain %d", chainSelector)
	}

	ref, err := datastore.FindUniqueRef(env.DataStore.Addresses(), datastore.AddressRef{
		ChainSelector: chainSelector,
		Type:          datastore.ContractType(contractType),
	})
	if err != nil {
		return "", fmt.Errorf("error fetching address in datastore for %s in chain %d: %w", contractType, chainSelector, err)
	}

	return ref.Address, nil
}
