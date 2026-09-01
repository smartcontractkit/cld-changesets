package stellartransfertotimelock

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
)

var seqTransferToTimelock = operations.NewSequence(
	"stellar-transfer-to-timelock",
	semver.MustParse("1.0.0"),
	"Transfer Stellar contract ownership to the MCMS timelock",
	func(b operations.Bundle, deps transfertotimelock.Deps, in transfertotimelock.ChainInput) (sequenceutils.OnChainOutput, error) {
		chain, ok := deps.BlockChains.StellarChains()[in.ChainSelector]
		if !ok {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("stellar chain %d not found in environment", in.ChainSelector)
		}
		if in.MCMS == nil {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("MCMS config is required for Stellar chain %d", in.ChainSelector)
		}

		env := transfertotimelock.EnvFromDeps(deps)
		reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyStellar)
		if !ok {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("no MCMS reader registered for chain family %q", chainselectors.FamilyStellar)
		}

		timelockRef, err := reader.GetTimelockRef(env, in.ChainSelector, *in.MCMS)
		if err != nil {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("resolve Stellar timelock for chain %d: %w", in.ChainSelector, err)
		}

		batchOps := make([]mcmstypes.BatchOperation, 0, len(in.Contracts))
		for i, contractRef := range in.Contracts {
			ref, err := contractRef.Resolve(env)
			if err != nil {
				return sequenceutils.OnChainOutput{}, fmt.Errorf("contracts[%d]: %w", i, err)
			}

			report, err := operations.ExecuteOperation(
				b,
				OpTransferToTimelock,
				chain,
				OpTransferToTimelockInput{
					Contract: OwnableContract{
						Address: ref.Address,
						Type:    cldf.ContractType(ref.Type),
					},
					Timelock:            timelockRef.Address,
					OnlyAcceptOwnership: in.OnlyAcceptOwnership,
				},
			)
			if err != nil {
				return sequenceutils.OnChainOutput{}, fmt.Errorf("transfer %s to timelock: %w", ref.Address, err)
			}

			batchOps = append(batchOps, report.Output.BatchOps...)
		}

		return sequenceutils.OnChainOutput{BatchOps: batchOps}, nil
	},
)
