package soldeploy

import (
	"fmt"

	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	solops "github.com/smartcontractkit/cld-changesets/pkg/family/solana/operations"
	solseqs "github.com/smartcontractkit/cld-changesets/pkg/family/solana/sequences"
)

var seqDeployMCMSWithTimelock = operations.NewSequence(
	"seq-solana-mcms-deploy-with-timelock",
	&semvers.V1_0_0,
	"Deploy MCMS and timelock programs on a Solana chain",
	deployMCMSWithTimelock,
)

func deployMCMSWithTimelock(
	b operations.Bundle,
	deps deploy.Deps,
	in deploy.ChainInput,
) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.SolanaChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("solana chain %d not found in environment", in.ChainSelector)
	}

	chainState, err := loadChainState(deps.DataStore, in.ChainSelector)
	if err != nil {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("load solana mcms state: %w", err)
	}

	memDS := cldfdatastore.NewMemoryDataStore()
	_, err = operations.ExecuteSequence(
		b,
		solseqs.DeployMCMSWithTimelockSeq,
		solops.Deps{
			State:     chainState,
			Chain:     chain,
			Datastore: memDS,
		},
		solseqs.DeployMCMSWithTimelockInput{
			MCMConfig:        in.Config,
			TimelockMinDelay: in.Config.TimelockMinDelay,
		},
	)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}

	newRefs, err := collectOutputRefs(memDS, chainState, deps.DataStore, in.ChainSelector)
	if err != nil {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("collect deployed address refs: %w", err)
	}

	qualifier := qualifierFromConfig(in.Config.Qualifier)
	label := labelFromConfig(in.Config.Label)

	return sequenceutils.OnChainOutput{
		Metadata: cldfdatastore.MetadataBundle{
			Addresses: decorateAddressRefs(newRefs, qualifier, label),
		},
	}, nil
}
