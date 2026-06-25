package deploycustomtopology

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

// Input wraps the topology config with optional MCMS timelock-proposal settings. When MCMS is
// set, ownership-transfer sequences run in no-send mode and the changeset builds
// accept-ownership timelock proposals from the returned batch operations.
type Input = sequenceutils.WithMCMS[MCMSTopologyConfig]

// MCMSTopologyConfig is the top-level, chain-agnostic changeset input.
type MCMSTopologyConfig struct {
	ChainConfigs map[uint64]ChainTopologyConfig `json:"chainConfigs"`
}

// ChainTopologyConfig describes the MCMS topology to deploy on a single chain.
type ChainTopologyConfig struct {
	MCMs           []MCMSpec                   `json:"mcms"`
	Timelocks      []TimelockSpec              `json:"timelocks"`
	GasBoostConfig *opscontract.GasBoostConfig `json:"gasBoostConfig,omitempty"`

	// ExtraArgs carries chain-family-specific input that the default per-chain
	// parameters above cannot express. It is forwarded verbatim to the family
	// sequence, which parses it if necessary. EVM uses it to carry call-proxy
	// settings (see mcms/evm/deploy-custom-topology); Solana ignores it today.
	// Future families may require it for family-specific topology input.
	ExtraArgs any `json:"extraArgs,omitempty"`
}

// MCMSpec is one ManyChainMultiSig contract to deploy.
type MCMSpec struct {
	// Ref is a logical handle, unique within the chain. RoleHolder.MCMRef and
	// other specs reference an MCM by this Ref.
	Ref string `json:"ref"`

	Config mcmstypes.Config `json:"config"`

	Qualifier string `json:"qualifier"`

	ContractType cldf.ContractType `json:"contractType"`

	Label *string `json:"label,omitempty"`
}

// TimelockSpec is one timelock (RBACTimelock on EVM) to deploy, with its role
// wiring and optional ownership transfers.
type TimelockSpec struct {
	// Ref is a logical handle, unique within the chain and distinct from every
	// MCMSpec.Ref on the same chain.
	Ref string `json:"ref"`

	MinDelay  *big.Int `json:"minDelay"`
	Qualifier string   `json:"qualifier"`
	Label     *string  `json:"label,omitempty"`

	Roles RoleAssignments `json:"roles"`

	// TransferOwnership (optional): ownable contracts whose ownership should be
	// moved to THIS timelock. Duplicate targets are deduplicated by resolved address.
	// The deployer runs transferOwnership() now; acceptOwnership() batch ops are
	// emitted for MCMS and turned into accept-ownership timelock proposals.
	// Requires Input.MCMS to be set.
	TransferOwnership []RoleHolder `json:"transferOwnership,omitempty"`
}

// RoleAssignments wires timelock roles to MCMs or addresses.
type RoleAssignments struct {
	Proposers  []RoleHolder `json:"proposers"`
	Cancellers []RoleHolder `json:"cancellers"`
	Bypassers  []RoleHolder `json:"bypassers"`
	Executors  []RoleHolder `json:"executors"` // EVM only
	// Admins are optional extra admins; the deployer is always the initial admin
	// and the timelock is granted admin over itself (mirrors the legacy flow).
	Admins []RoleHolder `json:"admins"` // EVM only - just 1 supported in solana
}

// RoleHolder identifies a role grantee, either an MCM deployed in this run
// (by Ref) or a direct address.
type RoleHolder struct {
	MCMRef  string          `json:"mcmRef,omitempty"`
	Address *common.Address `json:"address,omitempty"`
}

// ChainInput is the family-agnostic, per-chain request passed to every family
// sequence by the changeset.
type ChainInput struct {
	ChainSelector uint64              `json:"chainSelector"`
	Config        ChainTopologyConfig `json:"config"`
	// ExtraArgs mirrors ChainTopologyConfig.ExtraArgs and is parsed by the family
	// sequence if needed.
	ExtraArgs any `json:"extraArgs,omitempty"`
	// MCMS is the proposal input used to build accept-ownership timelock proposals
	// when TransferOwnership is set. nil disables proposal building.
	MCMS *cldf.MCMSTimelockProposalInput `json:"mcms,omitempty"`
}

// Deps is the read-only dependency bundle available to every family sequence.
type Deps struct {
	BlockChains chain.BlockChains
	DataStore   cldfdatastore.DataStore
}

// Sequence is the required operations sequence type for all family implementations.
type Sequence = operations.Sequence[ChainInput, sequenceutils.OnChainOutput, Deps]

// EnvFromDeps reconstructs the environment fields sequences need for ref and
// MCMS resolution.
func EnvFromDeps(deps Deps) cldf.Environment {
	return cldf.Environment{
		BlockChains: deps.BlockChains,
		DataStore:   deps.DataStore,
	}
}
