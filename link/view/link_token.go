package view

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"

	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
)

type LinkTokenView struct {
	cldchangesetscommon.ContractMetaData
	Decimals uint8            `json:"decimals"`
	Supply   *big.Int         `json:"supply"`
	Minters  []common.Address `json:"minters"`
	Burners  []common.Address `json:"burners"`
}

func GenerateLinkTokenView(lt *link_token.LinkToken) (LinkTokenView, error) {
	owner, err := lt.Owner(nil)
	if err != nil {
		owner = common.Address{}
	}
	decimals, err := lt.Decimals(nil)
	if err != nil {
		return LinkTokenView{}, fmt.Errorf("failed to get decimals %s: %w", lt.Address(), err)
	}
	totalSupply, err := lt.TotalSupply(nil)
	if err != nil {
		return LinkTokenView{}, fmt.Errorf("failed to get total supply %s: %w", lt.Address(), err)
	}
	minters, err := lt.GetMinters(nil)
	if err != nil {
		minters = []common.Address{}
	}
	burners, err := lt.GetBurners(nil)
	if err != nil {
		burners = []common.Address{}
	}
	return LinkTokenView{
		ContractMetaData: cldchangesetscommon.ContractMetaData{
			TypeAndVersion: cldf.TypeAndVersion{
				Type:    linkcontracts.LinkToken,
				Version: cldchangesetscommon.Version1_0_0,
			}.String(),
			Address: lt.Address(),
			Owner:   owner,
		},
		Decimals: decimals,
		Supply:   totalSupply,
		Minters:  minters,
		Burners:  burners,
	}, nil
}
