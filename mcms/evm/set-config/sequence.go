package evmsetconfig

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
)

var seqSetConfig = operations.NewSequence(
	"seq-evm-set-config-mcms",
	semver.MustParse("1.0.0"),
	"Sets MCMS config on EVM chains from datastore refs",
	runEVMSetConfig,
)

func runEVMSetConfig(
	b operations.Bundle,
	deps setconfig.Deps,
	in setconfig.ChainInput,
) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.EVMChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("EVM chain %d not found in environment", in.ChainSelector)
	}

	targets, err := setConfigTargets(setconfig.EnvFromDeps(deps), in.Targets)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}

	useMCMS := in.MCMS != nil

	var outs []EVMCallOutput
	if useMCMS {
		outs = make([]EVMCallOutput, 0, len(targets))
	}

	for _, target := range targets {
		opReport, execErr := operations.ExecuteOperation(
			b,
			OpEVMSetConfigMCM,
			chain,
			OpEVMSetConfigInput{
				Target: target,
				NoSend: useMCMS,
			},
			retrySetConfigWithGasBoost(nil),
			operations.WithIdempotencyKey[OpEVMSetConfigInput, cldf_evm.Chain](strconv.FormatUint(chain.Selector, 10)+":"+target.Address.Hex()),
		)
		if execErr != nil {
			return sequenceutils.OnChainOutput{}, execErr
		}

		if !useMCMS {
			continue
		}

		out := opReport.Output
		out.ContractType = target.ContractType
		outs = append(outs, out)
	}

	out := sequenceutils.OnChainOutput{}
	if !useMCMS {
		return out, nil
	}

	batch, err := evmCallOutputsToBatch(in.ChainSelector, outs)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}
	if len(batch.Transactions) > 0 {
		out.BatchOps = []mcmstypes.BatchOperation{batch}
	}

	return out, nil
}

func setConfigTargets(e cldf.Environment, configs []setconfig.ContractSetConfig) ([]MCMSetConfigTarget, error) {
	targets := make([]MCMSetConfigTarget, 0, len(configs))

	for i, cfg := range configs {
		ref, err := cfg.Ref.Resolve(e)
		if err != nil {
			return nil, fmt.Errorf("targets[%d]: %w", i, err)
		}
		if !common.IsHexAddress(ref.Address) {
			return nil, fmt.Errorf("targets[%d]: invalid EVM address %q", i, ref.Address)
		}

		targets = append(targets, MCMSetConfigTarget{
			Address:      common.HexToAddress(ref.Address),
			Config:       cfg.Config,
			ContractType: cldf.ContractType(ref.Type),
		})
	}

	return targets, nil
}

func evmCallOutputsToBatch(chainSelector uint64, outs []EVMCallOutput) (mcmstypes.BatchOperation, error) {
	result := mcmstypes.BatchOperation{
		ChainSelector: mcmstypes.ChainSelector(chainSelector),
		Transactions:  []mcmstypes.Transaction{},
	}

	for _, out := range outs {
		if out.Confirmed {
			continue
		}

		batchOperation, err := cldfproposalutils.BatchOperationForChain(
			chainSelector,
			out.To.Hex(),
			out.Data,
			big.NewInt(0),
			string(out.ContractType),
			[]string{},
		)
		if err != nil {
			return mcmstypes.BatchOperation{}, fmt.Errorf("failed to create batch operation for chain %d: %w", chainSelector, err)
		}
		result.Transactions = append(result.Transactions, batchOperation.Transactions...)
	}

	return result, nil
}
