package stellarsetconfig

import (
	"fmt"
	"strconv"

	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmsstellar "github.com/smartcontractkit/mcms/sdk/stellar"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
)

var seqSetConfig = operations.NewSequence(
	"seq-stellar-set-config-mcms",
	&semvers.V1_0_0,
	"Sets MCMS config on Stellar chains from datastore refs",
	runStellarSetConfig,
)

func runStellarSetConfig(b operations.Bundle, deps setconfig.Deps, in setconfig.ChainInput) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.StellarChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("stellar chain %d not found in environment", in.ChainSelector)
	}

	targets, err := setConfigTargets(
		setconfig.EnvFromDeps(deps),
		in.Targets,
	)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}

	useMCMS := in.MCMS != nil

	var batchOps []mcmstypes.BatchOperation
	if useMCMS {
		batchOps = make([]mcmstypes.BatchOperation, 0, len(targets))
	}

	for _, target := range targets {
		opReport, execErr := operations.ExecuteOperation(
			b,
			OpStellarSetConfigMCM,
			chain,
			OpStellarSetConfigInput{
				Target: target,
				NoSend: useMCMS,
			},
			operations.WithIdempotencyKey[OpStellarSetConfigInput, cldfstellar.Chain](
				strconv.FormatUint(chain.Selector, 10)+":"+target.Address),
		)
		if execErr != nil {
			return sequenceutils.OnChainOutput{}, execErr
		}

		if useMCMS && opReport.Output.BatchOperation != nil {
			batchOps = append(
				batchOps,
				*opReport.Output.BatchOperation,
			)
		}
	}

	if useMCMS && len(batchOps) == 0 {
		return sequenceutils.OnChainOutput{}, nil
	}

	return sequenceutils.OnChainOutput{BatchOps: batchOps}, nil
}

func setConfigTargets(e cldf.Environment, configs []setconfig.ContractSetConfig) ([]MCMSetConfigTarget, error) {
	targets := make([]MCMSetConfigTarget, 0, len(configs))

	for i, cfg := range configs {
		ref, err := cfg.Ref.Resolve(e)
		if err != nil {
			return nil, fmt.Errorf("targets[%d]: %w", i, err)
		}

		if !mcmsstellar.IsAddress(ref.Address) {
			return nil, fmt.Errorf("targets[%d]: invalid Stellar address %q", i, ref.Address)
		}

		targets = append(
			targets,
			MCMSetConfigTarget{
				Address:      ref.Address,
				Config:       cfg.Config,
				ContractType: string(ref.Type),
			},
		)
	}

	return targets, nil
}
