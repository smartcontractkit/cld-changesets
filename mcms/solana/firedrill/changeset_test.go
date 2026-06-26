package solfiredrill_test

import (
	"testing"
	"time"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/smartcontractkit/mcms"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	legacymcms "github.com/smartcontractkit/cld-changesets/legacy/mcms/changesets"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	firedrill "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill"

	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/firedrill"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func TestChangeset_Apply_solanaProposal(t *testing.T) {
	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	rt := newSolanaFireDrillRuntime(t, selector)

	err := rt.Exec(runtime.ChangesetTask(firedrill.Changeset{}, firedrill.Input{
		MCMS: &cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			TimelockDelay:  mcmstypes.NewDuration(time.Second),
			Description:    "firedrill solana integration test",
		},
		Cfg: firedrill.Config{Selectors: []uint64{selector}},
	}))
	require.NoError(t, err)

	var proposal mcms.TimelockProposal
	var foundProposal bool
	for _, out := range rt.State().Outputs {
		if len(out.MCMSTimelockProposals) > 0 {
			proposal = out.MCMSTimelockProposals[0]
			foundProposal = true
		}
	}
	require.True(t, foundProposal, "expected one MCMS timelock proposal")

	require.Equal(t, "firedrill solana integration test", proposal.Description)
	require.Len(t, proposal.Operations, 1)
	require.Equal(t, mcmstypes.ChainSelector(selector), proposal.Operations[0].ChainSelector)
	require.Len(t, proposal.Operations[0].Transactions, 1)
	require.Equal(t, "Memo", proposal.Operations[0].Transactions[0].ContractType)
}

func newSolanaFireDrillRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	programsPath, programIDs, ab := soltestutils.PreloadMCMS(t, selector)
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{selector}, programsPath, programIDs),
		environment.WithAddressBook(ab),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)
	require.Contains(t, rt.Environment().BlockChains.SolanaChains(), selector)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(legacymcms.DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	return rt
}
