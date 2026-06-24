package fundmcmpdas

import (
	"fmt"

	"github.com/gagliardetto/solana-go/rpc"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	solfundmcmpdas "github.com/smartcontractkit/cld-changesets/mcms/solana/fund-mcm-pdas"
)

func validateMCMSRefs(e cldf.Environment, chainSelector uint64, cfg FundingConfig) error {
	if _, err := solfundmcmpdas.ResolveFundingTargets(e, chainSelector, cfg); err != nil {
		return fmt.Errorf("validate funding targets for chain %d: %w", chainSelector, err)
	}

	return nil
}

func validateDeployerBalance(e cldf.Environment, chainSelector uint64, cfg FundingConfig) error {
	chain := e.BlockChains.SolanaChains()[chainSelector]
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
