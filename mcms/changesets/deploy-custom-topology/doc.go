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
// # Example: one proposer MCM and one timelock
//
// [MCMSTopologyConfig.ChainConfigs] maps chain selectors to the topology to deploy
// on that chain. Each chain declares any number of [MCMSpec] entries (the MCMs to
// deploy) and [TimelockSpec] entries (the timelocks to deploy). Ref fields are local
// handles used within that chain; [RoleHolder.MCMRef] must match an [MCMSpec.Ref]
// on the same chain.
//
// Minimal example — one proposer MCM wired as the sole proposer on one timelock:
//
//	rt.Exec(runtime.ChangesetTask(topology.Changeset{}, topology.Input{
//		Cfg: topology.MCMSTopologyConfig{
//			ChainConfigs: map[uint64]topology.ChainTopologyConfig{
//				selector: {
//					MCMs: []topology.MCMSpec{{
//						Ref:          "proposer",
//						Qualifier:    "CCIP",
//						ContractType: "ProposerManyChainMultisig",
//						Config:       proposerCfg, // mcmstypes.Config: signers + quorum
//					}},
//					Timelocks: []topology.TimelockSpec{{
//						Ref:       "timelock",
//						Qualifier: "CCIP",
//						MinDelay:  big.NewInt(3600),
//						Roles: topology.RoleAssignments{
//							Proposers: []topology.RoleHolder{{MCMRef: "proposer"}},
//						},
//					}},
//				},
//			},
//		},
//	}))
//
//	// CLD:
//	registry.Add("deploy_custom_topology", Configure(topology.Changeset{}).WithEnvInput())
//
// Multiple MCMs on the same timelock — add more [MCMSpec] entries and reference
// them from [TimelockSpec.Roles]:
//
//	MCMs: []topology.MCMSpec{
//		{Ref: "proposer", Qualifier: "CCIP", ContractType: "ProposerManyChainMultisig", Config: proposerCfg},
//		{Ref: "bypasser", Qualifier: "CCIP", ContractType: "BypasserManyChainMultisig", Config: bypasserCfg},
//	},
//	Timelocks: []topology.TimelockSpec{{
//		Ref: "timelock", Qualifier: "CCIP", MinDelay: big.NewInt(3600),
//		Roles: topology.RoleAssignments{
//			Proposers:  []topology.RoleHolder{{MCMRef: "proposer"}},
//			Bypassers:  []topology.RoleHolder{{MCMRef: "bypasser"}},
//			Cancellers: []topology.RoleHolder{{MCMRef: "proposer"}, {MCMRef: "bypasser"}},
//		},
//	}},
//
// Leave [Input.MCMS] nil for deploy-only runs. Set [TimelockSpec.TransferOwnership]
// to move ownership onto a timelock: the changeset executes transferOwnership on-chain
// and returns accept-ownership timelock proposals grouped by timelock qualifier.
//
// See mcms/evm/deploy-custom-topology/sequence_test.go for full EVM examples,
// including the standard three-MCM topology and multi-qualifier setups.
//
// # Behaviour
//
// [Changeset] iterates the configured chains, runs each chain's registered deploy
// sequence, writes deployed-address metadata to the changeset datastore, and — when
// [Input.MCMS] is set and a timelock takes ownership of contracts — builds one MCMS
// timelock proposal per timelock qualifier from the returned batch operations.
//
// Each family sequence returns [sequenceutils.OnChainOutput]. Populate Metadata with
// deployed addresses and BatchOps with accept-ownership batch operations (only when
// running with MCMS input). The changeset groups batch operations by timelock qualifier
// when building proposals.
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
//   - register.go — exports Registration() and calls [Registry.Register] in init()
//   - sequence.go — operations.Sequence that deploys the topology for one chain
//   - addresses.go — datastore ref resolution / AddressRef construction
//   - extra_args.go — optional, parses family-specific [ChainTopologyConfig.ExtraArgs]
//
// The sequence must match [Sequence]: it receives [ChainInput] and [Deps] and returns
// [sequenceutils.OnChainOutput].
//
// Register at startup from init (recommended, mirrors EVM and Solana):
//
//	func init() { deploycustomtopology.Registry.Register(Registration()) }
//
// Each family may only be registered once; duplicate registration panics. Use
// [Registry.RegisteredFamilies] to inspect the registry in tests.
package deploycustomtopology
