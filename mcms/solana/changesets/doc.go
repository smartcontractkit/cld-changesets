// Package changesets holds Solana-only MCMS changesets.
//
// Multi-family changesets (set-config, deploy, firedrill, and others) live under
// mcms/changesets and register per-family implementations from mcms/solana/<name>
// and mcms/evm/<name>. Solana-only changesets that have no chain-agnostic layer
// — because the concept is inherently Solana-specific — live here instead.
//
// # Available changesets
//
//   - fund-mcm-pdas — fund MCMS signer PDAs with lamports
//     (github.com/smartcontractkit/cld-changesets/mcms/solana/changesets/fund-mcm-pdas)
//
// Import path pattern:
//
//	github.com/smartcontractkit/cld-changesets/mcms/solana/changesets/<name>
package changesets
