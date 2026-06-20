// Package deploycustomtopology provides the DeployCustomTopology changeset and a
// registry for per-chain-family deploy implementations. It deploys an arbitrary
// MCMS topology per chain — any number of MCMs and timelocks, custom role wiring,
// and optional ownership transfers — unlike the fixed standard-topology deploy.
//
// # Usage
//
// Import the changeset and blank-import each chain family you use (plus MCMS
// readers, required when ownership transfers build timelock proposals). This
// follows the Go plugin pattern used by database/sql drivers:
//
//	import (
//		topology "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy-custom-topology"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy-custom-topology"
//	)
//
// For pipelines that need every built-in family, blank-import [all] instead:
//
//	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy-custom-topology/all"
//
// # Behaviour
//
// [Changeset] iterates the configured chains, runs each chain's registered deploy
// sequence, writes deployed-address metadata to the changeset datastore, and — when
// [Input.MCMS] is set and a timelock takes ownership of contracts — builds one MCMS
// timelock proposal per timelock qualifier from the returned batch operations.
//
// Each family sequence returns a [ChainOutput] carrying deployed-address Metadata
// and accept-ownership [ProposalGroup]s keyed by the owning timelock's qualifier.
// The changeset emits one proposal per distinct qualifier, so a chain with several
// independent timelocks each yields its own proposal resolving its own
// (timelock, proposer MCM) pair.
//
// # Built-in EVM and Solana support
//
// EVM and Solana register themselves via init when their packages are imported.
// Importing this package alone does not register any family — you must blank-import
// mcms/<family>/deploy-custom-topology (and mcms/<family>/readers for MCMS proposals).
//
// # Adding a new chain family
//
// Implement a family under mcms/<family>/deploy-custom-topology and register it at
// process startup. Follow this layout:
//
//   - register.go — exports Registration() and calls [Register] in init()
//   - sequence.go — operations.Sequence that deploys the topology for one chain
//   - addresses.go — datastore ref resolution / AddressRef construction
//   - extra_args.go — optional, parses family-specific [ChainTopologyConfig.ExtraArgs]
//
// The sequence must match [Sequence]: it receives [ChainInput] and [Deps] and returns
// [ChainOutput]. Populate Metadata with deployed addresses and ProposalGroups with
// accept-ownership batch operations (only when running with MCMS input).
//
// Each family may only be registered once; duplicate registration panics. Use
// [RegisteredFamilies] to inspect the registry in tests.
package deploycustomtopology
