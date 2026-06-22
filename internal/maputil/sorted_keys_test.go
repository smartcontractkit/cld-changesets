package maputil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortedMapKeys(t *testing.T) {
	t.Parallel()

	require.Empty(t, SortedMapKeys(map[uint64]int{}))
	require.Equal(t, []uint64{1, 2, 3}, SortedMapKeys(map[uint64]int{2: 0, 1: 0, 3: 0}))
}
