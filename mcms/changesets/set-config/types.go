package setconfig

import (
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
)

// ContractSetConfig binds an MCMS config to a datastore contract reference.
type ContractSetConfig struct {
	Ref    refkey.RefKey    `json:"ref"`
	Config mcmstypes.Config `json:"config"`
}

// ChainInput is the family-agnostic, per-chain request for a set-config sequence.
type ChainInput struct {
	ChainSelector uint64                          `json:"chainSelector"`
	Targets       []ContractSetConfig             `json:"targets"`
	MCMS          *cldf.MCMSTimelockProposalInput `json:"mcms,omitempty"`
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
