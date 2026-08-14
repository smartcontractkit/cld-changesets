package operations

import (
	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"

	bindings "github.com/smartcontractkit/ccip-owner-contracts/gethwrappers"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	zkbindings "github.com/smartcontractkit/mcms/sdk/zksync/bindings"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/legacy/mcms/oputils"
)

type OpEVMDeployMCMOutput struct {
	Address common.Address `json:"address"`
}

var OpDeployProposerMCM = oputils.NewEVMDeployOperation(
	"evm-proposer-mcm-deploy",
	semver.MustParse("1.0.0"),
	"Deploys Proposer MCM contract",
	mcmscontracts.ProposerManyChainMultisig,
	bindings.ManyChainMultiSigMetaData,
	&oputils.ContractOpts{
		Version:          &semvers.V1_0_0,
		EVMBytecode:      common.FromHex(bindings.ManyChainMultiSigBin),
		ZkSyncVMBytecode: zkbindings.ManyChainMultiSigZkBytecode,
	},
	func(input any) []any {
		return []any{}
	},
)

var OpDeployBypasserMCM = oputils.NewEVMDeployOperation(
	"evm-bypasser-mcm-deploy",
	semver.MustParse("1.0.0"),
	"Deploys Bypasser MCM contract",
	mcmscontracts.BypasserManyChainMultisig,
	bindings.ManyChainMultiSigMetaData,
	&oputils.ContractOpts{
		Version:          &semvers.V1_0_0,
		EVMBytecode:      common.FromHex(bindings.ManyChainMultiSigBin),
		ZkSyncVMBytecode: zkbindings.ManyChainMultiSigZkBytecode,
	},
	func(input any) []any {
		return []any{}
	},
)

var OpDeployCancellerMCM = oputils.NewEVMDeployOperation(
	"evm-canceller-mcm-deploy",
	semver.MustParse("1.0.0"),
	"Deploys Canceller MCM contract",
	mcmscontracts.CancellerManyChainMultisig,
	bindings.ManyChainMultiSigMetaData,
	&oputils.ContractOpts{
		Version:          &semvers.V1_0_0,
		EVMBytecode:      common.FromHex(bindings.ManyChainMultiSigBin),
		ZkSyncVMBytecode: zkbindings.ManyChainMultiSigZkBytecode,
	},
	func(input any) []any {
		return []any{}
	},
)
