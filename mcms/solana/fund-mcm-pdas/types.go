package solfundmcmpdas

// FundingConfig holds the lamport amounts to send to each MCMS signer PDA on a chain.
type FundingConfig struct {
	ProposeMCM   uint64 `json:"proposeMcm"`
	CancellerMCM uint64 `json:"cancellerMcm"`
	BypasserMCM  uint64 `json:"bypasserMcm"`
	Timelock     uint64 `json:"timelock"`
	Qualifier    string `json:"qualifier,omitempty"`
}

// Config holds the funding amounts per chain for the changeset.
type Config struct {
	FundingPerChain map[uint64]FundingConfig `json:"fundingPerChain"`
}

// RequiredFunding returns the total lamports required to fund all MCMS PDAs on a chain.
func (c FundingConfig) RequiredFunding() uint64 {
	return c.ProposeMCM + c.CancellerMCM + c.BypasserMCM + c.Timelock
}
