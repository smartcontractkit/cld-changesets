package transfertotimelock

import (
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
)

// Config selects ownable contracts to transfer to the MCMS timelock per chain.
type Config struct {
	// ContractsByChain lists contracts as datastore refs for every supported chain family.
	ContractsByChain map[uint64][]refkey.RefKey `json:"contractsByChain,omitempty"`
	// OnlyAcceptOwnership skips the on-chain transferOwnership step and only
	// builds accept-ownership operations for the MCMS proposal.
	OnlyAcceptOwnership bool `json:"onlyAcceptOwnership,omitempty"`
}

// Input is the changeset configuration with MCMS timelock proposal settings.
type Input = sequenceutils.WithMCMS[Config]

// ChainInput is the per-chain request passed to a family sequence.
type ChainInput struct {
	ChainSelector       uint64                          `json:"chainSelector"`
	Contracts           []refkey.RefKey                 `json:"contracts,omitempty"`
	OnlyAcceptOwnership bool                            `json:"onlyAcceptOwnership,omitempty"`
	MCMS                *cldf.MCMSTimelockProposalInput `json:"mcms,omitempty"`
}

// Deps is the read-only dependency bundle available to every family sequence.
type Deps struct {
	BlockChains chain.BlockChains
	DataStore   cldfdatastore.DataStore
}

// Sequence is the required operations sequence type for all family implementations.
type Sequence = operations.Sequence[ChainInput, sequenceutils.OnChainOutput, Deps]

// EnvFromDeps reconstructs the environment fields sequences need for datastore resolution.
func EnvFromDeps(deps Deps) cldf.Environment {
	return cldf.Environment{
		BlockChains: deps.BlockChains,
		DataStore:   deps.DataStore,
	}
}
