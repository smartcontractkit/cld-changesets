package solgrantrole

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	solanago "github.com/gagliardetto/solana-go"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

// GrantRoleTarget identifies one timelock role grant on Solana.
type GrantRoleTarget struct {
	Timelock string               `json:"timelock"`
	Role     mcmssdk.TimelockRole `json:"role"`
	Address  string               `json:"address"`
}

// OpSolanaGrantRoleInput is the input for granting one timelock role on Solana.
type OpSolanaGrantRoleInput struct {
	Target           GrantRoleTarget    `json:"target"`
	NoSend           bool               `json:"noSend"`
	AuthorityAccount solanago.PublicKey `json:"authorityAccount"`
}

// OpSolanaGrantRoleOutput is the output of a Solana grant-role operation.
type OpSolanaGrantRoleOutput struct {
	Confirmed      bool                     `json:"confirmed"`
	BatchOperation mcmstypes.BatchOperation `json:"batchOperation"`
	Signature      string                   `json:"signature,omitempty"`
}

// OpSolanaGrantRole grants a timelock role via the MCMS SDK timelock configurer.
var OpSolanaGrantRole = operations.NewOperation(
	"solana-grant-role",
	semver.MustParse("1.0.0"),
	"Grants a timelock role to one Solana address via the MCMS SDK timelock configurer",
	func(b operations.Bundle, deps cldfsol.Chain, in OpSolanaGrantRoleInput) (OpSolanaGrantRoleOutput, error) {
		if err := validateGrantRoleTarget(in.Target); err != nil {
			return OpSolanaGrantRoleOutput{}, err
		}
		if deps.DeployerKey == nil {
			return OpSolanaGrantRoleOutput{}, fmt.Errorf("missing deployer key for chain %d", deps.Selector)
		}
		if deps.Client == nil {
			return OpSolanaGrantRoleOutput{}, fmt.Errorf("missing rpc client for chain %d", deps.Selector)
		}

		configurer := newGrantRoleTimelockConfigurer(deps, in)

		res, err := configurer.GrantRole(b.GetContext(), in.Target.Timelock, in.Target.Role, in.Target.Address)
		if err != nil {
			return OpSolanaGrantRoleOutput{}, fmt.Errorf("grant role %s to %s on %s: %w",
				in.Target.Role.String(), in.Target.Address, in.Target.Timelock, err)
		}

		if in.NoSend {
			tx, ok := res.RawData.(mcmstypes.Transaction)
			if !ok {
				return OpSolanaGrantRoleOutput{}, fmt.Errorf("unexpected raw data type %T from GrantRole", res.RawData)
			}

			return OpSolanaGrantRoleOutput{
				BatchOperation: mcmstypes.BatchOperation{
					ChainSelector: mcmstypes.ChainSelector(deps.Selector),
					Transactions:  []mcmstypes.Transaction{tx},
				},
			}, nil
		}

		b.Logger.Infow("GrantRole confirmed",
			"chainSelector", deps.Selector,
			"timelock", in.Target.Timelock,
			"role", in.Target.Role.String(),
			"address", in.Target.Address,
			"signature", res.Hash,
		)

		return OpSolanaGrantRoleOutput{Confirmed: true, Signature: res.Hash}, nil
	},
)

func newGrantRoleTimelockConfigurer(deps cldfsol.Chain, in OpSolanaGrantRoleInput) *mcmssolana.TimelockConfigurer {
	client := deps.Client
	auth := *deps.DeployerKey

	if in.NoSend {
		if !in.AuthorityAccount.IsZero() {
			return mcmssolana.NewTimelockConfigurer(
				client,
				auth,
				mcmssolana.WithDoNotSendTimelockInstructionsOnChain(),
				mcmssolana.WithTimelockAuthorityAccount(in.AuthorityAccount),
			)
		}

		return mcmssolana.NewTimelockConfigurer(
			client,
			auth,
			mcmssolana.WithDoNotSendTimelockInstructionsOnChain(),
		)
	}

	if !in.AuthorityAccount.IsZero() {
		return mcmssolana.NewTimelockConfigurer(
			client,
			auth,
			mcmssolana.WithTimelockAuthorityAccount(in.AuthorityAccount),
		)
	}

	return mcmssolana.NewTimelockConfigurer(client, auth)
}

func validateGrantRoleTarget(target GrantRoleTarget) error {
	if target.Timelock == "" {
		return errors.New("timelock address must not be empty")
	}
	if !target.Role.Valid() {
		return errors.New("role is unsupported")
	}
	if target.Role == mcmssdk.TimelockRoleAdmin {
		return errors.New("admin role not supported on solana")
	}
	if target.Address == "" {
		return errors.New("address must not be empty")
	}

	return nil
}
