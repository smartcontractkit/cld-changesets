package evmdeploy

import (
	"strconv"

	cldfevm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// chainIdempotencyKey scopes operation report reuse to a single chain. Without
// this, multi-chain deploys can collide on identical operation inputs and reuse
// another chain's cached deployment address.
func chainIdempotencyKey[IN, DEP any](chain cldfevm.Chain) operations.ExecuteOption[IN, DEP] {
	return operations.WithIdempotencyKey[IN, DEP](strconv.FormatUint(chain.Selector, 10))
}

func chainSequenceIdempotencyKey[IN, DEP any](chain cldfevm.Chain) operations.ExecuteSequenceOption[IN, DEP] {
	return operations.WithSequenceIdempotencyKey[IN, DEP](strconv.FormatUint(chain.Selector, 10))
}

// outputAddressIdempotencyKey scopes report reuse to an operation targeting a
// specific deployed contract instance (chain selector + contract address).
func outputAddressIdempotencyKey[IN, DEP any](chain cldfevm.Chain, address string) operations.ExecuteOption[IN, DEP] {
	return operations.WithIdempotencyKey[IN, DEP](strconv.FormatUint(chain.Selector, 10) + ":" + address)
}
