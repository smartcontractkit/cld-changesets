package solreaders

import (
	"testing"

	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

func TestInitRegistersSolanaReader(t *testing.T) {
	t.Parallel()

	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilySolana)
	require.True(t, ok)
	require.Equal(t, Reader{}, reader)
}
