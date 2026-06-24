package solfundmcmpdas

import (
	"fmt"

	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

// SeqFundSolanaMCMPDAsInput is the input for the low-level Solana fund-mcm-pdas sequence.
type SeqFundSolanaMCMPDAsInput struct {
	ChainSelector uint64          `json:"chainSelector"`
	Targets       []FundingTarget `json:"targets"`
}

// SeqFundSolanaMCMPDAs funds each provided signer PDA on a Solana chain.
var SeqFundSolanaMCMPDAs = operations.NewSequence(
	"seq-solana-fund-mcms-pdas",
	&semvers.V1_0_0,
	"Funds MCMS signer PDAs on a Solana chain",
	func(b operations.Bundle, deps cldfsol.Chain, in SeqFundSolanaMCMPDAsInput) (sequenceutils.OnChainOutput, error) {
		if in.ChainSelector != deps.Selector {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("mismatch between input chain selector and selector defined within dependencies: %d != %d", in.ChainSelector, deps.Selector)
		}

		for i, target := range in.Targets {
			if target.Amount == 0 {
				continue
			}

			_, err := operations.ExecuteOperation(
				b,
				OpFundKey,
				deps,
				OpFundKeyInput{
					Target: target.Address,
					Amount: target.Amount,
				},
			)
			if err != nil {
				return sequenceutils.OnChainOutput{}, fmt.Errorf("fund target[%d]: %w", i, err)
			}
		}

		return sequenceutils.OnChainOutput{}, nil
	},
)

// SeqFundMCMPDAs resolves MCMS signer PDAs from the datastore and funds them on a Solana chain.
var SeqFundMCMPDAs = operations.NewSequence(
	"seq-solana-fund-mcm-pdas",
	&semvers.V1_0_0,
	"Funds MCMS signer PDAs on a Solana chain from datastore refs",
	func(b operations.Bundle, deps Deps, in ChainInput) (sequenceutils.OnChainOutput, error) {
		chain, ok := deps.BlockChains.SolanaChains()[in.ChainSelector]
		if !ok {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("solana chain %d not found in environment", in.ChainSelector)
		}

		env := EnvFromDeps(deps)
		targets, err := ResolveFundingTargets(env, in.ChainSelector, in.FundingConfig)
		if err != nil {
			return sequenceutils.OnChainOutput{}, err
		}

		seqReport, err := operations.ExecuteSequence(
			b,
			SeqFundSolanaMCMPDAs,
			chain,
			SeqFundSolanaMCMPDAsInput{
				ChainSelector: in.ChainSelector,
				Targets:       targets,
			},
		)
		if err != nil {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("failed to execute Solana fund-mcm-pdas sequence: %w", err)
		}

		return seqReport.Output, nil
	},
)
