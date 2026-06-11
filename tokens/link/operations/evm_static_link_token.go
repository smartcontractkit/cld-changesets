package operations

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/generated/link_token_interface"

	"github.com/smartcontractkit/cld-changesets/tokens/link/types"
)

// OpEVMDeployStaticLinkToken deploys a non-burn/mint static LINK token contract.
var OpEVMDeployStaticLinkToken = contract.NewDeploy(contract.DeployParams[struct{}]{
	Name:             "evm-static-link-token-deploy",
	Description:      "Deploys static LINK token (non-burn/mint) contract",
	Version:          &types.StaticLinkTokenTypeAndVersion.Version,
	ContractMetadata: link_token_interface.LinkTokenMetaData,
	BytecodeByTypeAndVersion: map[string]contract.Bytecode{
		types.StaticLinkTokenTypeAndVersion.String(): {
			EVM: common.FromHex(link_token_interface.LinkTokenBin),
		},
	},
})
