package operations

import (
	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"

	zkbindings "github.com/smartcontractkit/mcms/sdk/zksync/bindings"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

type OpEVMDeployCallProxyInput struct {
	Timelock common.Address `json:"timelock"`
}

var OpEVMDeployCallProxy = NewEVMDeployOperation(
	"evm-call-proxy-deploy",
	semver.MustParse("1.0.0"),
	"Deploys CallProxy contract on the specified EVM chains",
	mcmscontracts.CallProxy,
	bindings.CallProxyMetaData,
	&ContractOpts{
		Version:          &semvers.V1_0_0,
		EVMBytecode:      common.FromHex(bindings.CallProxyBin),
		ZkSyncVMBytecode: zkbindings.CallProxyZkBytecode,
	},
	func(input OpEVMDeployCallProxyInput) []any {
		return []any{
			input.Timelock,
		}
	},
)
