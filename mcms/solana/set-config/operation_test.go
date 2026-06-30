package solsetconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
)

func TestOpSolanaSetConfigMCM_missingDeployerKey(t *testing.T) {
	t.Parallel()

	_, err := operations.ExecuteOperation(
		optest.NewBundle(t),
		OpSolanaSetConfigMCM,
		cldfsol.Chain{Selector: chainselectors.TEST_22222222222222222222222222222222222222222222.Selector},
		OpSolanaSetConfigInput{
			NoSend: false,
			Target: MCMSetConfigTarget{
				Address: "some-address",
			},
		},
	)
	require.ErrorContains(t, err, "missing deployer key")
}
