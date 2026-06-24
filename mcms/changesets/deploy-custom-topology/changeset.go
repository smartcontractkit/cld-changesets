package deploycustomtopology

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	chain_selectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
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

	var agg sequenceutils.OnChainOutput

	for _, chainSelector := range maputil.SortedMapKeys(input.Cfg.ChainConfigs) {
		seq, seqErr := Sequences.SequenceForChainSelector(chainSelector)
		if seqErr != nil {
			return buildOutput(e, input.Cfg, input.MCMS, agg, fmt.Errorf("chain selector %d: %w", chainSelector, seqErr))
		}

		var mergeErr error
		agg, mergeErr = sequenceutils.ExecuteOnChainSequenceAndMerge(
			e.OperationsBundle,
			deps,
			seq,
			ChainInput{
				ChainSelector: chainSelector,
				Config:        input.Cfg.ChainConfigs[chainSelector],
				ExtraArgs:     input.Cfg.ChainConfigs[chainSelector].ExtraArgs,
				MCMS:          input.MCMS,
			},
			agg,
		)
		if mergeErr != nil {
			return buildOutput(e, input.Cfg, input.MCMS, agg, fmt.Errorf("chain selector %d: %w", chainSelector, mergeErr))
		}
	}

	if len(agg.BatchOps) > 0 && input.MCMS == nil {
		return buildOutput(e, input.Cfg, input.MCMS, agg,
			errors.New("sequences returned accept-ownership batch operations but no MCMS input was provided"))
	}

	return buildOutput(e, input.Cfg, input.MCMS, agg, nil)
}

// buildOutput writes deployed-address metadata to a fresh datastore and, when an
// MCMS input is present, builds one timelock proposal per timelock qualifier from
// the aggregated accept-ownership batch operations.
func buildOutput(
	e cldf.Environment,
	cfg Config,
	mcmsInput *cldf.MCMSTimelockProposalInput,
	agg sequenceutils.OnChainOutput,
	runErr error,
) (cldf.ChangesetOutput, error) {
	outDS := cldfdatastore.NewMemoryDataStore()
	if metaErr := outDS.WriteMetadata(agg.Metadata); metaErr != nil {
		return cldf.ChangesetOutput{DataStore: outDS},
			fmt.Errorf("failed to write metadata to datastore: %w", metaErr)
	}

	if runErr != nil {
		return cldf.ChangesetOutput{DataStore: outDS}, runErr
	}

	// Resolve proposals against an environment view that includes the freshly
	// deployed timelock/MCM addresses (the original env datastore does not yet
	// contain them). The output datastore keeps only the delta.
	resolveEnv := e
	if mcmsInput != nil {
		resolveDS := cldfdatastore.NewMemoryDataStore()
		if e.DataStore != nil {
			if mergeErr := resolveDS.Merge(e.DataStore); mergeErr != nil {
				return cldf.ChangesetOutput{DataStore: outDS},
					fmt.Errorf("failed to merge environment datastore: %w", mergeErr)
			}
		}
		if metaErr := resolveDS.WriteMetadata(agg.Metadata); metaErr != nil {
			return cldf.ChangesetOutput{DataStore: outDS},
				fmt.Errorf("failed to stage deployed addresses for proposal resolution: %w", metaErr)
		}
		resolveEnv.DataStore = resolveDS.Seal()
	}

	builder := cldf.NewOutputBuilder(resolveEnv, outDS)
	if mcmsInput != nil {
		for _, qualifier := range sortedQualifiersWithTransfers(cfg) {
			ops := batchOpsForQualifier(agg.BatchOps, agg.Metadata, cfg, qualifier)
			if len(ops) == 0 {
				continue
			}
			input := *mcmsInput
			input.Qualifier = qualifier
			builder = builder.WithTimelockProposal(input, ops)
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
		if err := Sequences.VerifyForFamily(family, e, byFamily[family]); err != nil {
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

// sortedQualifiersWithTransfers returns the distinct, sorted timelock qualifiers
// that have ownership transfers configured across all chains.
func sortedQualifiersWithTransfers(cfg Config) []string {
	seen := make(map[string]struct{})
	for _, chainCfg := range cfg.ChainConfigs {
		for _, tl := range chainCfg.Timelocks {
			if len(tl.TransferOwnership) == 0 {
				continue
			}
			seen[tl.Qualifier] = struct{}{}
		}
	}

	qualifiers := make([]string, 0, len(seen))
	for q := range seen {
		qualifiers = append(qualifiers, q)
	}
	slices.Sort(qualifiers)

	return qualifiers
}

// batchOpsForQualifier selects accept-ownership batch operations for a timelock
// qualifier by matching transaction targets against the configured transfer list.
func batchOpsForQualifier(
	batchOps []mcmstypes.BatchOperation,
	meta cldfdatastore.MetadataBundle,
	cfg Config,
	qualifier string,
) []mcmstypes.BatchOperation {
	var ops []mcmstypes.BatchOperation
	for _, op := range batchOps {
		chainSelector := uint64(op.ChainSelector)
		chainCfg, ok := cfg.ChainConfigs[chainSelector]
		if !ok {
			continue
		}
		targets := transferTargetAddressesForQualifier(chainSelector, chainCfg, meta, qualifier)
		if len(targets) == 0 {
			continue
		}
		if batchOpMatchesTargets(op, targets) {
			ops = append(ops, op)
		}
	}

	return ops
}

func transferTargetAddressesForQualifier(
	chainSelector uint64,
	chainCfg ChainTopologyConfig,
	meta cldfdatastore.MetadataBundle,
	qualifier string,
) map[string]struct{} {
	targets := make(map[string]struct{})
	for _, tl := range chainCfg.Timelocks {
		if tl.Qualifier != qualifier || len(tl.TransferOwnership) == 0 {
			continue
		}
		for _, h := range tl.TransferOwnership {
			if addr, ok := resolveRoleHolderAddress(chainSelector, h, chainCfg.MCMs, meta); ok {
				targets[addr] = struct{}{}
			}
		}
	}

	return targets
}

func resolveRoleHolderAddress(
	chainSelector uint64,
	h RoleHolder,
	mcms []MCMSpec,
	meta cldfdatastore.MetadataBundle,
) (string, bool) {
	if h.Address != nil {
		return strings.ToLower(h.Address.Hex()), true
	}

	var spec *MCMSpec
	for i := range mcms {
		if mcms[i].Ref == h.MCMRef {
			spec = &mcms[i]
			break
		}
	}
	if spec == nil {
		return "", false
	}

	for _, ref := range meta.Addresses {
		if ref.ChainSelector != chainSelector {
			continue
		}
		if ref.Qualifier != spec.Qualifier || ref.Type != cldfdatastore.ContractType(spec.ContractType) {
			continue
		}

		return strings.ToLower(ref.Address), true
	}

	return "", false
}

func batchOpMatchesTargets(op mcmstypes.BatchOperation, targets map[string]struct{}) bool {
	for _, tx := range op.Transactions {
		if _, ok := targets[strings.ToLower(tx.To)]; ok {
			return true
		}
	}

	return false
}
