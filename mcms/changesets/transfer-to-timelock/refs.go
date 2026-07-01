package transfertotimelock

import (
	"fmt"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
)

func normalizeContractRefs(chainSelector uint64, refs []refkey.RefKey) ([]refkey.RefKey, error) {
	normalized := make([]refkey.RefKey, len(refs))
	for i, ref := range refs {
		if ref.ChainSelector != 0 && ref.ChainSelector != chainSelector {
			return nil, fmt.Errorf(
				"contracts[%d]: ref chain selector %d does not match chain %d",
				i,
				ref.ChainSelector,
				chainSelector,
			)
		}

		ref.ChainSelector = chainSelector
		normalized[i] = ref
	}

	return normalized, nil
}
