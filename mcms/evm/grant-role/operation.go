// Package evmgrantrole houses the EVM operation(s) for granting timelock
// roles. It lives alongside where its dedicated changeset would (once it is
// added) so all related code is colocated.
package evmgrantrole

import (
	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/mcms/sdk/evm/bindings"

	evmops "github.com/smartcontractkit/cld-changesets/mcms/evm/operations"
)

// OpGrantRoleInput is the input to OpGrantRole.
type OpGrantRoleInput struct {
	Account common.Address `json:"account"`
	RoleID  [32]byte       `json:"roleID"`
}

// OpGrantRole grants the given role to the given account on the EVM Timelock contract.
// TODO: refactor to use mcms lib
var OpGrantRole = evmops.NewEVMCallOperation(
	"evm-timelock-grant-role",
	semver.MustParse("1.0.0"),
	"Grants specified role to the ManyChainMultiSig contract on the EVM Timelock contract",
	bindings.RBACTimelockABI,
	mcmscontracts.RBACTimelock,
	bindings.NewRBACTimelock,
	func(timelock *bindings.RBACTimelock, opts *bind.TransactOpts, input OpGrantRoleInput) (*types.Transaction, error) {
		return timelock.GrantRole(opts, input.RoleID, input.Account)
	},
)
