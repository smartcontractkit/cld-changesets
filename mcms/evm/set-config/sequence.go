package evmsetconfig

import (
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"

	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/mcms/sdk"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	evmdeploy "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy"
	mcmops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/many_chain_multi_sig"
)

type mcmSetConfigTarget struct {
	Address      common.Address    `json:"address"`
	Config       mcmstypes.Config  `json:"config"`
	ContractType cldf.ContractType `json:"contractType"`
}

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

	writeOuts := make([]opscontract.WriteOutput, 0, len(targets))
	for _, target := range targets {
		writeOut, execErr := executeSetConfig(b, chain, target, nil)
		if execErr != nil {
			return sequenceutils.OnChainOutput{}, execErr
		}
		writeOuts = append(writeOuts, writeOut)
	}

	batch, err := opscontract.NewBatchOperationFromWrites(writeOuts)
	if err != nil {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("build set-config batch: %w", err)
	}

	out := sequenceutils.OnChainOutput{}
	if len(batch.Transactions) > 0 {
		out.BatchOps = []mcmstypes.BatchOperation{batch}
	}

	return out, nil
}

func executeSetConfig(
	b operations.Bundle,
	chain cldf_evm.Chain,
	target mcmSetConfigTarget,
	gasBoost *cldfproposalutils.GasBoostConfig,
) (opscontract.WriteOutput, error) {
	groupQuorums, groupParents, signerAddresses, signerGroups, err := sdk.ExtractSetConfigInputs(&target.Config)
	if err != nil {
		return opscontract.WriteOutput{}, err
	}

	mcm, err := bindings.NewManyChainMultiSig(target.Address, chain.Client)
	if err != nil {
		return opscontract.WriteOutput{}, fmt.Errorf("bind MCM at %s: %w", target.Address, err)
	}

	report, err := operations.ExecuteOperation(
		b,
		mcmops.NewWriteSetConfig(mcm),
		chain,
		opscontract.FunctionInput[mcmops.SetConfigArgs]{
			Args: mcmops.SetConfigArgs{
				SignerAddresses: signerAddresses,
				SignerGroups:    signerGroups,
				GroupQuorums:    groupQuorums,
				GroupParents:    groupParents,
				ClearRoot:       false,
			},
		},
		evmdeploy.RetryWriteWithGasBoost[mcmops.SetConfigArgs](gasBoost),
		operations.WithIdempotencyKey[opscontract.FunctionInput[mcmops.SetConfigArgs], cldf_evm.Chain](strconv.FormatUint(chain.Selector, 10)+":"+target.Address.Hex()),
	)
	if err != nil {
		return opscontract.WriteOutput{}, fmt.Errorf("set config on %s: %w", target.Address, err)
	}

	out := report.Output
	out.Tx.ContractType = string(target.ContractType)

	return out, nil
}

func setConfigTargets(e cldf.Environment, configs []setconfig.ContractSetConfig) ([]mcmSetConfigTarget, error) {
	targets := make([]mcmSetConfigTarget, 0, len(configs))

	for i, cfg := range configs {
		ref, err := cfg.Ref.Resolve(e)
		if err != nil {
			return nil, fmt.Errorf("targets[%d]: %w", i, err)
		}
		if !common.IsHexAddress(ref.Address) {
			return nil, fmt.Errorf("targets[%d]: invalid EVM address %q", i, ref.Address)
		}

		targets = append(targets, mcmSetConfigTarget{
			Address:      common.HexToAddress(ref.Address),
			Config:       cfg.Config,
			ContractType: cldf.ContractType(ref.Type),
		})
	}

	return targets, nil
}
