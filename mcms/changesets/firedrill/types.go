package firedrill

import (
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// Config selects chains for the MCMS signing fire drill.
type Config struct {
	Selectors []uint64 `json:"selectors,omitempty"`
}

// Input is the changeset configuration with MCMS timelock proposal settings.
type Input = sequenceutils.WithMCMS[Config]

// ChainInput is the family-agnostic, per-chain request for a fire-drill sequence.
type ChainInput struct {
	ChainSelector uint64                         `json:"chainSelector"`
	MCMS          cldf.MCMSTimelockProposalInput `json:"mcms"`
}

// Deps is the read-only dependency bundle available to every family sequence.
type Deps struct {
	BlockChains chain.BlockChains
	DataStore   cldfdatastore.DataStore
}

// Sequence is the required operations sequence type for all family implementations.
type Sequence = operations.Sequence[ChainInput, sequenceutils.OnChainOutput, Deps]

// EnvFromDeps reconstructs the environment fields sequences need for ref and MCMS resolution.
func EnvFromDeps(deps Deps) cldf.Environment {
	return cldf.Environment{
		BlockChains: deps.BlockChains,
		DataStore:   deps.DataStore,
	}
}
