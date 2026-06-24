package evmtransfertotimelock

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
)

func validateMCMS(env cldf.Environment, in transfertotimelock.ChainInput) error {
	if in.MCMS == nil {
		return errors.New("MCMS timelock proposal input is required")
	}

	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyEVM)
	if !ok {
		return fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilyEVM)
	}
	timelockRef, err := reader.GetTimelockRef(env, in.ChainSelector, *in.MCMS)
	if err != nil {
		return fmt.Errorf("timelock not present on chain %d: %w", in.ChainSelector, err)
	}
	if _, err = parseEVMAddress(timelockRef.Address, "timelock"); err != nil {
		return fmt.Errorf("invalid timelock ref on chain %d: %w", in.ChainSelector, err)
	}

	mcmsRef, err := reader.GetMCMSRef(env, in.ChainSelector, *in.MCMS)
	if err != nil {
		return fmt.Errorf("mcms not present on chain %d: %w", in.ChainSelector, err)
	}
	if _, err = parseEVMAddress(mcmsRef.Address, "mcms"); err != nil {
		return fmt.Errorf("invalid mcms ref on chain %d: %w", in.ChainSelector, err)
	}

	return nil
}

func validateContracts(env cldf.Environment, in transfertotimelock.ChainInput) error {
	chain, ok := env.BlockChains.EVMChains()[in.ChainSelector]
	if !ok {
		return fmt.Errorf("EVM chain %d not found in environment", in.ChainSelector)
	}
	if chain.DeployerKey == nil {
		return fmt.Errorf("missing deployer key for chain %d", in.ChainSelector)
	}

	seen := make(map[common.Address]struct{}, len(in.Contracts))
	for _, contract := range in.Contracts {
		if (contract == common.Address{}) {
			return errors.New("contract address must not be zero")
		}
		if _, dup := seen[contract]; dup {
			return fmt.Errorf("duplicate contract address %s", contract.Hex())
		}
		seen[contract] = struct{}{}
	}

	timelock, err := timelockAddress(env, in)
	if err != nil {
		return err
	}

	for _, contract := range in.Contracts {
		if err := contractInDatastore(env, in.ChainSelector, contract); err != nil {
			return fmt.Errorf("contract %s: %w", contract.Hex(), err)
		}

		binding, err := bindOwnableContract(contract, chain.Client)
		if err != nil {
			return fmt.Errorf("contract %s: %w", contract.Hex(), err)
		}

		owner, err := contractOwner(binding)
		if err != nil {
			return fmt.Errorf("contract %s: %w", contract.Hex(), err)
		}
		if err := validateContractOwner(contract, owner, chain.DeployerKey.From, timelock, in.OnlyAcceptOwnership); err != nil {
			return err
		}
	}

	return nil
}

func timelockAddress(env cldf.Environment, in transfertotimelock.ChainInput) (common.Address, error) {
	if in.MCMS == nil {
		return common.Address{}, errors.New("MCMS timelock proposal input is required")
	}

	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyEVM)
	if !ok {
		return common.Address{}, fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilyEVM)
	}
	timelockRef, err := reader.GetTimelockRef(env, in.ChainSelector, *in.MCMS)
	if err != nil {
		return common.Address{}, fmt.Errorf("resolve timelock for chain %d: %w", in.ChainSelector, err)
	}

	return parseEVMAddress(timelockRef.Address, "timelock")
}

func contractInDatastore(env cldf.Environment, chainSelector uint64, contract common.Address) error {
	if env.DataStore == nil {
		return errors.New("datastore is required")
	}

	refs := env.DataStore.Addresses().Filter(datastore.AddressRefByChainSelector(chainSelector))
	for _, ref := range refs {
		if common.HexToAddress(ref.Address) == contract {
			return nil
		}
	}

	return errors.New("not found in datastore")
}

// validateContractOwner enforces ownership preconditions for transfer-to-timelock.
// When onlyAccept is true, the timelock may already own the contract or the
// deployer may still be owner after an on-chain transferOwnership (pending accept).
func validateContractOwner(
	contract common.Address,
	owner common.Address,
	deployer common.Address,
	timelock common.Address,
	onlyAccept bool,
) error {
	if owner == timelock {
		return nil
	}

	if onlyAccept {
		if owner != deployer {
			return fmt.Errorf(
				"contract %s: only accept ownership requires current owner to be deployer or timelock, got %s",
				contract.Hex(),
				owner.Hex(),
			)
		}

		return nil
	}

	if owner != deployer {
		return fmt.Errorf("contract %s is not owned by the deployer key", contract.Hex())
	}

	return nil
}
