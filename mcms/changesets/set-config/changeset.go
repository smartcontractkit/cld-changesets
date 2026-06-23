package setconfig

import (
	"errors"
	"fmt"
	"slices"

	chain_selectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/internal/maputil"
)

var _ cldf.ChangeSetV2[Input] = Changeset{}

// Config holds the set-config targets for the changeset.
type Config struct {
	Targets []ContractSetConfig
}

// Input is the changeset configuration with optional MCMS timelock proposal settings.
type Input = sequenceutils.WithMCMS[Config]

// Changeset sets MCMS config on contracts identified by datastore refs.
type Changeset struct{}

func (Changeset) VerifyPreconditions(env cldf.Environment, input Input) error {
	if input.MCMS != nil {
		if err := input.MCMS.Validate(); err != nil {
			return fmt.Errorf("invalid MCMS timelock proposal input: %w", err)
		}
	}

	return validateConfig(env, input.Cfg, input.MCMS)
}

func (Changeset) Apply(e cldf.Environment, input Input) (cldf.ChangesetOutput, error) {
	byChain, err := groupTargetsByChain(input.Cfg.Targets)
	if err != nil {
		return cldf.ChangesetOutput{}, err
	}

	deps := Deps{
		BlockChains: e.BlockChains,
		DataStore:   e.DataStore,
	}

	var agg sequenceutils.OnChainOutput

	for _, chainSelector := range maputil.SortedMapKeys(byChain) {
		targets := byChain[chainSelector]
		seq, seqErr := SequenceForChainSelector(chainSelector)
		if seqErr != nil {
			return buildSetConfigOutput(e, input.MCMS, agg, fmt.Errorf("chain selector %d: %w", chainSelector, seqErr))
		}

		var mergeErr error
		agg, mergeErr = sequenceutils.ExecuteOnChainSequenceAndMerge(
			e.OperationsBundle,
			deps,
			seq,
			ChainInput{
				ChainSelector: chainSelector,
				Targets:       targets,
				MCMS:          input.MCMS,
			},
			agg,
		)
		if mergeErr != nil {
			return buildSetConfigOutput(e, input.MCMS, agg, mergeErr)
		}
	}

	return buildSetConfigOutput(e, input.MCMS, agg, nil)
}

func buildSetConfigOutput(
	e cldf.Environment,
	mcmsInput *cldf.MCMSTimelockProposalInput,
	agg sequenceutils.OnChainOutput,
	err error,
) (cldf.ChangesetOutput, error) {
	ds := datastore.NewMemoryDataStore()
	if metaErr := ds.WriteMetadata(agg.Metadata); metaErr != nil {
		return cldf.ChangesetOutput{DataStore: ds},
			fmt.Errorf("failed to write metadata to datastore: %w", metaErr)
	}

	partialOutput := cldf.ChangesetOutput{DataStore: ds}
	if err != nil {
		return partialOutput, err
	}

	builder := cldf.NewOutputBuilder(e, ds)
	if mcmsInput != nil {
		builder = builder.WithTimelockProposal(*mcmsInput, agg.BatchOps)
	}

	out, buildErr := builder.Build()
	if buildErr != nil {
		return out, fmt.Errorf("build changeset output: %w", buildErr)
	}

	if mcmsInput != nil && len(out.MCMSTimelockProposals) > 0 {
		e.Logger.Infow("SetConfigMCMS proposal created", "proposalCount", len(out.MCMSTimelockProposals))
	}

	return out, nil
}

func validateConfig(e cldf.Environment, cfg Config, mcms *cldf.MCMSTimelockProposalInput) error {
	if len(cfg.Targets) == 0 {
		return errors.New("no set-config targets provided")
	}

	byChain, err := groupTargetsByChain(cfg.Targets)
	if err != nil {
		return err
	}

	byFamily := make(map[string][]ChainInput)
	for _, chainSelector := range maputil.SortedMapKeys(byChain) {
		targets := byChain[chainSelector]
		family, err := chain_selectors.GetSelectorFamily(chainSelector)
		if err != nil {
			return err
		}
		byFamily[family] = append(byFamily[family], ChainInput{
			ChainSelector: chainSelector,
			Targets:       targets,
			MCMS:          mcms,
		})
	}

	families := make([]string, 0, len(byFamily))
	for family := range byFamily {
		families = append(families, family)
	}
	slices.Sort(families)

	for _, family := range families {
		if err := VerifyForFamily(family, e, byFamily[family]); err != nil {
			return err
		}
	}

	for i, target := range cfg.Targets {
		if target.Ref.ChainSelector == 0 {
			return fmt.Errorf("targets[%d]: ref chain selector is required", i)
		}
		if _, err := target.Ref.Resolve(e); err != nil {
			return fmt.Errorf("targets[%d]: %w", i, err)
		}
		if err := target.Config.Validate(); err != nil {
			return fmt.Errorf("targets[%d]: invalid config: %w", i, err)
		}
	}

	return nil
}

func groupTargetsByChain(targets []ContractSetConfig) (map[uint64][]ContractSetConfig, error) {
	byChain := make(map[uint64][]ContractSetConfig)
	for i, target := range targets {
		if target.Ref.ChainSelector == 0 {
			return nil, fmt.Errorf("targets[%d]: ref chain selector is required", i)
		}
		byChain[target.Ref.ChainSelector] = append(byChain[target.Ref.ChainSelector], target)
	}

	return byChain, nil
}
