package operations_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmslib "github.com/smartcontractkit/mcms"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	mcmschangesets "github.com/smartcontractkit/cld-changesets/legacy/mcms/changesets"
	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
	evmops "github.com/smartcontractkit/cld-changesets/mcms/evm/operations"
)

func TestAddEVMCallSequenceToCSOutput_SequenceError(t *testing.T) {
	t.Parallel()

	env, err := environment.New(t.Context(),
		environment.WithEVMSimulatedN(t, 1),
	)
	require.NoError(t, err)

	csOutput := cldf.ChangesetOutput{}
	seqReport := operations.SequenceReport[string, map[uint64][]evmops.EVMCallOutput]{}
	seqErr := errors.New("sequence failed")

	result, err := evmops.AddEVMCallSequenceToCSOutput(
		*env,
		csOutput,
		seqReport,
		seqErr,
		nil,
		nil,
		"test",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to execute")
	assert.Contains(t, err.Error(), "sequence failed")
	assert.Equal(t, seqReport.ExecutionReports, result.Reports)
}

func TestAddEVMCallSequenceToCSOutput_NoMCMS(t *testing.T) {
	t.Parallel()

	env, err := environment.New(t.Context(),
		environment.WithEVMSimulatedN(t, 1),
	)
	require.NoError(t, err)

	csOutput := cldf.ChangesetOutput{}
	seqReport := operations.SequenceReport[string, map[uint64][]evmops.EVMCallOutput]{}

	result, err := evmops.AddEVMCallSequenceToCSOutput(
		*env,
		csOutput,
		seqReport,
		nil,
		nil,
		nil,
		"test",
	)

	require.NoError(t, err)
	assert.Equal(t, seqReport.ExecutionReports, result.Reports)
}

func TestAddEVMCallSequenceToCSOutput_AllConfirmed(t *testing.T) {
	t.Parallel()

	env, err := environment.New(t.Context(),
		environment.WithEVMSimulatedN(t, 1),
	)
	require.NoError(t, err)

	csOutput := cldf.ChangesetOutput{}
	seqReport := operations.SequenceReport[string, map[uint64][]evmops.EVMCallOutput]{}
	mcmsCfg := &cldfproposalutils.TimelockConfig{}

	result, err := evmops.AddEVMCallSequenceToCSOutput(
		*env,
		csOutput,
		seqReport,
		nil,
		map[uint64]evmstate.MCMSWithTimelockState{},
		mcmsCfg,
		"test",
	)

	require.NoError(t, err)
	assert.Equal(t, seqReport.ExecutionReports, result.Reports)
	assert.Nil(t, result.MCMSTimelockProposals)
}

func TestAddEVMCallSequenceToCSOutput_ProposalCombination(t *testing.T) {
	t.Parallel()

	selector1 := chain_selectors.TEST_90000001.Selector
	selector2 := chain_selectors.TEST_90000002.Selector
	selectors := []uint64{selector1, selector2}

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, selectors),
	))
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(mcmschangesets.DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector1: cldftesthelpers.SingleGroupTimelockConfig(t),
			selector2: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	mcmsStateByChain := loadEVMMCMSStateByChain(t, rt.Environment(), selectors)

	existingProposal1 := mcmslib.TimelockProposal{
		BaseProposal: mcmslib.BaseProposal{Description: "First proposal"},
		Operations: []mcmstypes.BatchOperation{{
			ChainSelector: mcmstypes.ChainSelector(selector1),
			Transactions: []mcmstypes.Transaction{{
				To:               common.HexToAddress("0x1111111111111111111111111111111111111111").String(),
				Data:             []byte("data1"),
				AdditionalFields: json.RawMessage(`{"value": 0}`),
			}},
		}},
	}

	existingProposal2 := mcmslib.TimelockProposal{
		BaseProposal: mcmslib.BaseProposal{Description: "Second proposal"},
		Operations: []mcmstypes.BatchOperation{{
			ChainSelector: mcmstypes.ChainSelector(selector2),
			Transactions: []mcmstypes.Transaction{{
				To:               common.HexToAddress("0x1111112222222222222222222222222222222222").String(),
				Data:             []byte("data2"),
				AdditionalFields: json.RawMessage(`{"value": 0}`),
			}},
		}},
	}

	csOutput := cldf.ChangesetOutput{
		MCMSTimelockProposals: []mcmslib.TimelockProposal{existingProposal1, existingProposal2},
	}

	seqReport := operations.SequenceReport[string, map[uint64][]evmops.EVMCallOutput]{
		Report: operations.Report[string, map[uint64][]evmops.EVMCallOutput]{
			Output: map[uint64][]evmops.EVMCallOutput{
				selector2: {{
					To:           common.HexToAddress("0x3333333333333333333333333333333333333333"),
					Data:         []byte("new_call_data"),
					ContractType: "TestContract",
					Confirmed:    false,
				}},
			},
		},
	}

	mcmsCfg := &cldfproposalutils.TimelockConfig{
		MinDelay:   0 * time.Second,
		MCMSAction: mcmstypes.TimelockActionSchedule,
	}

	result, err := evmops.AddEVMCallSequenceToCSOutput(
		rt.Environment(),
		csOutput,
		seqReport,
		nil,
		mcmsStateByChain,
		mcmsCfg,
		"Third proposal",
	)
	require.NoError(t, err)
	assert.Equal(t, seqReport.ExecutionReports, result.Reports)

	require.Len(t, result.MCMSTimelockProposals, 1)
	aggregatedProposal := result.MCMSTimelockProposals[0]
	assert.Equal(t, "First proposal, Second proposal, Third proposal", aggregatedProposal.Description)
	assert.NotEmpty(t, aggregatedProposal.Operations)
}

func loadEVMMCMSStateByChain(t *testing.T, env cldf.Environment, selectors []uint64) map[uint64]evmstate.MCMSWithTimelockState {
	t.Helper()

	statePtrs, err := evmstate.MaybeLoadMCMSWithTimelockState(env, selectors)
	require.NoError(t, err)

	out := make(map[uint64]evmstate.MCMSWithTimelockState, len(statePtrs))
	for sel, st := range statePtrs {
		require.NotNilf(t, st, "expected non-nil MCMS state for chain %d", sel)
		require.NoErrorf(t, st.Validate(), "MCMS state for chain %d failed validation", sel)
		out[sel] = *st
	}

	return out
}
