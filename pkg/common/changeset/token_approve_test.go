package changeset

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/erc20"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"
)

func TestApproveToken_success_simulatedEVM(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	env, err := environment.New(t.Context(),
		environment.WithEVMSimulated(t, []uint64{selector}),
	)
	require.NoError(t, err)

	chain := env.BlockChains.EVMChains()[selector]

	_, tx, lt, err := link_token.DeployLinkToken(chain.DeployerKey, chain.Client)
	require.NoError(t, err)
	_, err = chain.Confirm(tx)
	require.NoError(t, err)

	tx, err = lt.GrantMintAndBurnRoles(chain.DeployerKey, chain.DeployerKey.From)
	require.NoError(t, err)
	_, err = chain.Confirm(tx)
	require.NoError(t, err)

	tx, err = lt.Mint(chain.DeployerKey, chain.DeployerKey.From, big.NewInt(1_000_000))
	require.NoError(t, err)
	_, err = chain.Confirm(tx)
	require.NoError(t, err)

	spender := common.HexToAddress("0x00000000000000000000000000000000000000Ab")
	amount := big.NewInt(12345)

	require.NoError(t, ApproveToken(*env, selector, lt.Address(), spender, amount))

	token, err := erc20.NewERC20(lt.Address(), chain.Client)
	require.NoError(t, err)
	got, err := token.Allowance(nil, chain.DeployerKey.From, spender)
	require.NoError(t, err)
	require.Zero(t, got.Cmp(amount))
}

func TestApproveToken_unknownChainSelector(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	env, err := environment.New(t.Context(),
		environment.WithEVMSimulated(t, []uint64{selector}),
	)
	require.NoError(t, err)

	err = ApproveToken(*env, selector+999_999_999, common.Address{}, common.Address{}, big.NewInt(1))
	require.ErrorContains(t, err, "not found in environment")
}
