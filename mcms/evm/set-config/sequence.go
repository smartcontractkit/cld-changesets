package evmsetconfig

import (
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
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
	var writes []opscontract.WriteOutput

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

		if useMCMS {
			writes = append(writes, opReport.Output)
			continue
		}

		if opReport.Output.Executed() {
			b.Logger.Infow("SetConfig tx confirmed",
				"chainSelector", chain.Selector,
				"address", target.Address.Hex(),
				"txHash", opReport.Output.ExecInfo.Hash,
			)
		}
	}

	if !useMCMS {
		return sequenceutils.OnChainOutput{}, nil
	}

	batch, err := opscontract.NewBatchOperationFromWrites(writes)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}
	if len(batch.Transactions) == 0 {
		return sequenceutils.OnChainOutput{}, nil
	}

	return sequenceutils.OnChainOutput{BatchOps: []mcmstypes.BatchOperation{batch}}, nil
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
