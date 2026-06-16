package evmreaders

import (
	"testing"

	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

func TestInitRegistersEVMReader(t *testing.T) {
	t.Parallel()

	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyEVM)
	require.True(t, ok)
	require.Equal(t, Reader{}, reader)
}
