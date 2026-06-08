package evmtest

import (
	"crypto/rand"

	"github.com/ethereum/go-ethereum/common"
)

// RandomAddress generates a random EVM address.
func RandomAddress() common.Address {
	b := make([]byte, 20)
	_, _ = rand.Read(b) // Assignment for errcheck. Only used in tests so we can ignore.

	return common.BytesToAddress(b)
}
