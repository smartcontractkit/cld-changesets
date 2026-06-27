package grantrole

import (
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
)

// RoleGrant grants one timelock role to multiple accounts.
type RoleGrant struct {
	Role      mcmssdk.TimelockRole `json:"role"`
	Addresses []string             `json:"addresses"`
}

// Config selects timelock role grants by chain selector.
type Config struct {
	GrantsByChain map[uint64][]RoleGrant `json:"grantsByChain"`
	// GasBoostConfig optionally configures EVM retry gas boosting for direct sends.
	GasBoostConfig *proposalutils.GasBoostConfig `json:"gasBoostConfig,omitempty"`
}

// Input is the grant-role changeset configuration with optional MCMS proposal settings.
type Input = sequenceutils.WithMCMS[Config]

// SeqInput is the evm input for calling grant role on the timelock contract
type SeqInput struct {
	ChainSelector  uint64                          `json:"chainSelector"`
	Grants         []RoleGrant                     `json:"grants"`
	MCMS           *cldf.MCMSTimelockProposalInput `json:"mcms,omitempty"`
	GasBoostConfig *proposalutils.GasBoostConfig   `json:"gasBoostConfig,omitempty"`
}

// Deps is the read-only dependency bundle available to every family sequence.
type Deps struct {
	BlockChains chain.BlockChains
	DataStore   cldfdatastore.DataStore
}

// Sequence is the required operations sequence type for all family implementations.
type Sequence = operations.Sequence[SeqInput, sequenceutils.OnChainOutput, Deps]

// EnvFromDeps reconstructs the environment fields sequences need for datastore resolution.
func EnvFromDeps(deps Deps) cldf.Environment {
	return cldf.Environment{
		BlockChains: deps.BlockChains,
		DataStore:   deps.DataStore,
	}
}
