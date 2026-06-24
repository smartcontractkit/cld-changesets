// Package deploy provides the DeployMCMSWithTimelock changeset and a registry
// for per-chain-family deploy implementations.
//
// # Usage
//
// Import the changeset and at least one registered chain family, then execute
// with a per-chain MCMS+timelock config:
//
//		import (
//			"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
//			_ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy" // registers EVM
//		)
//
//	 Testing:
//		rt.Exec(runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
//			ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
//				selector: cfg,
//			},
//		}))
//
//	 CLD:
//	 registry.Add("deploy_mcms_with_timelock", Configure(deploy.Changeset{}).WithEnvInput())
//
// [Changeset] groups input chains by chain-selectors family, runs each chain's
// deploy sequence sequentially, merges newly deployed addresses into the
// datastore, and returns operation reports.
//
// # Built-in EVM support
//
// EVM registers itself via init when its package is imported (blank import is
// enough). No call to [RegisterFamilies] is required for EVM-only deployments.
//
// # Adding a new chain family
//
// Implement a family under mcms/<family>/deploy (for example mcms/solana/deploy)
// and register it at process startup. Follow this layout:
//
//   - register.go — exports Registration() and calls deploy.Registry.Register in init()
//   - sequence.go — operations.Sequence that deploys contracts for one chain
//   - addresses.go  — helpers to load existing addresses from the datastore
//
// Step 1: implement Registration
//
// Return a [Registration] with the chain-selectors family string, a deploy
// sequence, and an optional Verify function:
//
//	func Registration() deploy.Registration {
//		return deploy.Registration{
//			Family:   chainselectors.FamilySolana, // or FamilyAptos, etc.
//			Sequence: seqDeployMCMSWithTimelock,
//			Verify:   verifySolanaChains,
//		}
//	}
//
// Step 2: implement the sequence
//
// The sequence must match [Sequence]: it receives [ChainInput] and [Deps], and
// returns [sequenceutils.OnChainOutput]. Write newly deployed contract addresses
// to output.Metadata.Addresses (see mcms/evm/deploy for AddressRef conventions).
// Skip contracts already present in deps.DataStore for the chain and qualifier.
//
// Step 3: register at startup
//
// Either auto-register from init (recommended, mirrors EVM):
//
//	func init() { deploy.Registry.Register(Registration()) }
//
// Or register explicitly from the caller's main/setup code:
//
//	deploy.RegisterFamilies(solanadeploy.Registration())
//
// Each family may only be registered once; duplicate registration panics.
//
// # Registration contract
//
//   - Family — must match chainselectors.GetSelectorFamily for chains you support
//   - Sequence — required; one invocation per chain in ConfigByChain
//   - Verify — optional; called during VerifyPreconditions with all chains of
//     that family in the input (use to assert chains exist in the environment)
//
// If a chain's family has no registered sequence, Apply returns an error
// listing families that are registered. Use [Registry.RegisteredFamilies] to inspect the
// registry in tests.
//
// # Reference implementation
//
// See mcms/evm/deploy for a complete EVM implementation: idempotent deploy from
// the datastore, timelock role grants, and address metadata output.
package deploy
