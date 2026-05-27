package operations

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/gagliardetto/solana-go"
	solCommonUtil "github.com/smartcontractkit/chainlink-ccip/chains/solana/utils/common"
	solTokenUtil "github.com/smartcontractkit/chainlink-ccip/chains/solana/utils/tokens"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

type SolanaLinkDeployInput struct {
	MintKey  solana.PrivateKey
	Decimals uint8
}

type SolanaLinkDeployOutput struct {
	Address string
}

// OpSolanaDeployLinkToken deploys a LINK SPL token on a Solana chain.
var OpSolanaDeployLinkToken = operations.NewOperation(
	"solana-link-token-deploy",
	semver.MustParse("1.0.0"),
	"Deploys LINK token (SPL token) on a Solana chain",
	func(b operations.Bundle, chain cldf_solana.Chain, input SolanaLinkDeployInput) (SolanaLinkDeployOutput, error) {
		instructions, err := solTokenUtil.CreateToken(
			b.GetContext(),
			solana.TokenProgramID,
			input.MintKey.PublicKey(),
			chain.DeployerKey.PublicKey(),
			input.Decimals,
			chain.Client,
			cldf_solana.SolDefaultCommitment,
		)
		if err != nil {
			return SolanaLinkDeployOutput{}, fmt.Errorf("failed to generate token instructions: %w", err)
		}
		if err = chain.Confirm(instructions, solCommonUtil.AddSigners(input.MintKey)); err != nil {
			return SolanaLinkDeployOutput{}, fmt.Errorf("failed to confirm token deployment: %w", err)
		}

		return SolanaLinkDeployOutput{Address: input.MintKey.PublicKey().String()}, nil
	},
)
