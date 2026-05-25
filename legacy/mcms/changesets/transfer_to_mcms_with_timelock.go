package changesets

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	owner_helpers "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	mcmslib "github.com/smartcontractkit/mcms"
	"github.com/smartcontractkit/mcms/sdk"
	"github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
	seqs "github.com/smartcontractkit/cld-changesets/pkg/family/evm/sequences"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/burn_mint_erc677"

	"github.com/smartcontractkit/cld-changesets/legacy/mcms/proposeutils"
)

type TransferToMCMSWithTimelockConfig struct {
	ContractsByChain map[uint64][]common.Address
	// MCMSConfig is for the accept ownership proposal
	MCMSConfig cldfproposalutils.TimelockConfig

	// Optional flag: Only runs accept ownership (not transfer ownership) if enabled
	OnlyAcceptOwnership bool
}

func LoadOwnableContract(addr common.Address, client bind.ContractBackend) (common.Address, evmstate.Ownable, error) {
	// Just using the ownership interface from here.
	c, err := burn_mint_erc677.NewBurnMintERC677(addr, client)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to create contract: %w", err)
	}
	owner, err := c.Owner(nil)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to get owner of contract %s: %w", c.Address(), err)
	}

	return owner, c, nil
}

// searchContractInBothSources searches for a contract type in both AddressBook and DataStore
// Returns the address if found in either source (similar to cldf.SearchAddressBook)
func searchContractInBothSources(e cldf.Environment, chainSelector uint64, contractType cldf.ContractType, qualifier string) (string, error) {
	// Use the merged address loading from the EVM state function
	addressesChain, err := evmstate.AddressesForChain(e, chainSelector, qualifier)
	if err != nil {
		return "", fmt.Errorf("failed to load addresses: %w", err)
	}

	// Search through merged addresses for the contract type
	for addr, tv := range addressesChain {
		if tv.Type == contractType {
			return addr, nil
		}
	}

	return "", fmt.Errorf("%s not found", contractType)
}

func (t TransferToMCMSWithTimelockConfig) Validate(e cldf.Environment) error {
	evmChains := e.BlockChains.EVMChains()
	for chainSelector, contracts := range t.ContractsByChain {
		for _, contract := range contracts {
			// Cannot transfer an unknown address.
			// Note this also assures non-zero addresses.
			if exists, err := searchAddress(e, chainSelector, contract.String()); err != nil || !exists {
				if err != nil {
					return fmt.Errorf("failed to search address: %w", err)
				}

				return fmt.Errorf("contract %s not found in address book or datastore", contract)
			}

			owner, _, err := LoadOwnableContract(contract, evmChains[chainSelector].Client)
			if err != nil {
				return fmt.Errorf("failed to load ownable: %w", err)
			}
			if !t.OnlyAcceptOwnership && owner != evmChains[chainSelector].DeployerKey.From {
				return fmt.Errorf("contract %s is not owned by the deployer key", contract)
			}
		}
		// If there is no timelock and mcms proposer on the chain, the transfer will fail.
		qualifier := ""
		if t.MCMSConfig.TimelockQualifierPerChain != nil {
			qualifier = t.MCMSConfig.TimelockQualifierPerChain[chainSelector]
		}
		if _, err := searchContractInBothSources(e, chainSelector, mcmscontracts.RBACTimelock, qualifier); err != nil {
			return fmt.Errorf("timelock not present on the chain %w", err)
		}
		if _, err := searchContractInBothSources(e, chainSelector, mcmscontracts.ProposerManyChainMultisig, qualifier); err != nil {
			return fmt.Errorf("mcms proposer not present on the chain %w", err)
		}
	}

	return nil
}

var _ cldf.ChangeSet[TransferToMCMSWithTimelockConfig] = TransferToMCMSWithTimelockV2

// TransferToMCMSWithTimelockV2 is a reimplementation of TransferToMCMSWithTimelock which uses the new MCMS library.
func TransferToMCMSWithTimelockV2(
	e cldf.Environment,
	cfg TransferToMCMSWithTimelockConfig,
) (cldf.ChangesetOutput, error) {
	if err := cfg.Validate(e); err != nil {
		return cldf.ChangesetOutput{}, err
	}
	batches := []mcmstypes.BatchOperation{}
	timelockAddressByChain := make(map[uint64]string)
	inspectorPerChain := map[uint64]sdk.Inspector{}
	proposerAddressByChain := make(map[uint64]string)
	evmChains := e.BlockChains.EVMChains()
	execReports := make([]operations.Report[any, any], 0)
	for chainSelector, contracts := range cfg.ContractsByChain {
		qualifier := ""
		if cfg.MCMSConfig.TimelockQualifierPerChain != nil {
			qualifier = cfg.MCMSConfig.TimelockQualifierPerChain[chainSelector]
		}
		// Already validated that the timelock/proposer exists.
		timelockAddr, _ := searchContractInBothSources(e, chainSelector, mcmscontracts.RBACTimelock, qualifier)
		proposerAddr, _ := searchContractInBothSources(e, chainSelector, mcmscontracts.ProposerManyChainMultisig, qualifier)
		timelockAddressByChain[chainSelector] = timelockAddr
		proposerAddressByChain[chainSelector] = proposerAddr
		e.Logger.Infof("timelock on chain %d is %s, proposer is %s", chainSelector, timelockAddr, proposerAddr)
		inspectorPerChain[chainSelector] = evm.NewInspector(evmChains[chainSelector].Client)

		seqReport, err := operations.ExecuteSequence(e.OperationsBundle, seqs.SeqTransferToMCMSWithTimelockV2,
			seqs.SeqTransferToMCMSWithTimelockV2Deps{
				Chain: evmChains[chainSelector],
			},
			seqs.SeqTransferToMCMSWithTimelockV2Input{
				ChainSelector:       chainSelector,
				Timelock:            common.HexToAddress(timelockAddr),
				Contracts:           contracts,
				OnlyAcceptOwnership: cfg.OnlyAcceptOwnership,
			},
		)
		execReports = append(execReports, seqReport.ExecutionReports...)

		if err != nil {
			return cldf.ChangesetOutput{
				Reports: execReports,
			}, fmt.Errorf("failed to execute sequence: %w", err)
		}

		batches = append(batches, mcmstypes.BatchOperation{
			ChainSelector: mcmstypes.ChainSelector(chainSelector),
			Transactions:  seqReport.Output.OpsMcms,
		})
	}
	proposal, err := proposeutils.BuildProposalFromBatchesV2(
		e,
		timelockAddressByChain, proposerAddressByChain, inspectorPerChain,
		batches, "Transfer ownership to timelock", cfg.MCMSConfig)
	if err != nil {
		return cldf.ChangesetOutput{Reports: execReports}, fmt.Errorf("failed to build proposal from batch: %w, batches: %+v", err, batches)
	}
	e.Logger.Infof("created proposal %s with timelocks %v", proposal.Description, proposal.TimelockAddresses)

	return cldf.ChangesetOutput{Reports: execReports, MCMSTimelockProposals: []mcmslib.TimelockProposal{*proposal}}, nil
}

var _ cldf.ChangeSet[TransferToDeployerConfig] = TransferToDeployer

type TransferToDeployerConfig struct {
	ContractAddress common.Address
	ChainSel        uint64
}

// TransferToDeployer relies on the deployer key
// still being a timelock admin and transfers the ownership of a contract
// back to the deployer key. It's effectively the rollback function of transferring
// to the timelock.
func TransferToDeployer(e cldf.Environment, cfg TransferToDeployerConfig) (cldf.ChangesetOutput, error) {
	evmChains := e.BlockChains.EVMChains()
	owner, ownable, err := LoadOwnableContract(cfg.ContractAddress, evmChains[cfg.ChainSel].Client)
	if err != nil {
		return cldf.ChangesetOutput{}, err
	}
	if owner == evmChains[cfg.ChainSel].DeployerKey.From {
		e.Logger.Infof("Contract %s already owned by deployer", cfg.ContractAddress)
		return cldf.ChangesetOutput{}, nil
	}
	tx, err := ownable.TransferOwnership(cldf.SimTransactOpts(), evmChains[cfg.ChainSel].DeployerKey.From)
	if err != nil {
		return cldf.ChangesetOutput{}, err
	}
	addrs, err := e.ExistingAddresses.AddressesForChain(cfg.ChainSel) //nolint already flagged as legacy in pkg
	if err != nil {
		return cldf.ChangesetOutput{}, err
	}
	tls, err := evmstate.MaybeLoadMCMSWithTimelockChainState(evmChains[cfg.ChainSel], addrs)
	if err != nil {
		return cldf.ChangesetOutput{}, err
	}
	calls := []owner_helpers.RBACTimelockCall{
		{
			Target: ownable.Address(),
			Data:   tx.Data(),
			Value:  big.NewInt(0),
		},
	}
	var salt [32]byte
	binary.BigEndian.PutUint32(salt[:], uint32(time.Now().Unix())) //nolint:gosec // this is a salt, so any value is fine
	tx, err = tls.Timelock.ScheduleBatch(evmChains[cfg.ChainSel].DeployerKey, calls, [32]byte{}, salt, big.NewInt(0))
	if _, err = cldf.ConfirmIfNoErrorWithABI(evmChains[cfg.ChainSel], tx, owner_helpers.RBACTimelockABI, err); err != nil {
		return cldf.ChangesetOutput{}, err
	}
	e.Logger.Infof("scheduled transfer ownership batch with tx %s", tx.Hash().Hex())
	timelockExecutorProxy, err := owner_helpers.NewRBACTimelock(tls.CallProxy.Address(), evmChains[cfg.ChainSel].Client)
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("error creating timelock executor proxy: %w", err)
	}

	tx, err = timelockExecutorProxy.ExecuteBatch(
		evmChains[cfg.ChainSel].DeployerKey, calls, [32]byte{}, salt)
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("error executing batch: %w", err)
	}
	if _, err = cldf.ConfirmIfNoErrorWithABI(evmChains[cfg.ChainSel], tx, owner_helpers.RBACTimelockABI, err); err != nil {
		return cldf.ChangesetOutput{}, err
	}

	e.Logger.Infof("executed transfer ownership to deployer key with tx %s", tx.Hash().Hex())

	tx, err = ownable.AcceptOwnership(evmChains[cfg.ChainSel].DeployerKey)
	if _, err = cldf.ConfirmIfNoError(evmChains[cfg.ChainSel], tx, err); err != nil {
		return cldf.ChangesetOutput{}, err
	}
	e.Logger.Infof("deployer key accepted ownership tx %s", tx.Hash().Hex())

	return cldf.ChangesetOutput{}, nil
}

var _ cldf.ChangeSet[RenounceTimelockDeployerConfig] = RenounceTimelockDeployer

type RenounceTimelockDeployerConfig struct {
	ChainSel  uint64
	Qualifier string
}

func (cfg RenounceTimelockDeployerConfig) Validate(e cldf.Environment) error {
	if err := cldf.IsValidChainSelector(cfg.ChainSel); err != nil {
		return fmt.Errorf("invalid chain selector: %w", err)
	}

	_, ok := e.BlockChains.EVMChains()[cfg.ChainSel]
	if !ok {
		return fmt.Errorf("chain selector: %d not found in environment", cfg.ChainSel)
	}

	// MCMS should already exists
	contracts, err := evmstate.MaybeLoadMCMSWithTimelockStateWithQualifier(e, []uint64{cfg.ChainSel}, cfg.Qualifier)
	if err != nil {
		return err
	}

	contract, ok := contracts[cfg.ChainSel]
	if !ok {
		return fmt.Errorf("mcms contracts not found on chain %d", cfg.ChainSel)
	}
	if contract.Timelock == nil {
		return fmt.Errorf("timelock not found on chain %d", cfg.ChainSel)
	}

	return nil
}

// RenounceTimelockDeployer revokes the deployer key from administering the contract.
func RenounceTimelockDeployer(e cldf.Environment, cfg RenounceTimelockDeployerConfig) (cldf.ChangesetOutput, error) {
	if err := cfg.Validate(e); err != nil {
		return cldf.ChangesetOutput{}, err
	}

	contracts, err := evmstate.MaybeLoadMCMSWithTimelockStateWithQualifier(e, []uint64{cfg.ChainSel}, cfg.Qualifier)
	if err != nil {
		return cldf.ChangesetOutput{}, err
	}
	tl := contracts[cfg.ChainSel].Timelock
	admin, err := tl.ADMINROLE(&bind.CallOpts{Context: e.GetContext()})
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("failed to get admin role: %w", err)
	}

	chain := e.BlockChains.EVMChains()[cfg.ChainSel]
	tx, errRen := tl.RenounceRole(chain.DeployerKey, admin, chain.DeployerKey.From)
	if errRen != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("failed to revoke deployer key: %w", errRen)
	}
	if _, errConfirm := cldf.ConfirmIfNoErrorWithABI(chain, tx, owner_helpers.RBACTimelockABI, err); errConfirm != nil {
		return cldf.ChangesetOutput{}, errConfirm
	}
	e.Logger.Infof("revoked deployer key from owning contract %s", tl.Address().Hex())

	return cldf.ChangesetOutput{}, nil
}

// searchAddress searches for a contract address in both AddressBook and DataStore.
//
// Returns true if the address is found in either source.
func searchAddress(e cldf.Environment, chainSelector uint64, address string) (bool, error) {
	// Use the merged address loading from the EVM state function
	addressesChain, err := evmstate.AddressesForChain(e, chainSelector, "")
	if err != nil {
		return false, fmt.Errorf("failed to load addresses: %w", err)
	}

	// Search through merged addresses for the contract type
	for addr := range addressesChain {
		if addr == address {
			return true, nil
		}
	}

	return false, nil
}
