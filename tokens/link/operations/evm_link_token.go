package operations

import (
	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	opsevm "github.com/smartcontractkit/cld-changesets/pkg/family/evm/operations"
)

// OpEVMDeployLinkToken deploys a burn/mint ERC677 LINK token contract, with ZkSync support.
var OpEVMDeployLinkToken = opsevm.NewEVMDeployOperation(
	"evm-link-token-deploy",
	semver.MustParse("1.0.0"),
	"Deploys LINK token (burn/mint ERC677) contract",
	linkcontracts.LinkToken,
	link_token.LinkTokenMetaData,
	&opsevm.ContractOpts{
		Version:          &semvers.V1_0_0,
		EVMBytecode:      common.FromHex(link_token.LinkTokenBin),
		ZkSyncVMBytecode: link_token.ZkBytecode,
	},
	func(_ any) []any { return []any{} },
)
