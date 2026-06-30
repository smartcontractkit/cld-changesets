package evmtransfertotimelock

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
)

func resolveEVMAddress(env cldf.Environment, chainSelector uint64, ref refkey.RefKey) (common.Address, error) {
	if ref.ChainSelector != 0 && ref.ChainSelector != chainSelector {
		return common.Address{}, fmt.Errorf(
			"ref chain selector %d does not match chain %d",
			ref.ChainSelector,
			chainSelector,
		)
	}
	if ref.ChainSelector == 0 {
		ref.ChainSelector = chainSelector
	}

	resolved, err := ref.Resolve(env)
	if err != nil {
		return common.Address{}, err
	}

	return parseEVMAddress(resolved.Address, "contract")
}

func parseEVMAddress(addr string, label string) (common.Address, error) {
	if !common.IsHexAddress(addr) {
		return common.Address{}, fmt.Errorf("invalid %s address %q", label, addr)
	}

	parsed := common.HexToAddress(addr)
	if parsed == (common.Address{}) {
		return common.Address{}, fmt.Errorf("%s address must not be zero", label)
	}

	return parsed, nil
}
