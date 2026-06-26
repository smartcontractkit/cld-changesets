package evmgrantrole

import (
	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/mcms/sdk/evm/bindings"

	evmops "github.com/smartcontractkit/cld-changesets/legacy/mcms/oputils"
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
	"Grants the specified role to the given account on the EVM Timelock contract",
	bindings.RBACTimelockABI,
	mcmscontracts.RBACTimelock,
	bindings.NewRBACTimelock,
	func(timelock *bindings.RBACTimelock, opts *bind.TransactOpts, input OpGrantRoleInput) (*types.Transaction, error) {
		return timelock.GrantRole(opts, input.RoleID, input.Account)
	},
)
