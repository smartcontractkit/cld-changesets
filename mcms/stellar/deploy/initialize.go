package stellardeploy

import (
	"context"
	"fmt"

	chainsel "github.com/smartcontractkit/chain-selectors"
	mcmsstellar "github.com/smartcontractkit/mcms/sdk/stellar"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stellar/go-stellar-sdk/network"

	"github.com/smartcontractkit/chainlink-stellar/bindings"
	mcmsbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/mcms"
	timelockbindings "github.com/smartcontractkit/chainlink-stellar/bindings/contracts/timelock"
)

// Contract initialization lives in the deploy changeset, not in the MCMS SDK:
// deployment and initialization are chain-family deployment concerns, while the
// SDK owns only protocol operations (set_config, set_root, execute, readers).
// Both helpers invoke the generated Soroban binding clients directly, mirroring
// how the EVM and Solana deploy packages use their chain-native bindings.

const maxInstanceLabelLength = 32

type initializeMCMSInput struct {
	ContractID    string
	Owner         string
	ChainID       string
	Config        *mcmstypes.Config
	InstanceLabel string
}

// initializeMCMS initializes a newly deployed Stellar MCMS contract.
func initializeMCMS(
	ctx context.Context,
	invoker bindings.Invoker,
	in initializeMCMSInput,
) error {
	if invoker == nil {
		return fmt.Errorf("stellar MCMS invoker is nil")
	}

	if in.ContractID == "" {
		return fmt.Errorf("stellar MCMS contract ID is empty")
	}

	if in.Owner == "" {
		return fmt.Errorf("stellar MCMS owner is empty")
	}

	if in.ChainID == "" {
		return fmt.Errorf("stellar MCMS chain ID is empty")
	}

	if in.Config == nil {
		return fmt.Errorf("stellar MCMS config is nil")
	}

	if in.InstanceLabel == "" {
		return fmt.Errorf("stellar MCMS instance label is empty")
	}
	if len(in.InstanceLabel) > maxInstanceLabelLength {
		return fmt.Errorf(
			"stellar MCMS instance label %q exceeds %d bytes",
			in.InstanceLabel,
			maxInstanceLabelLength,
		)
	}

	if err := in.Config.Validate(); err != nil {
		return fmt.Errorf("validate stellar MCMS config: %w", err)
	}

	networkPassphrase, err := chainsel.StellarPassphraseFromChainId(in.ChainID)
	if err != nil {
		return fmt.Errorf(
			"get stellar network passphrase from chain ID %q: %w",
			in.ChainID,
			err,
		)
	}

	networkID := network.ID(networkPassphrase)

	signerAddresses, signerGroups, groupQuorums, groupParents, err := mcmsstellar.ConfigToSetConfigInputs(in.Config)
	if err != nil {
		return fmt.Errorf("convert stellar MCMS config: %w", err)
	}

	client := mcmsbindings.NewMcmsClient(invoker, in.ContractID)

	if err := client.Initialize(
		ctx,
		in.Owner,
		networkID,
		signerAddresses,
		signerGroups,
		groupQuorums,
		groupParents,
		in.InstanceLabel,
	); err != nil {
		return fmt.Errorf("initialize stellar MCMS %s: %w", in.ContractID, err)
	}

	return nil
}

type initializeTimelockInput struct {
	ContractID string
	MinDelay   uint64
	Proposers  []string
	Cancellers []string
	Bypassers  []string
}

// initializeTimelock initializes a newly deployed Stellar timelock.
func initializeTimelock(
	ctx context.Context,
	invoker bindings.Invoker,
	in initializeTimelockInput,
) error {
	if invoker == nil {
		return fmt.Errorf("stellar timelock invoker is nil")
	}
	if in.ContractID == "" {
		return fmt.Errorf("stellar timelock contract ID is empty")
	}
	if err := validateTimelockRoleAddresses("proposers", in.Proposers); err != nil {
		return err
	}
	if err := validateTimelockRoleAddresses("cancellers", in.Cancellers); err != nil {
		return err
	}
	if err := validateTimelockRoleAddresses("bypassers", in.Bypassers); err != nil {
		return err
	}

	client := timelockbindings.NewTimelockClient(invoker, in.ContractID)

	if err := client.Initialize(
		ctx,
		in.MinDelay,
		in.Proposers,
		in.Cancellers,
		in.Bypassers,
	); err != nil {
		return fmt.Errorf(
			"initialize stellar timelock %s: %w",
			in.ContractID,
			err,
		)
	}

	return nil
}

func validateTimelockRoleAddresses(role string, addresses []string) error {
	if len(addresses) == 0 {
		return fmt.Errorf("initialize stellar timelock: %s are empty", role)
	}

	seen := make(map[string]struct{}, len(addresses))
	for i, address := range addresses {
		if address == "" {
			return fmt.Errorf(
				"initialize stellar timelock: %s[%d] is empty",
				role,
				i,
			)
		}
		if _, exists := seen[address]; exists {
			return fmt.Errorf(
				"initialize stellar timelock: %s contains duplicate address %q",
				role,
				address,
			)
		}
		seen[address] = struct{}{}
	}

	return nil
}
