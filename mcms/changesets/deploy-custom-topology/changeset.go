package deploycustomtopology

import (
	"errors"
	"fmt"
	"slices"

	chain_selectors "github.com/smartcontractkit/chain-selectors"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/maputil"
)

var _ cldf.ChangeSetV2[Input] = Changeset{}

// Changeset deploys a custom MCMS topology (arbitrary MCMs + timelocks, custom
// role wiring, optional ownership transfers) per chain, dispatching to the
// registered family sequence for each chain. New addresses are written to the
// datastore; accept-ownership calls are returned as MCMS timelock proposals.
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
	deps := Deps{
		BlockChains: e.BlockChains,
		DataStore:   e.DataStore,
	}

	var agg ChainOutput
	var reports []operations.Report[any, any]

	for _, chainSelector := range maputil.SortedMapKeys(input.Cfg.ChainConfigs) {
		seq, seqErr := SequenceForChainSelector(chainSelector)
		if seqErr != nil {
			return buildOutput(e, input.MCMS, agg, reports, fmt.Errorf("chain selector %d: %w", chainSelector, seqErr))
		}

		report, runErr := operations.ExecuteSequence(
			e.OperationsBundle,
			seq,
			deps,
			ChainInput{
				ChainSelector: chainSelector,
				Config:        input.Cfg.ChainConfigs[chainSelector],
				ExtraArgs:     input.Cfg.ChainConfigs[chainSelector].ExtraArgs,
				MCMS:          input.MCMS,
			},
		)
		reports = append(reports, report.ExecutionReports...)
		if runErr != nil {
			return buildOutput(e, input.MCMS, agg, reports, fmt.Errorf("chain selector %d: %w", chainSelector, runErr))
		}

		agg = mergeChainOutputs(agg, report.Output)
	}

	if len(agg.ProposalGroups) > 0 && input.MCMS == nil {
		return buildOutput(e, input.MCMS, agg, reports,
			errors.New("sequences returned accept-ownership batch operations but no MCMS input was provided"))
	}

	return buildOutput(e, input.MCMS, agg, reports, nil)
}

// buildOutput writes deployed-address metadata to a fresh datastore and, when an
// MCMS input is present, builds one timelock proposal per timelock qualifier from
// the aggregated accept-ownership batch operations.
func buildOutput(
	e cldf.Environment,
	mcmsInput *cldf.MCMSTimelockProposalInput,
	agg ChainOutput,
	reports []operations.Report[any, any],
	runErr error,
) (cldf.ChangesetOutput, error) {
	outDS := cldfdatastore.NewMemoryDataStore()
	if metaErr := outDS.WriteMetadata(agg.Metadata); metaErr != nil {
		return cldf.ChangesetOutput{DataStore: outDS, Reports: reports},
			fmt.Errorf("failed to write metadata to datastore: %w", metaErr)
	}

	if runErr != nil {
		return cldf.ChangesetOutput{DataStore: outDS, Reports: reports}, runErr
	}

	// Resolve proposals against an environment view that includes the freshly
	// deployed timelock/MCM addresses (the original env datastore does not yet
	// contain them). The output datastore keeps only the delta.
	resolveEnv := e
	if mcmsInput != nil {
		resolveDS := cldfdatastore.NewMemoryDataStore()
		if e.DataStore != nil {
			if mergeErr := resolveDS.Merge(e.DataStore); mergeErr != nil {
				return cldf.ChangesetOutput{DataStore: outDS, Reports: reports},
					fmt.Errorf("failed to merge environment datastore: %w", mergeErr)
			}
		}
		if metaErr := resolveDS.WriteMetadata(agg.Metadata); metaErr != nil {
			return cldf.ChangesetOutput{DataStore: outDS, Reports: reports},
				fmt.Errorf("failed to stage deployed addresses for proposal resolution: %w", metaErr)
		}
		resolveEnv.DataStore = resolveDS.Seal()
	}

	builder := cldf.NewOutputBuilder(resolveEnv, outDS).WithOperationsReports(reports)
	if mcmsInput != nil {
		for _, qualifier := range sortedQualifiers(agg.ProposalGroups) {
			input := *mcmsInput
			input.Qualifier = qualifier
			builder = builder.WithTimelockProposal(input, batchOpsForQualifier(agg.ProposalGroups, qualifier))
		}
	}

	out, buildErr := builder.Build()
	if buildErr != nil {
		return out, fmt.Errorf("build changeset output: %w", buildErr)
	}

	if mcmsInput != nil && len(out.MCMSTimelockProposals) > 0 {
		e.Logger.Infow("DeployCustomTopology accept-ownership proposals created",
			"proposalCount", len(out.MCMSTimelockProposals))
	}

	return out, nil
}

func validateConfig(e cldf.Environment, cfg Config, mcms *cldf.MCMSTimelockProposalInput) error {
	if len(cfg.ChainConfigs) == 0 {
		return errors.New("no chain configs provided")
	}

	byFamily := make(map[string][]ChainInput)
	for _, chainSelector := range maputil.SortedMapKeys(cfg.ChainConfigs) {
		chainCfg := cfg.ChainConfigs[chainSelector]
		family, err := chain_selectors.GetSelectorFamily(chainSelector)
		if err != nil {
			return fmt.Errorf("chain selector %d: %w", chainSelector, err)
		}
		byFamily[family] = append(byFamily[family], ChainInput{
			ChainSelector: chainSelector,
			Config:        chainCfg,
			ExtraArgs:     chainCfg.ExtraArgs,
			MCMS:          mcms,
		})

		if err := validateChainConfig(chainSelector, chainCfg, mcms); err != nil {
			return err
		}
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

	return nil
}

// validateChainConfig checks a single chain's topology: unique refs within MCMs
// and timelocks (with no overlap between the two), every referenced MCMRef is
// declared, every RoleHolder is well-formed, and ownership transfers require an
// MCMS input.
func validateChainConfig(chainSelector uint64, cfg ChainTopologyConfig, mcms *cldf.MCMSTimelockProposalInput) error {
	if len(cfg.MCMs) == 0 && len(cfg.Timelocks) == 0 {
		return fmt.Errorf("chain %d: no MCMs or timelocks to deploy", chainSelector)
	}

	mcmRefs := make(map[string]struct{}, len(cfg.MCMs))
	for i, m := range cfg.MCMs {
		if m.Ref == "" {
			return fmt.Errorf("chain %d: mcms[%d]: ref is required", chainSelector, i)
		}
		if _, dup := mcmRefs[m.Ref]; dup {
			return fmt.Errorf("chain %d: duplicate MCM ref %q", chainSelector, m.Ref)
		}
		mcmRefs[m.Ref] = struct{}{}
	}

	timelockRefs := make(map[string]struct{}, len(cfg.Timelocks))
	for i, t := range cfg.Timelocks {
		if t.Ref == "" {
			return fmt.Errorf("chain %d: timelocks[%d]: ref is required", chainSelector, i)
		}
		if _, dup := timelockRefs[t.Ref]; dup {
			return fmt.Errorf("chain %d: duplicate timelock ref %q", chainSelector, t.Ref)
		}
		if _, collides := mcmRefs[t.Ref]; collides {
			return fmt.Errorf("chain %d: timelock ref %q conflicts with an MCM ref", chainSelector, t.Ref)
		}
		timelockRefs[t.Ref] = struct{}{}

		if t.Qualifier == "" {
			return fmt.Errorf("chain %d: timelock %q: qualifier is required", chainSelector, t.Ref)
		}
		roleSets := map[string][]RoleHolder{
			"proposers":  t.Roles.Proposers,
			"cancellers": t.Roles.Cancellers,
			"bypassers":  t.Roles.Bypassers,
			"executors":  t.Roles.Executors,
			"admins":     t.Roles.Admins,
		}
		for name, holders := range roleSets {
			if err := validateRoleHolders(chainSelector, t.Ref, name, holders, mcmRefs); err != nil {
				return err
			}
		}
		if err := validateRoleHolders(chainSelector, t.Ref, "transferOwnership", t.TransferOwnership, mcmRefs); err != nil {
			return err
		}
		if len(t.TransferOwnership) > 0 && mcms == nil {
			return fmt.Errorf("chain %d: timelock %q: TransferOwnership requires an MCMS input to build the accept-ownership proposal", chainSelector, t.Ref)
		}
	}

	return nil
}

func validateRoleHolders(chainSelector uint64, timelockRef, field string, holders []RoleHolder, declared map[string]struct{}) error {
	for i, h := range holders {
		hasRef := h.MCMRef != ""
		hasAddr := h.Address != nil
		if hasRef == hasAddr {
			return fmt.Errorf("chain %d: timelock %q: %s[%d]: exactly one of mcmRef or address is required", chainSelector, timelockRef, field, i)
		}
		if hasRef {
			if _, ok := declared[h.MCMRef]; !ok {
				return fmt.Errorf("chain %d: timelock %q: %s[%d]: mcmRef %q is not declared in this chain's MCMs", chainSelector, timelockRef, field, i, h.MCMRef)
			}
		}
	}

	return nil
}

// mergeChainOutputs concatenates metadata and proposal groups from one chain's
// sequence output into the running aggregate.
func mergeChainOutputs(agg, out ChainOutput) ChainOutput {
	agg.Metadata.Addresses = append(agg.Metadata.Addresses, out.Metadata.Addresses...)
	agg.Metadata.Contracts = append(agg.Metadata.Contracts, out.Metadata.Contracts...)
	agg.Metadata.Chains = append(agg.Metadata.Chains, out.Metadata.Chains...)
	if out.Metadata.Env != nil {
		agg.Metadata.Env = out.Metadata.Env
	}
	agg.ProposalGroups = append(agg.ProposalGroups, out.ProposalGroups...)

	return agg
}

// sortedQualifiers returns the distinct, sorted qualifiers across all proposal
// groups (groups sharing a qualifier are merged into one proposal spanning chains).
func sortedQualifiers(groups []ProposalGroup) []string {
	seen := make(map[string]struct{}, len(groups))
	qualifiers := make([]string, 0, len(groups))
	for _, g := range groups {
		if len(g.BatchOps) == 0 {
			continue
		}
		if _, ok := seen[g.Qualifier]; ok {
			continue
		}
		seen[g.Qualifier] = struct{}{}
		qualifiers = append(qualifiers, g.Qualifier)
	}
	slices.Sort(qualifiers)

	return qualifiers
}

func batchOpsForQualifier(groups []ProposalGroup, qualifier string) []mcmstypes.BatchOperation {
	var ops []mcmstypes.BatchOperation
	for _, g := range groups {
		if g.Qualifier == qualifier {
			ops = append(ops, g.BatchOps...)
		}
	}

	return ops
}
