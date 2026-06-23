package solfundmcmpdas

import (
	"fmt"

	"github.com/gagliardetto/solana-go/rpc"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

func validateMCMSRefs(e cldf.Environment, chainSelector uint64, cfg FundingConfig) error {
	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilySolana)
	if !ok {
		return fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilySolana)
	}

	qualifier := cfg.Qualifier
	if _, err := reader.GetTimelockRef(e, chainSelector, cldf.MCMSTimelockProposalInput{Qualifier: qualifier}); err != nil {
		return fmt.Errorf("validate timelock ref for chain %d: %w", chainSelector, err)
	}

	for _, action := range []mcmstypes.TimelockAction{
		mcmstypes.TimelockActionSchedule,
		mcmstypes.TimelockActionCancel,
		mcmstypes.TimelockActionBypass,
	} {
		if _, err := reader.GetMCMSRef(e, chainSelector, cldf.MCMSTimelockProposalInput{
			TimelockAction: action,
			Qualifier:      qualifier,
		}); err != nil {
			return fmt.Errorf("validate mcms ref for chain %d action %q: %w", chainSelector, action, err)
		}
	}

	if _, err := ResolveFundingTargets(e, chainSelector, cfg); err != nil {
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
