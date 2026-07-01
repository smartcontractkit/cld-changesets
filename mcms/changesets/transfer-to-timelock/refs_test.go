package transfertotimelock

import (
	"testing"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

func TestNormalizeContractRefs_fillsChainSelector(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	ref := refkey.RefKey{
		Type:      "LinkToken",
		Version:   &semvers.V1_0_0,
		Qualifier: "",
	}

	got, err := normalizeContractRefs(selector, []refkey.RefKey{ref})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, selector, got[0].ChainSelector)
}

func TestNormalizeContractRefs_rejectsMismatchedChainSelector(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	other := chainselectors.TEST_90000002.Selector

	_, err := normalizeContractRefs(selector, []refkey.RefKey{
		refkey.New(other, "LinkToken", &semvers.V1_0_0, ""),
	})
	require.ErrorContains(t, err, "does not match chain")
}
