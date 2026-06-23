package deploy

import (
	"errors"
	"fmt"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/internal/maputil"
)

var _ cldf.ChangeSetV2[Input] = Changeset{}

// Input configures MCMS with timelock deployment per chain selector.
type Input struct {
	ConfigByChain map[uint64]cldfproposalutils.MCMSWithTimelockConfig
}

// Changeset deploys MCMS with timelock across all configured chains.
//
// Each chain is dispatched sequentially to its family's registered sequence.
// New addresses are written to the datastore.
type Changeset struct{}

func (Changeset) VerifyPreconditions(env cldf.Environment, input Input) error {
	if len(input.ConfigByChain) == 0 {
		return errors.New("no chain configs provided")
	}

	byFamily, err := groupByFamily(input.ConfigByChain)
	if err != nil {
		return err
	}

	for family, chains := range byFamily {
		reg, err := get(family)
		if err != nil {
			return err
		}
		if reg.Verify != nil {
			if err := reg.Verify(env, chains); err != nil {
				return fmt.Errorf("family %s: %w", family, err)
			}
		}
	}

	return nil
}

func (Changeset) Apply(env cldf.Environment, input Input) (cldf.ChangesetOutput, error) {
	deps := Deps{
		BlockChains: env.BlockChains,
		DataStore:   env.DataStore,
	}

	var (
		results []sequenceutils.OnChainOutput
		reports []operations.Report[any, any]
	)

	for _, selector := range maputil.SortedMapKeys(input.ConfigByChain) {
		cfg := input.ConfigByChain[selector]

		family, err := chainselectors.GetSelectorFamily(selector)
		if err != nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("chain selector %d: %w", selector, err)
		}

		reg, err := get(family)
		if err != nil {
			return cldf.ChangesetOutput{}, err
		}

		report, err := operations.ExecuteSequence(
			env.OperationsBundle,
			reg.Sequence,
			deps,
			ChainInput{ChainSelector: selector, Config: cfg},
		)
		if err != nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("chain %d: %w", selector, err)
		}

		results = append(results, report.Output)
		reports = append(reports, report.ExecutionReports...)
	}

	agg := mergeOutputs(results)

	ds := cldfdatastore.NewMemoryDataStore()
	if env.DataStore != nil {
		if err := ds.Merge(env.DataStore); err != nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("merge environment datastore: %w", err)
		}
	}
	if err := ds.WriteMetadata(agg.Metadata); err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("write deployment metadata to datastore: %w", err)
	}

	return cldf.NewOutputBuilder(env, ds).WithOperationsReports(reports).Build()
}

// mergeOutputs combines all per-chain OnChainOutput results into a single
// aggregate by appending their metadata slices and batch operations.
func mergeOutputs(outputs []sequenceutils.OnChainOutput) sequenceutils.OnChainOutput {
	var agg sequenceutils.OnChainOutput
	for _, out := range outputs {
		agg.BatchOps = append(agg.BatchOps, out.BatchOps...)
		agg.Metadata.Addresses = append(agg.Metadata.Addresses, out.Metadata.Addresses...)
		agg.Metadata.Contracts = append(agg.Metadata.Contracts, out.Metadata.Contracts...)
		agg.Metadata.Chains = append(agg.Metadata.Chains, out.Metadata.Chains...)
		if agg.Metadata.Env == nil && out.Metadata.Env != nil {
			agg.Metadata.Env = out.Metadata.Env
		}
	}

	return agg
}
