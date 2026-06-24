package fundmcmpdas

import solfundmcmpdas "github.com/smartcontractkit/cld-changesets/mcms/solana/fund-mcm-pdas"

// FundingConfig holds the lamport amounts to send to each MCMS signer PDA on a chain.
type FundingConfig = solfundmcmpdas.FundingConfig

// Config holds the funding amounts per chain for the changeset.
type Config struct {
	FundingPerChain map[uint64]FundingConfig `json:"fundingPerChain"`
}
