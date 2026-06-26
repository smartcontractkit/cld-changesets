package solfundmcmpdas

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	solCommonUtil "github.com/smartcontractkit/chainlink-ccip/chains/solana/utils/common"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	"github.com/stretchr/testify/require"
)

func TestOpFundKey(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	deployerKey := solana.NewWallet().PrivateKey
	target := solana.NewWallet().PublicKey()
	var confirmed []solana.Instruction

	chain := cldf_solana.Chain{
		Selector:    selector,
		DeployerKey: &deployerKey,
		Confirm: func(instructions []solana.Instruction, _ ...solCommonUtil.TxModifier) error {
			confirmed = append(confirmed, instructions...)
			return nil
		},
	}

	report, err := operations.ExecuteOperation(
		optest.NewBundle(t),
		OpFundKey,
		chain,
		OpFundKeyInput{
			Target: target,
			Amount: 42,
		},
	)
	require.NoError(t, err)
	require.True(t, report.Output.Confirmed)
	require.Len(t, confirmed, 1)

	ix := confirmed[0]
	require.True(t, ix.ProgramID().Equals(system.ProgramID))
	accounts := ix.Accounts()
	require.True(t, accounts[0].PublicKey.Equals(deployerKey.PublicKey()))
	require.True(t, accounts[1].PublicKey.Equals(target))
}

func TestOpFundKey_missingDeployerKey(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	_, err := operations.ExecuteOperation(
		optest.NewBundle(t),
		OpFundKey,
		cldf_solana.Chain{Selector: selector},
		OpFundKeyInput{
			Target: solana.NewWallet().PublicKey(),
			Amount: 1,
		},
	)
	require.ErrorContains(t, err, "missing deployer key")
}

func TestOpFundKey_confirmError(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	deployerKey := solana.NewWallet().PrivateKey
	chain := cldf_solana.Chain{
		Selector:    selector,
		DeployerKey: &deployerKey,
		Confirm: func(_ []solana.Instruction, _ ...solCommonUtil.TxModifier) error {
			return assertAnError("confirm failed")
		},
	}

	_, err := operations.ExecuteOperation(
		optest.NewBundle(t),
		OpFundKey,
		chain,
		OpFundKeyInput{
			Target: solana.NewWallet().PublicKey(),
			Amount: 1,
		},
	)
	require.ErrorContains(t, err, "failed to fund target")
}

func assertAnError(msg string) error {
	return &testError{msg: msg}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
