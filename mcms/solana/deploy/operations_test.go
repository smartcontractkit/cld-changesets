package soldeploy

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	timelockBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/timelock"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	solutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

func testPubkey(n byte) solanago.PublicKey {
	var b [32]byte
	b[31] = n

	return solanago.PublicKeyFromBytes(b[:])
}

func testSeed(label string) legacysolana.PDASeed {
	var seed legacysolana.PDASeed
	copy(seed[:], label)

	return seed
}

func TestTimelockMinDelayUint64(t *testing.T) {
	t.Parallel()

	t.Run("nil is zero", func(t *testing.T) {
		t.Parallel()
		got, err := timelockMinDelayUint64(nil)
		require.NoError(t, err)
		require.Equal(t, uint64(0), got)
	})

	t.Run("valid delay", func(t *testing.T) {
		t.Parallel()
		got, err := timelockMinDelayUint64(big.NewInt(42))
		require.NoError(t, err)
		require.Equal(t, uint64(42), got)
	})

	t.Run("negative rejected", func(t *testing.T) {
		t.Parallel()
		_, err := timelockMinDelayUint64(big.NewInt(-1))
		require.Error(t, err)
	})

	t.Run("overflow rejected", func(t *testing.T) {
		t.Parallel()
		_, err := timelockMinDelayUint64(new(big.Int).Lsh(big.NewInt(1), 65))
		require.Error(t, err)
	})
}

func TestAddressRef(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	ref := addressRef(selector, mcmscontracts.RBACTimelock, "addr", "prod", "label")

	require.Equal(t, "addr", ref.Address)
	require.Equal(t, selector, ref.ChainSelector)
	require.Equal(t, cldfdatastore.ContractType(mcmscontracts.RBACTimelock), ref.Type)
	require.True(t, semvers.V1_0_0.Equal(ref.Version))
	require.Equal(t, "prod", ref.Qualifier)
	require.Equal(t, []string{"label"}, ref.Labels.List())
}

func TestTimelockRoleGrants(t *testing.T) {
	t.Parallel()

	mcmProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgMCM))
	timelockProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgTimelock))
	acProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgAccessController))
	deployer := solanago.MustPublicKeyFromBase58("11111111111111111111111111111112")

	proposerSeed := testSeed("proposer-seed-123456789012345678")
	cancellerSeed := testSeed("canceller-seed-12345678901234567")
	bypasserSeed := testSeed("bypasser-seed-123456789012345678")

	proposerAC := testPubkey(1)
	executorAC := testPubkey(2)
	cancellerAC := testPubkey(3)
	bypasserAC := testPubkey(4)

	in := setupTimelockRolesInput{
		McmProgram:              mcmProgram,
		ProposerMCMSeed:         proposerSeed,
		CancellerMCMSeed:        cancellerSeed,
		BypasserMCMSeed:         bypasserSeed,
		TimelockProgram:         timelockProgram,
		TimelockSeed:            proposerSeed,
		AccessControllerProgram: acProgram,
		ProposerAC:              proposerAC,
		ExecutorAC:              executorAC,
		CancellerAC:             cancellerAC,
		BypasserAC:              bypasserAC,
	}

	grants := timelockRoleGrants(in, deployer)
	require.Len(t, grants, 4)

	proposerPDA := familysolana.GetMCMSignerPDA(mcmProgram, proposerSeed)
	cancellerPDA := familysolana.GetMCMSignerPDA(mcmProgram, cancellerSeed)
	bypasserPDA := familysolana.GetMCMSignerPDA(mcmProgram, bypasserSeed)

	require.Equal(t, timelockBindings.Proposer_Role, grants[0].role)
	require.Equal(t, []solanago.PublicKey{proposerPDA}, grants[0].accounts)
	require.Equal(t, proposerAC, grants[0].acAccount)

	require.Equal(t, timelockBindings.Executor_Role, grants[1].role)
	require.Equal(t, []solanago.PublicKey{deployer}, grants[1].accounts)
	require.Equal(t, executorAC, grants[1].acAccount)

	require.Equal(t, timelockBindings.Canceller_Role, grants[2].role)
	require.Equal(t, []solanago.PublicKey{cancellerPDA, proposerPDA, bypasserPDA}, grants[2].accounts)
	require.Equal(t, cancellerAC, grants[2].acAccount)

	require.Equal(t, timelockBindings.Bypasser_Role, grants[3].role)
	require.Equal(t, []solanago.PublicKey{bypasserPDA}, grants[3].accounts)
	require.Equal(t, bypasserAC, grants[3].acAccount)
}

//nolint:paralleltest // timelockBindings.SetProgramID is global in-process
func TestBuildTimelockBatchAddAccessInstruction(t *testing.T) {
	mcmProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgMCM))
	timelockProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgTimelock))
	acProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgAccessController))
	admin := solanago.MustPublicKeyFromBase58("11111111111111111111111111111112")
	proposerAC := testPubkey(1)

	timelockSeed := testSeed("timelock-seed-123456789012345678")
	proposerSeed := testSeed("proposer-seed-123456789012345678")

	in := setupTimelockRolesInput{
		McmProgram:              mcmProgram,
		ProposerMCMSeed:         proposerSeed,
		TimelockProgram:         timelockProgram,
		TimelockSeed:            timelockSeed,
		AccessControllerProgram: acProgram,
		ProposerAC:              proposerAC,
	}

	grant := timelockRoleGrants(in, admin)[0]
	ix, err := buildTimelockBatchAddAccessInstruction(in, grant, admin)
	require.NoError(t, err)
	require.Equal(t, timelockProgram, ix.ProgramID())

	accounts := ix.Accounts()
	require.GreaterOrEqual(t, len(accounts), 5)
	require.Equal(t, familysolana.GetTimelockConfigPDA(timelockProgram, timelockSeed), accounts[0].PublicKey)
	require.Equal(t, acProgram, accounts[1].PublicKey)
	require.Equal(t, proposerAC, accounts[2].PublicKey)
	require.Equal(t, admin, accounts[3].PublicKey)
	require.Equal(t, familysolana.GetMCMSignerPDA(mcmProgram, proposerSeed), accounts[4].PublicKey)
}

//nolint:paralleltest // accessControllerBindings.SetProgramID is global in-process
func TestBuildAccessControllerInitInstructions(t *testing.T) {
	programID := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgAccessController))
	payer := solanago.MustPublicKeyFromBase58("11111111111111111111111111111112")
	account, err := solanago.NewRandomPrivateKey()
	require.NoError(t, err)

	instructions, err := buildAccessControllerInitInstructions(programID, payer, account, 1_000_000)
	require.NoError(t, err)
	require.Len(t, instructions, 2)

	createIx := instructions[0]
	require.Equal(t, system.ProgramID, createIx.ProgramID())
	require.Equal(t, payer, createIx.Accounts()[0].PublicKey)
	require.Equal(t, account.PublicKey(), createIx.Accounts()[1].PublicKey)
	require.True(t, createIx.Accounts()[1].IsSigner)

	initIx := instructions[1]
	require.Equal(t, programID, initIx.ProgramID())
	require.Equal(t, account.PublicKey(), initIx.Accounts()[0].PublicKey)
	require.Equal(t, payer, initIx.Accounts()[1].PublicKey)
	require.True(t, initIx.Accounts()[1].IsSigner)
}

func TestWaitForProgramReadyWith(t *testing.T) {
	t.Parallel()

	programID := testPubkey(9)
	const (
		testPollInterval = 10 * time.Millisecond
		testTimeout      = 50 * time.Millisecond
	)

	t.Run("success when executable", func(t *testing.T) {
		t.Parallel()

		err := waitForProgramReadyWith(t.Context(), programID, func(context.Context, solanago.PublicKey) (*rpc.GetAccountInfoResult, error) {
			return &rpc.GetAccountInfoResult{Value: &rpc.Account{Executable: true}}, nil
		}, testPollInterval, testTimeout)
		require.NoError(t, err)
	})

	t.Run("success after transient RPC errors", func(t *testing.T) {
		t.Parallel()

		calls := 0
		err := waitForProgramReadyWith(t.Context(), programID, func(context.Context, solanago.PublicKey) (*rpc.GetAccountInfoResult, error) {
			calls++
			if calls < 3 {
				return nil, errors.New("rpc unavailable")
			}

			return &rpc.GetAccountInfoResult{Value: &rpc.Account{Executable: true}}, nil
		}, testPollInterval, 200*time.Millisecond)
		require.NoError(t, err)
		require.GreaterOrEqual(t, calls, 3)
	})

	t.Run("timeout wraps last RPC error", func(t *testing.T) {
		t.Parallel()

		rpcErr := errors.New("rpc unavailable")
		err := waitForProgramReadyWith(t.Context(), programID, func(context.Context, solanago.PublicKey) (*rpc.GetAccountInfoResult, error) {
			return nil, rpcErr
		}, testPollInterval, testTimeout)
		require.Error(t, err)
		require.ErrorContains(t, err, "timed out waiting for program")
		require.ErrorIs(t, err, rpcErr)
	})

	t.Run("timeout when account never becomes executable", func(t *testing.T) {
		t.Parallel()

		err := waitForProgramReadyWith(t.Context(), programID, func(context.Context, solanago.PublicKey) (*rpc.GetAccountInfoResult, error) {
			return &rpc.GetAccountInfoResult{Value: &rpc.Account{Executable: false}}, nil
		}, testPollInterval, testTimeout)
		require.Error(t, err)
		require.EqualError(t, err, fmt.Sprintf("timed out waiting for program %s to be executable", programID))
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		errCh := make(chan error, 1)
		go func() {
			errCh <- waitForProgramReadyWith(ctx, programID, func(context.Context, solanago.PublicKey) (*rpc.GetAccountInfoResult, error) {
				return &rpc.GetAccountInfoResult{Value: &rpc.Account{Executable: false}}, nil
			}, testPollInterval, time.Second)
		}()

		cancel()

		select {
		case err := <-errCh:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for waitForProgramReadyWith to return")
		}
	})
}
