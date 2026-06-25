package evmdeploytopology

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	deploycustomtopology "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy-custom-topology"
)

func TestVerifyEVMChains(t *testing.T) {
	t.Parallel()

	env := cldf.Environment{BlockChains: chain.NewBlockChains(nil)}
	err := verifyEVMChains(env, []deploycustomtopology.ChainInput{
		{ChainSelector: chainselectors.TEST_90000001.Selector},
	})
	require.EqualError(t, err, fmt.Sprintf("EVM chain %d not found in environment", chainselectors.TEST_90000001.Selector))
}

func TestRegistration(t *testing.T) {
	t.Parallel()

	reg := Registration()
	require.Equal(t, chainselectors.FamilyEVM, reg.Family)
	require.NotNil(t, reg.Sequence)
	require.NotNil(t, reg.Verify)
}
