package maputil

import (
	"cmp"
	"slices"
)

// SortedMapKeys returns the sorted keys of m.
func SortedMapKeys[K cmp.Ordered, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	return keys
}
