package solsetconfig

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	solanasdk "github.com/gagliardetto/solana-go"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

type mcmSetConfigTarget struct {
	Address      string           `json:"address"`
	Config       mcmstypes.Config `json:"config"`
	ContractType string           `json:"contractType"`
}

// OpSolanaSetConfigInput is the input for setting config on a single Solana MCM account.
type OpSolanaSetConfigInput struct {
	Target           mcmSetConfigTarget  `json:"target"`
	NoSend           bool                `json:"noSend"`
	AuthorityAccount solanasdk.PublicKey `json:"authorityAccount"`
}

// OpSolanaSetConfigOutput is the output of a Solana set-config operation.
type OpSolanaSetConfigOutput struct {
	Confirmed      bool                     `json:"confirmed"`
	BatchOperation mcmstypes.BatchOperation `json:"batchOperation"`
}

// OpSolanaSetConfigMCM sets MCMS config on a Solana MCM account.
var OpSolanaSetConfigMCM = operations.NewOperation(
	"solana-mcm-set-config",
	semver.MustParse("1.0.0"),
	"Sets MCMS config on a Solana MCM account",
	func(b operations.Bundle, deps cldfsol.Chain, in OpSolanaSetConfigInput) (OpSolanaSetConfigOutput, error) {
		if deps.DeployerKey == nil {
			return OpSolanaSetConfigOutput{}, fmt.Errorf("missing deployer key for chain %d", deps.Selector)
		}

		var configurer *mcmssolana.Configurer

		if in.NoSend {
			configurer = mcmssolana.NewConfigurer(
				deps.Client,
				*deps.DeployerKey,
				mcmstypes.ChainSelector(deps.Selector),
				mcmssolana.WithDoNotSendInstructionsOnChain(),
				mcmssolana.WithAuthorityAccount(in.AuthorityAccount),
			)
		} else {
			configurer = mcmssolana.NewConfigurer(
				deps.Client,
				*deps.DeployerKey,
				mcmstypes.ChainSelector(deps.Selector),
			)
		}

		res, err := configurer.SetConfig(b.GetContext(), in.Target.Address, &in.Target.Config, false)
		if err != nil {
			return OpSolanaSetConfigOutput{}, fmt.Errorf("failed to set config on %s: %w", in.Target.Address, err)
		}

		if in.NoSend {
			instructions, ok := res.RawData.([]solanasdk.Instruction)
			if !ok {
				return OpSolanaSetConfigOutput{}, fmt.Errorf("unexpected raw data type %T from SetConfig", res.RawData)
			}

			txs := make([]mcmstypes.Transaction, 0, len(instructions))
			for _, ix := range instructions {
				tx, txErr := mcmssolana.NewTransactionFromInstruction(ix, in.Target.ContractType, []string{})
				if txErr != nil {
					return OpSolanaSetConfigOutput{}, txErr
				}
				txs = append(txs, tx)
			}

			return OpSolanaSetConfigOutput{
				BatchOperation: mcmstypes.BatchOperation{
					ChainSelector: mcmstypes.ChainSelector(deps.Selector),
					Transactions:  txs,
				},
			}, nil
		}

		b.Logger.Infow("SetConfig tx confirmed", "txHash", res.Hash, "address", in.Target.Address)

		return OpSolanaSetConfigOutput{Confirmed: true}, nil
	},
)
