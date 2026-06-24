package firedrill

import (
	"errors"
	"fmt"
	"slices"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

var _ cldf.ChangeSetV2[Input] = Changeset{}

// Changeset creates an MCMS signing fire-drill proposal with noop operations per chain.
// It exercises signing and execution pipelines without mutating on-chain configuration.
type Changeset struct{}

// ResolvedSelectors returns the chain selectors VerifyPreconditions and Apply will use.
// When cfg.Selectors is empty, it defaults to every Solana chain in the environment followed by every EVM chain.
func (cfg Config) ResolvedSelectors(e cldf.Environment) []uint64 {
	return resolvedSelectors(e, cfg.Selectors)
}

func (Changeset) VerifyPreconditions(e cldf.Environment, input Input) error {
	if e.DataStore == nil {
		return errors.New("datastore is required for MCMS fire drill")
	}
	if input.MCMS == nil {
		return errors.New("MCMS timelock proposal input is required")
	}
	if err := input.MCMS.Validate(); err != nil {
		return fmt.Errorf("invalid MCMS timelock proposal input: %w", err)
	}

	selectors := resolvedSelectors(e, input.Cfg.Selectors)
	if len(selectors) == 0 {
		return errors.New("no chain selectors resolved for MCMS fire drill")
	}

	byFamily := make(map[string][]ChainInput)
	for _, chainSelector := range selectors {
		family, err := chainselectors.GetSelectorFamily(chainSelector)
		if err != nil {
			return err
		}
		byFamily[family] = append(byFamily[family], ChainInput{
			ChainSelector: chainSelector,
			MCMS:          *input.MCMS,
		})
	}

	families := make([]string, 0, len(byFamily))
	for family := range byFamily {
		families = append(families, family)
	}
	slices.Sort(families)

	for _, family := range families {
		if err := Registry.VerifyForFamily(family, e, byFamily[family]); err != nil {
			return err
		}
	}

	return nil
}

func (Changeset) Apply(e cldf.Environment, input Input) (cldf.ChangesetOutput, error) {
	if input.MCMS == nil {
		return cldf.ChangesetOutput{}, errors.New("MCMS timelock proposal input is required")
	}

	selectors := resolvedSelectors(e, input.Cfg.Selectors)
	if len(selectors) == 0 {
		return cldf.ChangesetOutput{}, errors.New("no chain selectors resolved for MCMS fire drill")
	}

	deps := Deps{
		BlockChains: e.BlockChains,
		DataStore:   e.DataStore,
	}

	var agg sequenceutils.OnChainOutput

	for _, chainSelector := range selectors {
		seq, seqErr := Registry.SequenceForChainSelector(chainSelector)
		if seqErr != nil {
			return buildOutput(e, input.MCMS, agg, fmt.Errorf("chain selector %d: %w", chainSelector, seqErr))
		}

		var mergeErr error
		agg, mergeErr = sequenceutils.ExecuteOnChainSequenceAndMerge(
			e.OperationsBundle,
			deps,
			seq,
			ChainInput{
				ChainSelector: chainSelector,
				MCMS:          *input.MCMS,
			},
			agg,
		)
		if mergeErr != nil {
			return buildOutput(e, input.MCMS, agg, mergeErr)
		}
	}

	return buildOutput(e, input.MCMS, agg, nil)
}

func buildOutput(
	e cldf.Environment,
	mcmsInput *cldf.MCMSTimelockProposalInput,
	agg sequenceutils.OnChainOutput,
	err error,
) (cldf.ChangesetOutput, error) {
	ds := cldfdatastore.NewMemoryDataStore()
	if metaErr := ds.WriteMetadata(agg.Metadata); metaErr != nil {
		return cldf.ChangesetOutput{DataStore: ds},
			fmt.Errorf("write metadata to datastore: %w", metaErr)
	}

	partialOutput := cldf.ChangesetOutput{DataStore: ds}
	if err != nil {
		return partialOutput, err
	}

	builder := cldf.NewOutputBuilder(e, ds).
		WithTimelockProposal(*mcmsInput, agg.BatchOps)

	out, buildErr := builder.Build()
	if buildErr != nil {
		return out, fmt.Errorf("build changeset output: %w", buildErr)
	}

	if len(out.MCMSTimelockProposals) > 0 {
		e.Logger.Infow("MCMS fire drill proposal created", "proposalCount", len(out.MCMSTimelockProposals))
	}

	return out, nil
}

func resolvedSelectors(e cldf.Environment, selectors []uint64) []uint64 {
	if len(selectors) > 0 {
		return selectors
	}

	solSelectors := e.BlockChains.ListChainSelectors(cldf_chain.WithFamily(chainselectors.FamilySolana))
	evmSelectors := e.BlockChains.ListChainSelectors(cldf_chain.WithFamily(chainselectors.FamilyEVM))
	out := make([]uint64, 0, len(solSelectors)+len(evmSelectors))
	out = append(out, solSelectors...)
	out = append(out, evmSelectors...)

	return out
}
