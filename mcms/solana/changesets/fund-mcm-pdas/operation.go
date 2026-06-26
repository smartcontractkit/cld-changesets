package fundmcmpdas

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/gagliardetto/solana-go"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	solana2 "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
)

// OpFundKeyInput is the input of a Solana PDA funding operation.
type OpFundKeyInput struct {
	Target solana.PublicKey `json:"target"`
	Amount uint64           `json:"amount"`
}

// OpFundKeyOutput is the output of a Solana PDA funding operation.
type OpFundKeyOutput struct {
	Confirmed bool `json:"confirmed"`
}

// OpFundKey funds a single Solana account from the chain deployer key.
var OpFundKey = operations.NewOperation(
	"solana-fund-key",
	semver.MustParse("1.0.0"),
	"Funds a Solana account from the deployer key",
	func(b operations.Bundle, deps cldfsol.Chain, in OpFundKeyInput) (OpFundKeyOutput, error) {
		if deps.DeployerKey == nil {
			return OpFundKeyOutput{}, fmt.Errorf("missing deployer key for chain %d", deps.Selector)
		}
		err := solana2.FundFromDeployerKey(
			deps,
			[]solana.PublicKey{in.Target},
			in.Amount,
		)
		if err != nil {
			return OpFundKeyOutput{}, fmt.Errorf("failed to fund target %s: %w", in.Target, err)
		}
		b.Logger.Infow("funding success", "target", in.Target, "amount", in.Amount)

		return OpFundKeyOutput{Confirmed: true}, nil
	},
)
