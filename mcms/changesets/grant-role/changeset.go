package grantrole

import (
	"errors"
	"fmt"
	"slices"

	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/internal/maputil"
)

var _ cldf.ChangeSetV2[Input] = Changeset{}

// Changeset grants RBACTimelock roles across configured chains.
type Changeset struct{}

func (Changeset) VerifyPreconditions(env cldf.Environment, input Input) error {
	if env.DataStore == nil {
		return errors.New("datastore is required for grant-role")
	}
	if input.MCMS != nil {
		if err := input.MCMS.Validate(); err != nil {
			return fmt.Errorf("invalid MCMS timelock proposal input: %w", err)
		}
	}
	if len(input.Cfg.GrantsByChain) == 0 {
		return errors.New("no role grants provided")
	}
	if err := validateGrants(input.Cfg.GrantsByChain); err != nil {
		return err
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
		if err := Registry.VerifyForFamily(family, env, byFamily[family]); err != nil {
			return err
		}
	}

	return nil
}

func (Changeset) Apply(env cldf.Environment, input Input) (cldf.ChangesetOutput, error) {
	deps := Deps{
		BlockChains: env.BlockChains,
		DataStore:   env.DataStore,
	}

	var agg sequenceutils.OnChainOutput
	for _, chainSelector := range maputil.SortedMapKeys(input.Cfg.GrantsByChain) {
		grants := input.Cfg.GrantsByChain[chainSelector]

		seq, seqErr := Registry.SequenceForChainSelector(chainSelector)
		if seqErr != nil {
			return buildOutput(env, input.MCMS, agg, fmt.Errorf("chain selector %d: %w", chainSelector, seqErr))
		}

		var mergeErr error
		agg, mergeErr = sequenceutils.ExecuteOnChainSequenceAndMerge(
			env.OperationsBundle,
			deps,
			seq,
			SeqInput{
				ChainSelector:  chainSelector,
				Grants:         grants,
				MCMS:           input.MCMS,
				GasBoostConfig: input.Cfg.GasBoostConfig,
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

	builder := cldf.NewOutputBuilder(env, ds)
	if mcmsInput != nil {
		builder = builder.WithTimelockProposal(*mcmsInput, agg.BatchOps)
	}

	out, buildErr := builder.Build()
	if buildErr != nil {
		return out, fmt.Errorf("build changeset output: %w", buildErr)
	}

	if mcmsInput != nil && len(out.MCMSTimelockProposals) > 0 {
		env.Logger.Infow("GrantRole proposal created", "proposalCount", len(out.MCMSTimelockProposals))
	}

	return out, nil
}

func validateGrants(grantsByChain map[uint64][]RoleGrant) error {
	for chainSelector, grants := range grantsByChain {
		if len(grants) == 0 {
			return fmt.Errorf("chain %d: no role grants provided", chainSelector)
		}
		seen := make(map[string]struct{})
		for grantIdx, grant := range grants {
			if !grant.Role.Valid() {
				return fmt.Errorf("chain %d grants[%d]: unsupported timelock role %s", chainSelector, grantIdx, grant.Role.String())
			}
			if len(grant.Addresses) == 0 {
				return fmt.Errorf("chain %d grants[%d]: no addresses provided", chainSelector, grantIdx)
			}
			for addrIdx, addr := range grant.Addresses {
				if addr == "" {
					return fmt.Errorf("chain %d grants[%d].addresses[%d]: address must not be empty", chainSelector, grantIdx, addrIdx)
				}
				key := grant.Role.String() + ":" + addr
				if _, ok := seen[key]; ok {
					return fmt.Errorf("chain %d grants[%d].addresses[%d]: duplicate grant for role %s and address %s",
						chainSelector, grantIdx, addrIdx, grant.Role.String(), addr)
				}
				seen[key] = struct{}{}
			}
		}
	}

	return nil
}
