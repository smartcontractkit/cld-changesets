package soltransfertotimelock

import (
	"errors"
	"fmt"

	solanago "github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

var seqTransferToTimelock = operations.NewSequence(
	"seq-solana-transfer-to-timelock",
	&semvers.V1_0_0,
	"Transfers ownable Solana contract ownership to the MCMS timelock",
	runSolanaTransferToTimelock,
)

func runSolanaTransferToTimelock(
	b operations.Bundle,
	deps transfertotimelock.Deps,
	in transfertotimelock.ChainInput,
) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.SolanaChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("solana chain %d not found in environment", in.ChainSelector)
	}

	env := transfertotimelock.EnvFromDeps(deps)
	if in.MCMS == nil {
		return sequenceutils.OnChainOutput{}, errors.New("MCMS timelock proposal input is required")
	}
	if len(in.Contracts) == 0 {
		return sequenceutils.OnChainOutput{}, errors.New("no contracts provided for solana chain")
	}

	timelockSignerPDA, err := timelockSignerPDA(env, in)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}

	var batchOps []mcmstypes.BatchOperation
	for i, ref := range in.Contracts {
		contract, err := resolveOwnableContract(env, in.ChainSelector, ref)
		if err != nil {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("contracts[%d]: %w", i, err)
		}

		ops, err := transferContractToTimelock(b, chain, timelockSignerPDA, contract, in)
		if err != nil {
			return sequenceutils.OnChainOutput{}, fmt.Errorf("contract %s: %w", contract.Type, err)
		}
		batchOps = append(batchOps, ops...)
	}

	if len(batchOps) == 0 {
		return sequenceutils.OnChainOutput{}, nil
	}

	return sequenceutils.OnChainOutput{BatchOps: batchOps}, nil
}

func timelockSignerPDA(env cldf.Environment, in transfertotimelock.ChainInput) (solanago.PublicKey, error) {
	if in.MCMS == nil {
		return solanago.PublicKey{}, errors.New("MCMS timelock proposal input is required")
	}

	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilySolana)
	if !ok {
		return solanago.PublicKey{}, fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilySolana)
	}

	timelockRef, err := reader.GetTimelockRef(env, in.ChainSelector, *in.MCMS)
	if err != nil {
		return solanago.PublicKey{}, fmt.Errorf("resolve timelock for chain %d: %w", in.ChainSelector, err)
	}

	timelockProgram, timelockSeed, err := mcmssolanasdk.ParseContractAddress(timelockRef.Address)
	if err != nil {
		return solanago.PublicKey{}, fmt.Errorf("parse timelock ref address for chain %d: %w", in.ChainSelector, err)
	}

	var seed legacysolana.PDASeed
	copy(seed[:], timelockSeed[:])

	return familysolana.GetTimelockSignerPDA(timelockProgram, seed), nil
}

func requireSolanaChain(env cldf.Environment, chainSelector uint64) (cldfsol.Chain, error) {
	chain, ok := env.BlockChains.SolanaChains()[chainSelector]
	if !ok {
		return cldfsol.Chain{}, fmt.Errorf("solana chain %d not found in environment", chainSelector)
	}

	return chain, nil
}
