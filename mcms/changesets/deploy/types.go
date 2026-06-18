package deploy

import (
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// ChainInput is the per-chain input passed to every family sequence.
type ChainInput struct {
	ChainSelector uint64
	Config        cldfproposalutils.MCMSWithTimelockConfig
}

// Deps is the read-only dependency bundle available to every family sequence.
// Logger and context come from the operations.Bundle passed at execution time.
type Deps struct {
	BlockChains chain.BlockChains
	DataStore   cldfdatastore.DataStore
}

// Sequence is the required operations sequence type for all family implementations.
type Sequence = operations.Sequence[ChainInput, sequenceutils.OnChainOutput, Deps]

// Registration describes one chain family's MCMS deploy implementation.
type Registration struct {
	// Family is the chain-selectors family string (e.g. chainselectors.FamilyEVM).
	Family string
	// Sequence executes the per-chain deploy and returns newly deployed addresses
	// via OnChainOutput.Metadata.Addresses.
	Sequence *Sequence
	// Verify performs family-specific validation across all chains in the input.
	// It is called during VerifyPreconditions. Optional — nil means no extra checks.
	Verify func(env cldf.Environment, chains []ChainInput) error
}
