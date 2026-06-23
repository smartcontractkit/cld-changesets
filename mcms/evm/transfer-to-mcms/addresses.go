package evmtransfertomcms

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

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
