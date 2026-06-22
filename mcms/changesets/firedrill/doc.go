// Package firedrill provides the MCMS signing fire-drill changeset and a registry
// for per-chain-family fire-drill implementations.
//
// # Usage
//
// Import the changeset and blank-import each chain family you use (plus MCMS
// readers for timelock proposal resolution):
//
//	import (
//		"time"
//
//		firedrill "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/evm/firedrill"
//		cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
//		mcmstypes "github.com/smartcontractkit/mcms/types"
//	)
//
//	rt.Exec(runtime.ChangesetTask(firedrill.Changeset{}, firedrill.Input{
//		MCMS: &cldf.MCMSTimelockProposalInput{
//			TimelockAction: mcmstypes.TimelockActionSchedule,
//			ValidUntil:     uint32(time.Now().Add(24 * time.Hour).Unix()),
//			TimelockDelay:  mcmstypes.NewDuration(time.Hour),
//			Description:    "firedrill proposal",
//		},
//		Cfg: firedrill.Config{Selectors: []uint64{selector}},
//	}))
//
// For pipelines that need every built-in family, blank-import [all] instead:
//
//	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill/all"
//
// [Changeset] resolves target chains, runs each chain's fire-drill sequence to
// build noop batch operations from datastore MCMS refs, and returns an MCMS
// timelock proposal via [cldf.OutputBuilder.WithTimelockProposal].
//
// # MCMS qualifier
//
// MCMS and timelock refs are resolved from the environment datastore using
// [cldf.MCMSTimelockProposalInput.Qualifier]. That qualifier applies to every
// chain in the drill. Chains that use different qualifiers (for example, one
// EVM deployment with "" and another with "staging") must be run as separate
// fire-drill changeset executions with matching Qualifier values.
//
// # Built-in EVM and Solana support
//
// EVM and Solana register themselves via init when their packages are imported.
// Importing firedrill alone does not register any family — you must blank-import
// mcms/<family>/firedrill (and mcms/<family>/readers for MCMS resolution).
//
// # Reference implementations
//
// See mcms/evm/firedrill and mcms/solana/firedrill for complete implementations.
package firedrill
