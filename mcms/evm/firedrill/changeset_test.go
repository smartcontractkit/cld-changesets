package evmfiredrill_test

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

	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	firedrill "github.com/smartcontractkit/cld-changesets/mcms/changesets/firedrill"

	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/firedrill"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
)

func TestChangeset_Apply_evmProposal(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	rt := newEVMFireDrillRuntime(t, selector)

	err := rt.Exec(runtime.ChangesetTask(firedrill.Changeset{}, firedrill.Input{
		MCMS: &cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			TimelockDelay:  mcmstypes.NewDuration(time.Second),
			Description:    "firedrill integration test",
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

	require.Equal(t, "firedrill integration test", proposal.Description)
	require.Len(t, proposal.Operations, 1)
	require.Equal(t, mcmstypes.ChainSelector(selector), proposal.Operations[0].ChainSelector)
	require.Len(t, proposal.Operations[0].Transactions, 1)
	require.Equal(t, "FireDrillNoop", proposal.Operations[0].Transactions[0].ContractType)
}

func newEVMFireDrillRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
			ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
				selector: cldftesthelpers.SingleGroupTimelockConfig(t),
			},
		}),
	)
	require.NoError(t, err)

	return rt
}
