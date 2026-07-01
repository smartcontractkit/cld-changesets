// Package transfertotimelock provides the transfer-to-timelock changeset and a registry
// for per-chain-family implementations.
//
// # Usage
//
// Import the changeset and blank-import each chain family's transfer-to-timelock
// sequence (plus MCMS readers when building timelock proposals):
//
//	import (
//		transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
//		"github.com/smartcontractkit/cld-changesets/datastore/refkey"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/evm/transfer-to-timelock"
//	)
//
//	rt.Exec(runtime.ChangesetTask(transfertotimelock.Changeset{}, transfertotimelock.Input{
//		Cfg: transfertotimelock.Config{
//			ContractsByChain: map[uint64][]refkey.RefKey{
//				chainSelector: {
//					refkey.New(chainSelector, contractType, version, qualifier),
//				},
//			},
//		},
//		MCMS: mcmsInput,
//	}))
//
// For pipelines that need every built-in family, blank-import [all] instead:
//
//	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock/all"
//
// MCMS timelock refs are resolved from the environment datastore only.
package transfertotimelock
