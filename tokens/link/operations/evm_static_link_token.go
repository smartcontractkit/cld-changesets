package operations

import (
	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/generated/link_token_interface"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	opsevm "github.com/smartcontractkit/cld-changesets/pkg/family/evm/operations"
)

// OpEVMDeployStaticLinkToken deploys a non-burn/mint static LINK token contract.
var OpEVMDeployStaticLinkToken = opsevm.NewEVMDeployOperation(
	"evm-static-link-token-deploy",
	semver.MustParse("1.0.0"),
	"Deploys static LINK token (non-burn/mint) contract",
	linkcontracts.StaticLinkToken,
	link_token_interface.LinkTokenMetaData,
	&opsevm.ContractOpts{
		Version:     &semvers.V1_0_0,
		EVMBytecode: common.FromHex(link_token_interface.LinkTokenBin),
	},
	func(_ any) []any { return []any{} },
)
