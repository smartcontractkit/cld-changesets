package changesets

import (
	"errors"
	"fmt"

	"github.com/smartcontractkit/mcms"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	chainsel "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"

	mcops "github.com/smartcontractkit/cld-changesets/mcms/operations"
	evmstate "github.com/smartcontractkit/cld-changesets/pkg/family/evm"
)

var _ cldf.ChangeSetV2[FireDrillConfig] = MCMSSignFireDrillChangeset{}

// FireDrillConfig selects chains and MCMS timelock routing for a signing fire drill.
type FireDrillConfig struct {
	TimelockCfg cldfproposalutils.TimelockConfig `json:"timelockCfg"`
	Selectors   []uint64                         `json:"selectors,omitempty"`
}

// MCMSSignFireDrillChangeset creates an MCMS signing fire-drill proposal with noop operations per chain.
// It exercises signing and execution pipelines without mutating on-chain configuration.
type MCMSSignFireDrillChangeset struct{}

// ResolvedSelectors returns the chain selectors VerifyPreconditions and the fire-drill operation will use.
// When cfg.Selectors is empty, it defaults to every Solana chain in the environment followed by every EVM chain.
func (cfg FireDrillConfig) ResolvedSelectors(e cldf.Environment) []uint64 {
	return cfg.resolvedSelectors(e)
}

// VerifyPreconditions ensures each target chain exists and MCMS timelock state satisfies the configured action.
func (MCMSSignFireDrillChangeset) VerifyPreconditions(e cldf.Environment, cfg FireDrillConfig) error {
	selectors := cfg.ResolvedSelectors(e)
	if len(selectors) == 0 {
		return errors.New("no chain selectors resolved for MCMS fire drill")
	}

	for _, selector := range selectors {
		family, err := chainsel.GetSelectorFamily(selector)
		if err != nil {
			return err
		}

		switch family {
		case chainsel.FamilyEVM:
			ch, ok := e.BlockChains.EVMChains()[selector]
			if !ok {
				return fmt.Errorf("evm chain %d not found in environment", selector)
			}

			addresses, err := e.ExistingAddresses.AddressesForChain(selector) //nolint:staticcheck // SA1019
			if err != nil {
				return fmt.Errorf("addresses for chain %d: %w", selector, err)
			}

			st, err := evmstate.MaybeLoadMCMSWithTimelockChainState(ch, addresses)
			if err != nil {
				return fmt.Errorf("load MCMS timelock state for chain %d: %w", selector, err)
			}

			if err := cfg.TimelockCfg.Validate(ch, st); err != nil {
				return fmt.Errorf("timelock config for chain %d: %w", selector, err)
			}

		case chainsel.FamilySolana:
			if _, ok := e.BlockChains.SolanaChains()[selector]; !ok {
				return fmt.Errorf("solana chain %d not found in environment", selector)
			}

			if err := cfg.TimelockCfg.ValidateSolana(e, selector); err != nil {
				return fmt.Errorf("timelock config for chain %d: %w", selector, err)
			}

		default:
			return fmt.Errorf("unsupported chain family for selector %d", selector)
		}
	}

	return nil
}

// Apply builds the fire-drill proposal via the operations API (with force execute for repeatable drills).
func (MCMSSignFireDrillChangeset) Apply(e cldf.Environment, cfg FireDrillConfig) (cldf.ChangesetOutput, error) {
	deps := mcops.FireDrillDeps{Environment: e}
	input := mcops.FireDrillInput{TimelockCfg: cfg.TimelockCfg, Selectors: cfg.Selectors}

	report, err := fwops.ExecuteOperation[mcops.FireDrillInput, mcops.FireDrillOutput, mcops.FireDrillDeps](
		e.OperationsBundle,
		mcops.BuildMCMSFiredrillProposalOp,
		deps,
		input,
		fwops.WithForceExecute[mcops.FireDrillInput, mcops.FireDrillDeps](),
	)
	out := cldf.ChangesetOutput{
		Reports: []fwops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}

	out.MCMSTimelockProposals = []mcms.TimelockProposal{report.Output.Proposal}

	return out, nil
}

func (cfg FireDrillConfig) resolvedSelectors(e cldf.Environment) []uint64 {
	if len(cfg.Selectors) > 0 {
		return cfg.Selectors
	}
	solSelectors := e.BlockChains.ListChainSelectors(cldf_chain.WithFamily(chainsel.FamilySolana))
	evmSelectors := e.BlockChains.ListChainSelectors(cldf_chain.WithFamily(chainsel.FamilyEVM))
	out := make([]uint64, 0, len(solSelectors)+len(evmSelectors))
	out = append(out, solSelectors...)
	out = append(out, evmSelectors...)

	return out
}
