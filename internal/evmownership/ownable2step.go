package evmownership

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Minimal OpenZeppelin Ownable2Step ABI: owner(), transferOwnership(address),
// acceptOwnership().
const ownable2StepABIJSON = `[
	{"inputs":[],"name":"acceptOwnership","outputs":[],"stateMutability":"nonpayable","type":"function"},
	{"inputs":[],"name":"owner","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"internalType":"address","name":"newOwner","type":"address"}],"name":"transferOwnership","outputs":[],"stateMutability":"nonpayable","type":"function"}
]`

var ownable2StepABI = mustParseABI(ownable2StepABIJSON)

func mustParseABI(json string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(json))
	if err != nil {
		panic(fmt.Sprintf("evmownership: invalid Ownable2Step ABI: %v", err))
	}

	return parsed
}

type ownable2Step struct {
	address  common.Address
	contract *bind.BoundContract
}

func (o *ownable2Step) Address() common.Address {
	return o.address
}

func (o *ownable2Step) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	if err := o.contract.Call(opts, &out, "owner"); err != nil {
		return common.Address{}, err
	}

	return *abi.ConvertType(out[0], new(common.Address)).(*common.Address), nil
}

func (o *ownable2Step) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*gethtypes.Transaction, error) {
	return o.contract.Transact(opts, "transferOwnership", newOwner)
}

func (o *ownable2Step) AcceptOwnership(opts *bind.TransactOpts) (*gethtypes.Transaction, error) {
	return o.contract.Transact(opts, "acceptOwnership")
}

// NewOwnable2Step returns an Ownable handle backed by a minimal Ownable2Step ABI.
func NewOwnable2Step(addr common.Address, backend bind.ContractBackend) Ownable {
	return &ownable2Step{
		address:  addr,
		contract: bind.NewBoundContract(addr, ownable2StepABI, backend, backend, backend),
	}
}

// LoadOwnable binds addr as Ownable2Step and returns its current owner.
func LoadOwnable(addr common.Address, backend bind.ContractBackend) (common.Address, Ownable, error) {
	c := NewOwnable2Step(addr, backend)
	owner, err := c.Owner(nil)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to get owner of contract %s: %w", addr.Hex(), err)
	}

	return owner, c, nil
}
