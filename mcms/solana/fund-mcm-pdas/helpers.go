package solfundmcmpdas

import (
	"fmt"

	solanago "github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

// FundingTarget is a signer PDA and the lamports to send to it.
type FundingTarget struct {
	Address solanago.PublicKey `json:"address"`
	Amount  uint64             `json:"amount"`
}

// ResolveFundingTargets resolves MCMS and timelock signer PDAs from the environment datastore.
func ResolveFundingTargets(e cldf.Environment, chainSelector uint64, cfg FundingConfig) ([]FundingTarget, error) {
	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilySolana)
	if !ok {
		return nil, fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilySolana)
	}

	qualifier := cfg.Qualifier
	timelockRef, err := reader.GetTimelockRef(e, chainSelector, cldf.MCMSTimelockProposalInput{Qualifier: qualifier})
	if err != nil {
		return nil, fmt.Errorf("resolve timelock ref for chain %d: %w", chainSelector, err)
	}
	proposerRef, err := reader.GetMCMSRef(e, chainSelector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		Qualifier:      qualifier,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve proposer ref for chain %d: %w", chainSelector, err)
	}
	cancellerRef, err := reader.GetMCMSRef(e, chainSelector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionCancel,
		Qualifier:      qualifier,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve canceller ref for chain %d: %w", chainSelector, err)
	}
	bypasserRef, err := reader.GetMCMSRef(e, chainSelector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionBypass,
		Qualifier:      qualifier,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve bypasser ref for chain %d: %w", chainSelector, err)
	}

	timelockSigner, err := timelockSignerPDAFromRef(timelockRef.Address)
	if err != nil {
		return nil, fmt.Errorf("parse timelock signer PDA for chain %d: %w", chainSelector, err)
	}
	proposerSigner, err := mcmsSignerPDAFromRef(proposerRef.Address)
	if err != nil {
		return nil, fmt.Errorf("parse proposer signer PDA for chain %d: %w", chainSelector, err)
	}
	cancellerSigner, err := mcmsSignerPDAFromRef(cancellerRef.Address)
	if err != nil {
		return nil, fmt.Errorf("parse canceller signer PDA for chain %d: %w", chainSelector, err)
	}
	bypasserSigner, err := mcmsSignerPDAFromRef(bypasserRef.Address)
	if err != nil {
		return nil, fmt.Errorf("parse bypasser signer PDA for chain %d: %w", chainSelector, err)
	}

	return []FundingTarget{
		{Address: timelockSigner, Amount: cfg.Timelock},
		{Address: proposerSigner, Amount: cfg.ProposeMCM},
		{Address: cancellerSigner, Amount: cfg.CancellerMCM},
		{Address: bypasserSigner, Amount: cfg.BypasserMCM},
	}, nil
}

func timelockSignerPDAFromRef(address string) (solanago.PublicKey, error) {
	program, seed, err := mcmssolanasdk.ParseContractAddress(address)
	if err != nil {
		return solanago.PublicKey{}, err
	}

	var pdaSeed legacysolana.PDASeed
	copy(pdaSeed[:], seed[:])

	return familysolana.GetTimelockSignerPDA(program, pdaSeed), nil
}

func mcmsSignerPDAFromRef(address string) (solanago.PublicKey, error) {
	program, seed, err := mcmssolanasdk.ParseContractAddress(address)
	if err != nil {
		return solanago.PublicKey{}, err
	}

	var pdaSeed legacysolana.PDASeed
	copy(pdaSeed[:], seed[:])

	return familysolana.GetMCMSignerPDA(program, pdaSeed), nil
}
