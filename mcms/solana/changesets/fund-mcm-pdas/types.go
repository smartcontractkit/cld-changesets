package fundmcmpdas

import (
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

// FundingConfig holds the lamport amounts to send to each MCMS signer PDA on a chain.
type FundingConfig struct {
	ProposeMCM   uint64 `json:"proposeMcm"`
	CancellerMCM uint64 `json:"cancellerMcm"`
	BypasserMCM  uint64 `json:"bypasserMcm"`
	Timelock     uint64 `json:"timelock"`
	Qualifier    string `json:"qualifier,omitempty"`
}

// RequiredFunding returns the total lamports required to fund all MCMS PDAs on a chain.
func (c FundingConfig) RequiredFunding() uint64 {
	return c.ProposeMCM + c.CancellerMCM + c.BypasserMCM + c.Timelock
}

// Config holds the funding amounts per chain for the changeset.
type Config struct {
	FundingPerChain map[uint64]FundingConfig `json:"fundingPerChain"`
}

// Deps is the read-only dependency bundle available to the fund sequence.
type Deps struct {
	BlockChains chain.BlockChains
	DataStore   cldfdatastore.DataStore
}

// ChainInput is the per-chain request for the fund-mcm-pdas sequence.
type ChainInput struct {
	ChainSelector uint64
	FundingConfig FundingConfig
}

// EnvFromDeps reconstructs the environment fields sequences need for ref resolution.
func EnvFromDeps(deps Deps) cldf.Environment {
	return cldf.Environment{
		BlockChains: deps.BlockChains,
		DataStore:   deps.DataStore,
	}
}
