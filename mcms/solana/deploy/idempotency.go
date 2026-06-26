package soldeploy

import (
	"strconv"

	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// chainIdempotencyKey scopes an operation report to a single chain. Different
// calls on the same chain are distinguished by the operation's input fields
// (program name, contract type, MCM config, etc.) which are hashed together
// with the operation definition and this key to form the final cache key.
func chainIdempotencyKey[IN, DEP any](chain cldfsol.Chain) operations.ExecuteOption[IN, DEP] {
	return operations.WithIdempotencyKey[IN, DEP](strconv.FormatUint(chain.Selector, 10))
}
