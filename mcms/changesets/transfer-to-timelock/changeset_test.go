package transfertotimelock_test

import (
	"context"
	"testing"
	"time"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
)

func testEnvironment(t *testing.T, ds datastore.DataStore) cldf.Environment {
	t.Helper()

	return *cldf.NewEnvironment(
		"test",
		logger.Test(t),
		nil,
		ds,
		nil,
		nil,
		func() context.Context { return t.Context() },
		ocr.OCRSecrets{},
		cldf_chain.NewBlockChains(nil),
	)
}

func testMCMSInput() *cldf.MCMSTimelockProposalInput {
	return &cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
		Description:    "transfer-to-timelock test",
	}
}

func testContractRef(selector uint64) refkey.RefKey {
	return refkey.New(selector, "LinkToken", &semvers.V1_0_0, "")
}

func TestChangeset_VerifyPreconditions_NoDatastore(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, nil)
	input := transfertotimelock.Input{
		MCMS: testMCMSInput(),
		Cfg: transfertotimelock.Config{
			ContractsByChain: map[uint64][]refkey.RefKey{
				chainselectors.TEST_90000001.Selector: {testContractRef(chainselectors.TEST_90000001.Selector)},
			},
		},
	}

	err := transfertotimelock.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "datastore is required")
}

func TestChangeset_VerifyPreconditions_NoMCMSInput(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal())
	input := transfertotimelock.Input{
		Cfg: transfertotimelock.Config{
			ContractsByChain: map[uint64][]refkey.RefKey{
				chainselectors.TEST_90000001.Selector: {testContractRef(chainselectors.TEST_90000001.Selector)},
			},
		},
	}

	err := transfertotimelock.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "MCMS timelock proposal input is required")
}

func TestChangeset_VerifyPreconditions_NoContracts(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal())
	input := transfertotimelock.Input{
		MCMS: testMCMSInput(),
	}

	err := transfertotimelock.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "no contracts provided")
}

func TestChangeset_VerifyPreconditions_EmptyContractsForChain(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal())
	input := transfertotimelock.Input{
		MCMS: testMCMSInput(),
		Cfg: transfertotimelock.Config{
			ContractsByChain: map[uint64][]refkey.RefKey{
				selector: {},
			},
		},
	}

	err := transfertotimelock.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "no contracts provided")
}

func TestChangeset_VerifyPreconditions_UnsupportedChainFamily(t *testing.T) {
	t.Parallel()

	selector := chainselectors.APTOS_MAINNET.Selector
	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal())
	input := transfertotimelock.Input{
		MCMS: testMCMSInput(),
		Cfg: transfertotimelock.Config{
			ContractsByChain: map[uint64][]refkey.RefKey{
				selector: {testContractRef(selector)},
			},
		},
	}

	err := transfertotimelock.Changeset{}.VerifyPreconditions(env, input)
	require.ErrorContains(t, err, "no sequence registered for family")
}

func TestChangeset_Apply_NoMCMSInput(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal())
	input := transfertotimelock.Input{
		Cfg: transfertotimelock.Config{
			ContractsByChain: map[uint64][]refkey.RefKey{
				chainselectors.TEST_90000001.Selector: {testContractRef(chainselectors.TEST_90000001.Selector)},
			},
		},
	}

	_, err := transfertotimelock.Changeset{}.Apply(env, input)
	require.ErrorContains(t, err, "MCMS timelock proposal input is required")
}
