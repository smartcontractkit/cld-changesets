// Package changesets provides multi-family MCMS changesets.
//
// Start here when looking for a changeset to use. Each subdirectory is a
// self-contained changeset with a chain-agnostic entrypoint ([Changeset], Config,
// and optionally a registry for per-family sequences).
//
// # Multi-family changesets (this directory)
//
//   - set-config — configure MCMS contracts across chains
//   - deploy — deploy the standard MCMS topology
//   - deploy-custom-topology — deploy an arbitrary MCMS topology
//   - transfer-to-timelock — transfer contract ownership to a timelock
//   - firedrill — MCMS firedrill operations
//
// Import path pattern:
//
//	github.com/smartcontractkit/cld-changesets/mcms/changesets/<name>
//
// Blank-import the chain families you need, for example:
//
//	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/set-config"
//	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/set-config"
//
// # Family-only changesets
//
// Some changesets are inherently specific to one chain family and have no
// chain-agnostic wrapper. Solana-only changesets live under
// mcms/solana/changesets (for example fund-mcm-pdas). EVM-only changesets
// would follow mcms/evm/changesets if added in the future.
//
// Family implementation packages (mcms/evm/<name>, mcms/solana/<name>) contain
// sequences and operations registered by multi-family changesets via init.
// Solana-only changesets live under mcms/solana/changesets.
package changesets
