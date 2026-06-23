package transfertomcms

import (
	"errors"
	"fmt"
	"slices"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/internal/maputil"
)

var _ cldf.ChangeSetV2[Input] = Changeset{}

// Changeset transfers ownable contract ownership to the MCMS timelock.
// transferOwnership is sent on-chain by the deployer; acceptOwnership is
// returned as batch operations in an MCMS timelock proposal. MCMS refs are
// resolved from the datastore only.
type Changeset struct{}

func (Changeset) VerifyPreconditions(env cldf.Environment, input Input) error {
	if env.DataStore == nil {
		return errors.New("datastore is required for transfer to MCMS")
	}
	if input.MCMS == nil {
		return errors.New("MCMS timelock proposal input is required")
	}
	if err := input.MCMS.Validate(); err != nil {
		return fmt.Errorf("invalid MCMS timelock proposal input: %w", err)
	}
	if len(input.Cfg.ContractsByChain) == 0 {
		return errors.New("no contracts provided")
	}

	byFamily, err := groupByFamily(input)
	if err != nil {
		return err
	}

	families := make([]string, 0, len(byFamily))
	for family := range byFamily {
		families = append(families, family)
	}
	slices.Sort(families)

	for _, family := range families {
		if err := VerifyForFamily(family, env, byFamily[family]); err != nil {
			return err
		}
	}

	return nil
}

func (Changeset) Apply(env cldf.Environment, input Input) (cldf.ChangesetOutput, error) {
	if input.MCMS == nil {
		return cldf.ChangesetOutput{}, errors.New("MCMS timelock proposal input is required")
	}

	deps := Deps{
		BlockChains: env.BlockChains,
		DataStore:   env.DataStore,
	}

	var agg sequenceutils.OnChainOutput

	for _, chainSelector := range maputil.SortedMapKeys(input.Cfg.ContractsByChain) {
		contracts := input.Cfg.ContractsByChain[chainSelector]

		seq, seqErr := SequenceForChainSelector(chainSelector)
		if seqErr != nil {
			return buildOutput(env, input.MCMS, agg, fmt.Errorf("chain selector %d: %w", chainSelector, seqErr))
		}

		var mergeErr error
		agg, mergeErr = sequenceutils.ExecuteOnChainSequenceAndMerge(
			env.OperationsBundle,
			deps,
			seq,
			ChainInput{
				ChainSelector:       chainSelector,
				Contracts:           contracts,
				OnlyAcceptOwnership: input.Cfg.OnlyAcceptOwnership,
				MCMS:                input.MCMS,
			},
			agg,
		)
		if mergeErr != nil {
			return buildOutput(env, input.MCMS, agg, mergeErr)
		}
	}

	return buildOutput(env, input.MCMS, agg, nil)
}

func buildOutput(
	env cldf.Environment,
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

	builder := cldf.NewOutputBuilder(env, ds).
		WithTimelockProposal(*mcmsInput, agg.BatchOps)

	out, buildErr := builder.Build()
	if buildErr != nil {
		return out, fmt.Errorf("build changeset output: %w", buildErr)
	}

	if len(out.MCMSTimelockProposals) > 0 {
		env.Logger.Infow("Transfer to MCMS proposal created", "proposalCount", len(out.MCMSTimelockProposals))
	}

	return out, nil
}

func groupByFamily(input Input) (map[string][]ChainInput, error) {
	byFamily := make(map[string][]ChainInput)
	for chainSelector, contracts := range input.Cfg.ContractsByChain {
		if len(contracts) == 0 {
			return nil, fmt.Errorf("chain %d: no contracts provided", chainSelector)
		}
		family, err := chainselectors.GetSelectorFamily(chainSelector)
		if err != nil {
			return nil, fmt.Errorf("chain selector %d: %w", chainSelector, err)
		}
		byFamily[family] = append(byFamily[family], ChainInput{
			ChainSelector:       chainSelector,
			Contracts:           contracts,
			OnlyAcceptOwnership: input.Cfg.OnlyAcceptOwnership,
			MCMS:                input.MCMS,
		})
	}

	return byFamily, nil
}
