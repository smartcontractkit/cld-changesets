package operations

import (
	"math/big"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"

	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	zkbindings "github.com/smartcontractkit/mcms/sdk/zksync/bindings"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/legacy/mcms/oputils"
)

type OpDeployTimelockInput struct {
	TimelockMinDelay *big.Int         `json:"timelockMinDelay"`
	Admin            common.Address   `json:"admin"`      // Admin of the timelock contract, usually the deployer key
	Proposers        []common.Address `json:"proposers"`  // Proposer of the timelock contract, usually the deployer key
	Executors        []common.Address `json:"executors"`  // Executor of the timelock contract, usually the call proxy
	Cancellers       []common.Address `json:"cancellers"` // Canceller of the timelock contract, usually the deployer key
	Bypassers        []common.Address `json:"bypassers"`  // Bypasser of the timelock contract, usually the deployer key
}

var OpDeployTimelock = oputils.NewEVMDeployOperation(
	"evm-timelock-deploy",
	semver.MustParse("1.0.0"),
	"Deploys Timelock contract on the specified EVM chains",
	mcmscontracts.RBACTimelock,
	bindings.RBACTimelockMetaData,
	&oputils.ContractOpts{
		Version:          &semvers.V1_0_0,
		EVMBytecode:      common.FromHex(bindings.RBACTimelockBin),
		ZkSyncVMBytecode: zkbindings.RBACTimelockZkBytecode,
	},
	func(input OpDeployTimelockInput) []any {
		return []any{
			input.TimelockMinDelay,
			input.Admin,
			input.Proposers,
			input.Executors,
			input.Cancellers,
			input.Bypassers,
		}
	},
)
