// Package transfertomcms provides the TransferToMCMS changeset and a registry
// for per-chain-family implementations.
//
// # Usage
//
// Import the changeset and blank-import each chain family's transfer-to-mcms
// sequence (plus MCMS readers when building timelock proposals):
//
//	import (
//		"github.com/ethereum/go-ethereum/common"
//
//		transfertomcms "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-mcms"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/evm/transfer-to-mcms"
//	)
//
//	rt.Exec(runtime.ChangesetTask(transfertomcms.Changeset{}, transfertomcms.Input{
//		Cfg: transfertomcms.Config{
//			ContractsByChain: map[uint64][]common.Address{
//				chainSelector: {common.HexToAddress("0x...")},
//			},
//		},
//		MCMS: mcmsInput,
//	}))
//
// For pipelines that need every built-in family, blank-import [all] instead:
//
//	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-mcms/all"
//
// MCMS timelock refs are resolved from the environment datastore only.
package transfertomcms
