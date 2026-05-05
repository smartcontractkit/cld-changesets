package operations

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/gagliardetto/solana-go"

	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	accesscontrollerbindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/access_controller"
	timelockbindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/timelock"
	cldfsolana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	solanastate "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

type OpSolanaGrantRoleTimelockDeps struct {
	Chain cldfsolana.Chain
}

type OpSolanaGrantRoleTimelockInput struct {
	ChainState         *solanastate.MCMSWithTimelockState `json:"chainState"`
	Role               timelockbindings.Role              `json:"role"`
	Account            solana.PublicKey                   `json:"account"`
	IsDeployerKeyAdmin bool                               `json:"isDeployerKeyAdmin"`
}

type OpSolanaGrantRoleTimelockOutput struct {
	MCMSTransaction mcmstypes.Transaction `json:"mcmsTransaction"`
}

var OpSolanaGrantRoleTimelock = fwops.NewOperation(
	"solana-grant-role-timelock",
	semver.MustParse("1.0.0"),
	"Grant a role to an account in a Solana Timelock instance",
	func(b fwops.Bundle, deps OpSolanaGrantRoleTimelockDeps, in OpSolanaGrantRoleTimelockInput) (OpSolanaGrantRoleTimelockOutput, error) {
		accessController, err := selectAccessControllerGrantRole(in)
		if err != nil {
			return OpSolanaGrantRoleTimelockOutput{}, fmt.Errorf("failed to select access controller: %w", err)
		}

		timelockbindings.SetProgramID(in.ChainState.TimelockProgram)
		accesscontrollerbindings.SetProgramID(in.ChainState.AccessControllerProgram)
		var signer solana.PublicKey
		if in.IsDeployerKeyAdmin {
			signer = deps.Chain.DeployerKey.PublicKey()
		} else {
			signer = solanastate.GetTimelockSignerPDA(in.ChainState.TimelockProgram, in.ChainState.TimelockSeed)
		}

		ix, err := accesscontrollerbindings.NewAddAccessInstruction(accessController, signer, in.Account).ValidateAndBuild()
		if err != nil {
			return OpSolanaGrantRoleTimelockOutput{}, fmt.Errorf("failed to create update delay instruction: %w", err)
		}

		if in.IsDeployerKeyAdmin {
			cerr := deps.Chain.SendAndConfirm(b.GetContext(), []solana.Instruction{ix})
			if cerr != nil {
				return OpSolanaGrantRoleTimelockOutput{}, fmt.Errorf("failed to confirm instructions: %w", cerr)
			}

			return OpSolanaGrantRoleTimelockOutput{}, nil
		}

		transaction, err := mcmssolanasdk.NewTransactionFromInstruction(ix, "AccessController", []string{})
		if err != nil {
			return OpSolanaGrantRoleTimelockOutput{}, fmt.Errorf("failed to create transaction: %w", err)
		}

		return OpSolanaGrantRoleTimelockOutput{MCMSTransaction: transaction}, nil
	},
)

func selectAccessControllerGrantRole(in OpSolanaGrantRoleTimelockInput) (solana.PublicKey, error) {
	switch in.Role {
	case timelockbindings.Admin_Role:
		return solana.PublicKey{}, errors.New("admin role not supported")
	case timelockbindings.Proposer_Role:
		return in.ChainState.ProposerAccessControllerAccount, nil
	case timelockbindings.Executor_Role:
		return in.ChainState.ExecutorAccessControllerAccount, nil
	case timelockbindings.Canceller_Role:
		return in.ChainState.CancellerAccessControllerAccount, nil
	case timelockbindings.Bypasser_Role:
		return in.ChainState.BypasserAccessControllerAccount, nil
	default:
		return solana.PublicKey{}, fmt.Errorf("unknown role %s", in.Role)
	}
}

type SeqSolanaGrantRoleTimelockDeps struct {
	Chain cldfsolana.Chain
}

type SeqSolanaGrantRoleTimelockInput struct {
	ChainState         *solanastate.MCMSWithTimelockState `json:"chainState"`
	Role               timelockbindings.Role              `json:"role"`
	Accounts           []solana.PublicKey                 `json:"accounts"`
	IsDeployerKeyAdmin bool                               `json:"isDeployerKeyAdmin"`
}

type SeqSolanaGrantRoleTimelockOutput struct {
	McmsTransactions []mcmstypes.Transaction `json:"mcmsTxs"`
}

var SeqSolanaGrantRoleTimelock = fwops.NewSequence(
	"seq-solana-grant-role-timelock",
	semver.MustParse("1.0.0"),
	"Grant a role to multiple accounts in a Solana Timelock instance",
	func(b fwops.Bundle, deps SeqSolanaGrantRoleTimelockDeps, in SeqSolanaGrantRoleTimelockInput) (SeqSolanaGrantRoleTimelockOutput, error) {
		mcmsTxs := make([]mcmstypes.Transaction, 0, len(in.Accounts))

		for _, account := range in.Accounts {
			opReport, err := fwops.ExecuteOperation(b, OpSolanaGrantRoleTimelock,
				OpSolanaGrantRoleTimelockDeps(deps),
				OpSolanaGrantRoleTimelockInput{
					ChainState:         in.ChainState,
					Role:               in.Role,
					Account:            account,
					IsDeployerKeyAdmin: in.IsDeployerKeyAdmin,
				},
			)
			if err != nil {
				b.Logger.Errorw("Failed to grant role", "chainSelector", deps.Chain.ChainSelector(), "chainName", deps.Chain.Name(),
					"timelock", solanastate.EncodeAddressWithSeed(in.ChainState.TimelockProgram, in.ChainState.TimelockSeed),
					"role", in.Role, "account", account)

				return SeqSolanaGrantRoleTimelockOutput{}, err
			}

			mcmsTxs = append(mcmsTxs, opReport.Output.MCMSTransaction)
		}

		return SeqSolanaGrantRoleTimelockOutput{McmsTransactions: mcmsTxs}, nil
	},
)
