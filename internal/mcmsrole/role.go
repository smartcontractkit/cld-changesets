// Package mcmsrole provides RBACTimelock role names and IDs used by MCMS changesets.
//
// RBACTimelock defines five roles: ADMIN, PROPOSER, EXECUTOR, BYPASSER, and
// CANCELLER. Each role ID is the keccak256 hash of its name string, matching
// OpenZeppelin AccessControl. See RBACTimelock in ccip-owner-contracts:
// https://github.com/smartcontractkit/ccip-owner-contracts/blob/9d81692b324ce7ea2ef8a75e683889edbc7e2dd0/src/RBACTimelock.sol#L71
//
// These constants are defined here instead of via generated Go bindings so
// changesets can reference role IDs without importing timelock contract wrappers.
//
// The package lives under internal/ because both legacy and top-level changesets
// depend on it. Once MCMS changesets are consolidated, consider moving it into
// the mcms changesets tree or promoting it to the mcms library.
package mcmsrole

import (
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
)

// Each role string maps to the role names in the RBACTimelock contract.
// https://github.com/smartcontractkit/ccip-owner-contracts/blob/9d81692b324ce7ea2ef8a75e683889edbc7e2dd0/src/RBACTimelock.sol#L71-L75
const (
	adminRoleStr     = "ADMIN_ROLE"
	proposerRoleStr  = "PROPOSER_ROLE"
	bypasserRoleStr  = "BYPASSER_ROLE" //nolint:gosec // G101: These are not secrets and only used in tests.
	cancellerRoleStr = "CANCELLER_ROLE"
	executorRoleStr  = "EXECUTOR_ROLE"
)

var (
	AdminRole     = NewRole(adminRoleStr)
	ProposerRole  = NewRole(proposerRoleStr)
	BypasserRole  = NewRole(bypasserRoleStr)
	CancellerRole = NewRole(cancellerRoleStr)
	ExecutorRole  = NewRole(executorRoleStr)
)

// Role represents a role in the MCMS Timelock contracts.
type Role struct {
	ID   common.Hash
	Name string
}

// NewRole creates a new role with the given name and calculates the ID.
func NewRole(name string) Role {
	return Role{
		ID:   mustKeccakHash(name),
		Name: name,
	}
}

// mustKeccakHash calculates the keccak256 hash of the input string.
func mustKeccakHash(in string) common.Hash {
	hash := sha3.NewLegacyKeccak256()
	if _, err := hash.Write([]byte(in)); err != nil {
		panic(err)
	}

	return common.BytesToHash(hash.Sum(nil))
}
