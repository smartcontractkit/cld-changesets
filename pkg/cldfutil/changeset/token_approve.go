package changeset

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/erc20"
)

// ApproveToken approves routerAddress to pull up to amount of token from the deployer on chain src.
func ApproveToken(env cldf.Environment, src uint64, tokenAddress common.Address, routerAddress common.Address, amount *big.Int) error {
	evmChains := env.BlockChains.EVMChains()
	ch, ok := evmChains[src]
	if !ok {
		return fmt.Errorf("evm chain %d not found in environment", src)
	}

	if ch.Client == nil {
		return fmt.Errorf("evm chain %d has no RPC client", src)
	}

	if ch.DeployerKey == nil {
		return fmt.Errorf("evm chain %d has no deployer key", src)
	}

	token, err := erc20.NewERC20(tokenAddress, ch.Client)
	if err != nil {
		return err
	}

	tx, err := token.Approve(ch.DeployerKey, routerAddress, amount)
	if err != nil {
		return err
	}

	_, err = ch.Confirm(tx)
	if err != nil {
		return err
	}

	return nil
}
