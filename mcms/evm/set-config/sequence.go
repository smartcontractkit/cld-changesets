package evmsetconfig

import (
	"fmt"
	"math/big"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmsTypes "github.com/smartcontractkit/mcms/types"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
)

// SeqEVMSetConfigInput is the input for the generic EVM set-config sequence.
type SeqEVMSetConfigInput struct {
	ChainSelector  uint64                            `json:"chainSelector"`
	NoSend         bool                              `json:"noSend"`
	GasBoostConfig *cldfproposalutils.GasBoostConfig `json:"gasBoostConfig,omitempty"`
	Targets        []MCMSetConfigTarget              `json:"targets"`
}

// SeqEVMSetConfig sets config on each provided MCM contract via OpEVMSetConfigMCM.
var SeqEVMSetConfig = operations.NewSequence(
	"seq-evm-mcm-set-config",
	semver.MustParse("1.0.0"),
	"Sets MCMS config on one or more MCM contracts",
	func(b operations.Bundle, deps cldf_evm.Chain, in SeqEVMSetConfigInput) (sequenceutils.OnChainOutput, error) {
		outs := make([]EVMCallOutput, 0, len(in.Targets))

		for _, target := range in.Targets {
			opReport, err := operations.ExecuteOperation(
				b,
				OpEVMSetConfigMCM,
				deps,
				OpEVMSetConfigInput{
					Target: target,
					NoSend: in.NoSend,
				},
				retrySetConfigWithGasBoost(in.GasBoostConfig),
			)
			if err != nil {
				return sequenceutils.OnChainOutput{}, err
			}

			out := opReport.Output
			out.ContractType = target.ContractType
			outs = append(outs, out)
		}

		out := sequenceutils.OnChainOutput{}
		if !in.NoSend {
			return out, nil
		}

		batch, err := evmCallOutputsToBatch(in.ChainSelector, outs)
		if err != nil {
			return sequenceutils.OnChainOutput{}, err
		}
		if len(batch.Transactions) > 0 {
			out.BatchOps = []mcmsTypes.BatchOperation{batch}
		}

		return out, nil
	},
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

	seqReport, err := operations.ExecuteSequence(
		b,
		SeqEVMSetConfig,
		chain,
		SeqEVMSetConfigInput{
			ChainSelector: in.ChainSelector,
			NoSend:        in.MCMS != nil,
			Targets:       targets,
		},
	)
	if err != nil {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("failed to execute EVM set config sequence: %w", err)
	}

	return seqReport.Output, nil
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

func evmCallOutputsToBatch(chainSelector uint64, outs []EVMCallOutput) (mcmsTypes.BatchOperation, error) {
	result := mcmsTypes.BatchOperation{
		ChainSelector: mcmsTypes.ChainSelector(chainSelector),
		Transactions:  []mcmsTypes.Transaction{},
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
			return mcmsTypes.BatchOperation{}, fmt.Errorf("failed to create batch operation for chain %d: %w", chainSelector, err)
		}
		result.Transactions = append(result.Transactions, batchOperation.Transactions...)
	}

	return result, nil
}
