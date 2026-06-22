package solsetconfig

import (
	"fmt"
	"strconv"

	solanago "github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

var seqSetConfig = operations.NewSequence(
	"seq-solana-set-config-mcms",
	&semvers.V1_0_0,
	"Sets MCMS config on Solana chains from datastore refs",
	runSolanaSetConfig,
)

func runSolanaSetConfig(
	b operations.Bundle,
	deps setconfig.Deps,
	in setconfig.ChainInput,
) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.SolanaChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("solana chain %d not found in environment", in.ChainSelector)
	}

	env := setconfig.EnvFromDeps(deps)
	useMCMS := in.MCMS != nil
	authorityAccount := solanago.PublicKey{}
	if useMCMS {
		reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilySolana)
		if !ok {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilySolana)
		}

		timelockRef, err := reader.GetTimelockRef(env, in.ChainSelector, *in.MCMS)
		if err != nil {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("resolve timelock ref for chain %d: %w", in.ChainSelector, err)
		}

		timelockProgram, timelockSeed, err := mcmssolanasdk.ParseContractAddress(timelockRef.Address)
		if err != nil {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("parse timelock ref address for chain %d: %w", in.ChainSelector, err)
		}

		var seed legacysolana.PDASeed
		copy(seed[:], timelockSeed[:])
		authorityAccount = familysolana.GetTimelockSignerPDA(timelockProgram, seed)
	}

	targets, err := setConfigTargets(env, in.Targets)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}

	var batchOps []mcmstypes.BatchOperation
	if useMCMS {
		batchOps = make([]mcmstypes.BatchOperation, 0, len(targets))
	}

	for _, target := range targets {
		opReport, execErr := operations.ExecuteOperation(
			b,
			OpSolanaSetConfigMCM,
			chain,
			OpSolanaSetConfigInput{
				Target:           target,
				NoSend:           useMCMS,
				AuthorityAccount: authorityAccount,
			},
			operations.WithIdempotencyKey[OpSolanaSetConfigInput, cldfsol.Chain](strconv.FormatUint(chain.Selector, 10)+":"+target.Address),
		)
		if execErr != nil {
			return sequenceutils.OnChainOutput{}, execErr
		}

		if useMCMS {
			batchOps = append(batchOps, opReport.Output.BatchOperation)
		}
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

		targets = append(targets, MCMSetConfigTarget{
			Address:      ref.Address,
			Config:       cfg.Config,
			ContractType: string(ref.Type),
		})
	}

	return targets, nil
}
