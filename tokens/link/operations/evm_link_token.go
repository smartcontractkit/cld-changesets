package operations

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"

	"github.com/smartcontractkit/cld-changesets/tokens/link/types"
)

var OpEVMDeployLinkToken = contract.NewDeploy(contract.DeployParams[struct{}]{
	Name:             "evm-link-token-deploy",
	Description:      "Deploys LINK token (burn/mint ERC677) contract",
	Version:          &types.BurnMintLinkTokenTypeAndVersion.Version,
	ContractMetadata: link_token.LinkTokenMetaData,
	BytecodeByTypeAndVersion: map[string]contract.Bytecode{
		types.BurnMintLinkTokenTypeAndVersion.String(): {
			EVM:      common.FromHex(link_token.LinkTokenBin),
			ZkSyncVM: link_token.ZkBytecode,
		},
	},
})
