package soltransfertotimelock

import (
	"context"
	"fmt"

	solanago "github.com/gagliardetto/solana-go"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	accessControllerBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/access_controller"
	mcmBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/mcm"
	timelockBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/timelock"
)

func contractOwner(ctx context.Context, chain cldfsol.Chain, contract OwnableContract) (solanago.PublicKey, error) {
	switch contract.Type {
	case mcmscontracts.ProposerManyChainMultisig,
		mcmscontracts.CancellerManyChainMultisig,
		mcmscontracts.BypasserManyChainMultisig:
		var config mcmBindings.MultisigConfig
		if err := chain.GetAccountDataBorshInto(ctx, contract.OwnerPDA, &config); err != nil {
			return solanago.PublicKey{}, fmt.Errorf("read MCM config owner for %s: %w", contract.Type, err)
		}

		return config.Owner, nil
	case mcmscontracts.RBACTimelock:
		var config timelockBindings.Config
		if err := chain.GetAccountDataBorshInto(ctx, contract.OwnerPDA, &config); err != nil {
			return solanago.PublicKey{}, fmt.Errorf("read timelock config owner: %w", err)
		}

		return config.Owner, nil
	case mcmscontracts.ProposerAccessControllerAccount,
		mcmscontracts.ExecutorAccessControllerAccount,
		mcmscontracts.CancellerAccessControllerAccount,
		mcmscontracts.BypasserAccessControllerAccount:
		var config accessControllerBindings.AccessController
		if err := chain.GetAccountDataBorshInto(ctx, contract.OwnerPDA, &config); err != nil {
			return solanago.PublicKey{}, fmt.Errorf("read access controller owner for %s: %w", contract.Type, err)
		}

		return config.Owner, nil
	default:
		return solanago.PublicKey{}, fmt.Errorf("unsupported contract type %q for owner lookup", contract.Type)
	}
}

func validateContractOwner(
	contract OwnableContract,
	owner solanago.PublicKey,
	deployer solanago.PublicKey,
	timelock solanago.PublicKey,
	onlyAccept bool,
) error {
	if owner == timelock {
		return nil
	}

	if onlyAccept {
		if owner != deployer {
			return fmt.Errorf(
				"contract %s: only accept ownership requires current owner to be deployer or timelock, got %s",
				contract.Type,
				owner.String(),
			)
		}

		return nil
	}

	if owner != deployer {
		return fmt.Errorf("contract %s is not owned by the deployer key", contract.Type)
	}

	return nil
}
