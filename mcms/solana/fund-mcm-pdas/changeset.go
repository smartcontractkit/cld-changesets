package solfundmcmpdas

import (
	"errors"
	"fmt"

	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/internal/maputil"
)

var _ cldf.ChangeSetV2[Config] = Changeset{}

// Changeset funds MCMS signer PDAs on each configured Solana chain.
type Changeset struct{}

func (Changeset) VerifyPreconditions(env cldf.Environment, config Config) error {
	if len(config.FundingPerChain) == 0 {
		return errors.New("no funding config provided")
	}

	for chainSelector, chainCfg := range config.FundingPerChain {
		if _, ok := env.BlockChains.SolanaChains()[chainSelector]; !ok {
			return fmt.Errorf("solana chain %d not found in environment", chainSelector)
		}
		if err := validateMCMSRefs(env, chainSelector, chainCfg); err != nil {
			return err
		}
		if err := validateDeployerBalance(env, chainSelector, chainCfg); err != nil {
			return err
		}
	}

	return nil
}

func (Changeset) Apply(e cldf.Environment, config Config) (cldf.ChangesetOutput, error) {
	deps := seqDeps{
		BlockChains: e.BlockChains,
		DataStore:   e.DataStore,
	}

	var agg sequenceutils.OnChainOutput

	for _, chainSelector := range maputil.SortedMapKeys(config.FundingPerChain) {
		chainCfg := config.FundingPerChain[chainSelector]

		var mergeErr error
		agg, mergeErr = sequenceutils.ExecuteOnChainSequenceAndMerge(
			e.OperationsBundle,
			deps,
			SeqFundMCMPDAs,
			seqInput{
				ChainSelector: chainSelector,
				FundingConfig: chainCfg,
			},
			agg,
		)
		if mergeErr != nil {
			return buildOutput(e, agg, mergeErr)
		}
	}

	return buildOutput(e, agg, nil)
}

func buildOutput(
	e cldf.Environment,
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

	out, buildErr := cldf.NewOutputBuilder(e, ds).Build()
	if buildErr != nil {
		return out, fmt.Errorf("build changeset output: %w", buildErr)
	}

	return out, nil
}
