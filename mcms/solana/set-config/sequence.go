package solsetconfig

import (
	"fmt"

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

// SeqSolanaSetConfigInput is the input for the generic Solana set-config sequence.
type SeqSolanaSetConfigInput struct {
	ChainSelector    uint64               `json:"chainSelector"`
	NoSend           bool                 `json:"noSend"`
	AuthorityAccount solanago.PublicKey   `json:"authorityAccount"`
	Targets          []MCMSetConfigTarget `json:"targets"`
}

// SeqSolanaSetConfig sets config on each provided Solana MCM account via OpSolanaSetConfigMCM.
// When NoSend is true, one batch operation is returned per target to respect Solana transaction size limits.
var SeqSolanaSetConfig = operations.NewSequence(
	"seq-solana-mcm-set-config",
	&semvers.V1_0_0,
	"Sets MCMS config on one or more Solana MCM accounts",
	func(b operations.Bundle, deps cldfsol.Chain, in SeqSolanaSetConfigInput) (sequenceutils.OnChainOutput, error) {
		if in.ChainSelector != deps.Selector {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("mismatch between inputted chain selector and selector defined within dependencies: %d != %d", in.ChainSelector, deps.Selector)
		}

		var batchOps []mcmstypes.BatchOperation
		if in.NoSend {
			batchOps = make([]mcmstypes.BatchOperation, 0, len(in.Targets))
		}

		for _, target := range in.Targets {
			opReport, err := operations.ExecuteOperation(
				b,
				OpSolanaSetConfigMCM,
				deps,
				OpSolanaSetConfigInput{
					Target:           target,
					NoSend:           in.NoSend,
					AuthorityAccount: in.AuthorityAccount,
				},
			)
			if err != nil {
				return sequenceutils.OnChainOutput{}, err
			}

			if in.NoSend {
				batchOps = append(batchOps, opReport.Output.BatchOperation)
			}
		}

		return sequenceutils.OnChainOutput{BatchOps: batchOps}, nil
	},
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

	seqReport, err := operations.ExecuteSequence(
		b,
		SeqSolanaSetConfig,
		chain,
		SeqSolanaSetConfigInput{
			ChainSelector:    in.ChainSelector,
			NoSend:           useMCMS,
			AuthorityAccount: authorityAccount,
			Targets:          targets,
		},
	)
	if err != nil {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("failed to execute Solana set config sequence: %w", err)
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

		targets = append(targets, MCMSetConfigTarget{
			Address:      ref.Address,
			Config:       cfg.Config,
			ContractType: string(ref.Type),
		})
	}

	return targets, nil
}
