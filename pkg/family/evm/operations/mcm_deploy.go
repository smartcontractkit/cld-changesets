package operations

import (
	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"

	zkbindings "github.com/smartcontractkit/mcms/sdk/zksync/bindings"

	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
)

type OpEVMDeployMCMOutput struct {
	Address common.Address `json:"address"`
}

var OpEVMDeployProposerMCM = NewEVMDeployOperation(
	"evm-proposer-mcm-deploy",
	semver.MustParse("1.0.0"),
	"Deploys Proposer MCM contract",
	mcmscontracts.ProposerManyChainMultisig,
	bindings.ManyChainMultiSigMetaData,
	&ContractOpts{
		Version:          &cldchangesetscommon.Version1_0_0,
		EVMBytecode:      common.FromHex(bindings.ManyChainMultiSigBin),
		ZkSyncVMBytecode: zkbindings.ManyChainMultiSigZkBytecode,
	},
	func(input any) []any {
		return []any{}
	},
)

var OpEVMDeployBypasserMCM = NewEVMDeployOperation(
	"evm-bypasser-mcm-deploy",
	semver.MustParse("1.0.0"),
	"Deploys Bypasser MCM contract",
	mcmscontracts.BypasserManyChainMultisig,
	bindings.ManyChainMultiSigMetaData,
	&ContractOpts{
		Version:          &cldchangesetscommon.Version1_0_0,
		EVMBytecode:      common.FromHex(bindings.ManyChainMultiSigBin),
		ZkSyncVMBytecode: zkbindings.ManyChainMultiSigZkBytecode,
	},
	func(input any) []any {
		return []any{}
	},
)

var OpEVMDeployCancellerMCM = NewEVMDeployOperation(
	"evm-canceller-mcm-deploy",
	semver.MustParse("1.0.0"),
	"Deploys Canceller MCM contract",
	mcmscontracts.CancellerManyChainMultisig,
	bindings.ManyChainMultiSigMetaData,
	&ContractOpts{
		Version:          &cldchangesetscommon.Version1_0_0,
		EVMBytecode:      common.FromHex(bindings.ManyChainMultiSigBin),
		ZkSyncVMBytecode: zkbindings.ManyChainMultiSigZkBytecode,
	},
	func(input any) []any {
		return []any{}
	},
)
