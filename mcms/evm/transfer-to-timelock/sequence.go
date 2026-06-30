package evmtransfertotimelock

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	gobindings "github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/burn_mint_erc677"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
	ownableops "github.com/smartcontractkit/cld-changesets/mcms/evm/transfer-to-timelock/v1_0_0/operations/burn_mint_erc677"
)

var seqTransferToTimelock = operations.NewSequence(
	"seq-evm-transfer-to-timelock",
	semver.MustParse("1.0.0"),
	"Transfers ownable contract ownership to the MCMS timelock",
	runEVMTransferToTimelock,
)

func runEVMTransferToTimelock(
	b operations.Bundle,
	deps transfertotimelock.Deps,
	in transfertotimelock.ChainInput,
) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.EVMChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("EVM chain %d not found in environment", in.ChainSelector)
	}

	env := transfertotimelock.EnvFromDeps(deps)
	if in.MCMS == nil {
		return sequenceutils.OnChainOutput{}, errors.New("MCMS timelock proposal input is required")
	}

	timelock, err := timelockAddress(env, in)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}

	var transactions []mcmstypes.Transaction
	for i, ref := range in.Contracts {
		contract, err := resolveEVMAddress(env, in.ChainSelector, ref)
		if err != nil {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("contracts[%d]: %w", i, err)
		}

		txs, err := transferContractToTimelock(b, chain, timelock, contract, in)
		if err != nil {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("contract %s: %w", contract.Hex(), err)
		}
		transactions = append(transactions, txs...)
	}

	if len(transactions) == 0 {
		return sequenceutils.OnChainOutput{}, nil
	}

	return sequenceutils.OnChainOutput{
		BatchOps: []mcmstypes.BatchOperation{{
			ChainSelector: mcmstypes.ChainSelector(in.ChainSelector),
			Transactions:  transactions,
		}},
	}, nil
}

func transferContractToTimelock(
	b operations.Bundle,
	chain cldf_evm.Chain,
	timelock common.Address,
	contract common.Address,
	in transfertotimelock.ChainInput,
) ([]mcmstypes.Transaction, error) {
	binding, err := bindOwnableContract(contract, chain.Client)
	if err != nil {
		return nil, err
	}

	owner, err := contractOwner(binding)
	if err != nil {
		return nil, err
	}

	if owner == timelock {
		b.Logger.Infof("contract %s already owned by timelock", contract.Hex())
		return nil, nil
	}

	if !in.OnlyAcceptOwnership {
		_, err = operations.ExecuteOperation(
			b,
			ownableops.NewWriteTransferOwnership(binding),
			chain,
			opscontract.FunctionInput[common.Address]{Args: timelock},
			contractIdempotencyKey[opscontract.FunctionInput[common.Address]](chain, contract),
		)
		if err != nil {
			return nil, fmt.Errorf("transfer ownership: %w", err)
		}

		owner, err = contractOwner(binding)
		if err != nil {
			return nil, err
		}
		if owner == timelock {
			b.Logger.Infof("contract %s already owned by timelock after transfer", contract.Hex())
			return nil, nil
		}
	}

	acceptReport, err := operations.ExecuteOperation(
		b,
		ownableops.NewWriteAcceptOwnership(binding),
		chain,
		opscontract.FunctionInput[struct{}]{},
		contractIdempotencyKey[opscontract.FunctionInput[struct{}]](chain, contract),
	)
	if err != nil {
		return nil, fmt.Errorf("accept ownership: %w", err)
	}

	return []mcmstypes.Transaction{acceptReport.Output.Tx}, nil
}

func bindOwnableContract(addr common.Address, client bind.ContractBackend) (gobindings.BurnMintERC677Interface, error) {
	// BurnMintERC677 bindings provide a generic two-step ownable ABI surface
	// (owner, transferOwnership, acceptOwnership). Validation only calls Owner().
	c, err := gobindings.NewBurnMintERC677(addr, client)
	if err != nil {
		return nil, fmt.Errorf("create ownable contract binding: %w", err)
	}

	return c, nil
}

func contractOwner(c gobindings.BurnMintERC677Interface) (common.Address, error) {
	owner, err := c.Owner(nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("get owner of contract %s: %w", c.Address().Hex(), err)
	}

	return owner, nil
}

func contractIdempotencyKey[IN any](chain cldf_evm.Chain, contract common.Address) operations.ExecuteOption[IN, cldf_evm.Chain] {
	return operations.WithIdempotencyKey[IN, cldf_evm.Chain](strconv.FormatUint(chain.Selector, 10) + ":" + contract.Hex())
}
