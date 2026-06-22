package evmfiredrill

import (
	"fmt"
	"math/big"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmsevmsdk "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	firedrill "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill"
)

var seqFireDrill = operations.NewSequence(
	"seq-evm-firedrill",
	semver.MustParse("1.0.0"),
	"Build noop MCMS fire-drill batch operation for an EVM chain from datastore refs",
	runEVMFireDrill,
)

func runEVMFireDrill(
	_ operations.Bundle,
	deps firedrill.Deps,
	in firedrill.ChainInput,
) (sequenceutils.OnChainOutput, error) {
	if _, ok := deps.BlockChains.EVMChains()[in.ChainSelector]; !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("EVM chain %d not found in environment", in.ChainSelector)
	}

	env := firedrill.EnvFromDeps(deps)
	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyEVM)
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilyEVM)
	}

	timelockRef, err := reader.GetTimelockRef(env, in.ChainSelector, in.MCMS)
	if err != nil {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("resolve timelock ref for chain %d: %w", in.ChainSelector, err)
	}
	if !common.IsHexAddress(timelockRef.Address) {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("invalid timelock address %q for chain %d", timelockRef.Address, in.ChainSelector)
	}
	timelockAddr := common.HexToAddress(timelockRef.Address)
	if timelockAddr == (common.Address{}) {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("timelock address is zero for chain %d", in.ChainSelector)
	}

	tx := mcmsevmsdk.NewTransaction(
		timelockAddr,
		[]byte{},
		big.NewInt(0),
		"FireDrillNoop",
		nil,
	)

	return sequenceutils.OnChainOutput{
		BatchOps: []mcmstypes.BatchOperation{{
			ChainSelector: mcmstypes.ChainSelector(in.ChainSelector),
			Transactions:  []mcmstypes.Transaction{tx},
		}},
	}, nil
}
