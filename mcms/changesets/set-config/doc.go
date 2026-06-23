// Package setconfig provides the SetConfigMCMS changeset and a registry for
// per-chain-family set-config sequences.
//
// # Usage
//
// Import the changeset and blank-import each chain family's set-config sequence
// (plus MCMS readers when building timelock proposals):
//
//	import (
//		setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/evm/set-config"
//	)
//
//	// Testing:
//	rt.Exec(runtime.ChangesetTask(setconfig.Changeset{}, setconfig.Input{
//		Cfg: setconfig.Config{
//			Targets: []setconfig.ContractSetConfig{
//				{Ref: proposerRef, Config: proposerCfg},
//			},
//		},
//		MCMS: mcmsInput, // nil for direct on-chain execution
//	}))
//
//	// CLD:
//	registry.Add("set_config_mcms", Configure(setconfig.Changeset{}).WithEnvInput())
//
// For pipelines that need every built-in family, blank-import [all] instead:
//
//	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config/all"
//
// # Sequences
//
// [Changeset] groups targets by chain, runs each chain's set-config sequence,
// merges batch operations with [sequenceutils.ExecuteOnChainSequenceAndMerge],
// writes merged metadata to the datastore, and optionally builds an MCMS timelock
// proposal when [Input.MCMS] is set.
//
// When MCMS input is nil, sequences send transactions directly. When MCMS input
// is present, sequences run in no-send mode and return batch operations for
// [cldf.OutputBuilder.WithTimelockProposal].
//
// # Built-in EVM and Solana support
//
// EVM and Solana register themselves via init when their packages are imported.
// Importing set-config alone does not register any family — you must blank-import
// mcms/<family>/set-config (and mcms/<family>/readers for MCMS proposals).
//
// # Adding a new chain family
//
// Implement a family under mcms/<family>/set-config (for example
// mcms/aptos/set-config) and register it at process startup. Follow this layout:
//
//   - register.go — exports Registration() and calls [Register] in init()
//   - sequence.go — operations.Sequence that sets config for one chain
//   - operation.go  — per-contract set-config operation
//   - validate.go   — optional MCMS ref validation during VerifyPreconditions
//
// If the family supports MCMS proposals, also implement mcms/<family>/readers
// and register the reader with [cldf.GetMCMSReaderRegistry] from that package's
// init (see mcms/evm/readers).
//
// Step 1: implement Registration
//
// Return a [Registration] with the chain-selectors family string, a set-config
// sequence, and an optional Verify function:
//
//	func Registration() setconfig.Registration {
//		return setconfig.Registration{
//			Family:   chainselectors.FamilyAptos,
//			Sequence: seqSetConfig,
//			Verify:   verifyAptosChains,
//		}
//	}
//
// Step 2: implement the sequence
//
// The sequence must match [Sequence]: it receives [ChainInput] and [Deps], and
// returns [sequenceutils.OnChainOutput]. Populate BatchOps when running in
// no-send mode (MCMS input present). Write any new address or contract metadata
// to output.Metadata.
//
// Step 3: register at startup
//
// Auto-register from init (recommended, mirrors EVM and Solana):
//
//	func init() { setconfig.Registry.Register(Registration()) }
//
// Or register explicitly from the caller's main/setup code:
//
//	setconfig.Registry.Register(aptossetconfig.Registration())
//
// Each family may only be registered once; duplicate registration panics.
//
// # Registration contract
//
//   - Family — must match chainselectors.GetSelectorFamily for chains you support
//   - Sequence — required; one invocation per chain in the input targets
//   - Verify — optional; called during VerifyPreconditions with all chains of
//     that family in the input (use to assert chains exist in the environment
//     and MCMS refs resolve when MCMS input is present)
//
// If a chain's family has no registered sequence, Apply returns an error
// listing families that are registered. Use [Registry.RegisteredFamilies] to inspect the
// registry in tests.
//
// # Reference implementations
//
// See mcms/evm/set-config and mcms/solana/set-config for complete implementations:
// datastore ref resolution, direct-send vs proposal-only modes, and MCMS batch
// operation assembly.
package setconfig
