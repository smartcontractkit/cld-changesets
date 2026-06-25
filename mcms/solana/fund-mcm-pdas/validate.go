package solfundmcmpdas

import (
	"fmt"

	"github.com/gagliardetto/solana-go/rpc"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

func validateMCMSRefs(e cldf.Environment, chainSelector uint64, cfg FundingConfig) error {
	if _, err := ResolveFundingTargets(e, chainSelector, cfg); err != nil {
		return fmt.Errorf("validate funding targets for chain %d: %w", chainSelector, err)
	}

	return nil
}

func validateDeployerBalance(e cldf.Environment, chainSelector uint64, cfg FundingConfig) error {
	chain, ok := e.BlockChains.SolanaChains()[chainSelector]
	if !ok {
		return fmt.Errorf("solana chain %d not found in environment", chainSelector)
	}
	if chain.Client == nil {
		return fmt.Errorf("solana client missing for chain %d", chainSelector)
	}
	if chain.DeployerKey == nil {
		return fmt.Errorf("deployer key missing for chain %d", chainSelector)
	}

	result, err := chain.Client.GetBalance(e.GetContext(), chain.DeployerKey.PublicKey(), rpc.CommitmentConfirmed)
	if err != nil {
		return fmt.Errorf("failed to get deployer balance for chain %d: %w", chainSelector, err)
	}

	requiredAmount := cfg.RequiredFunding()
	if result.Value < requiredAmount {
		return fmt.Errorf("deployer balance is insufficient for chain %d, required: %d, actual: %d", chainSelector, requiredAmount, result.Value)
	}

	return nil
}
