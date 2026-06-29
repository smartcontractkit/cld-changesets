package evmgrantrole

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmsevm "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	legacymcms "github.com/smartcontractkit/cld-changesets/legacy/mcms/changesets"
	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
	evmreaders "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"

	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
)

func TestRunEVMGrantRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		useMCMS bool
	}{
		{name: "direct send", useMCMS: false},
		{name: "MCMS proposal", useMCMS: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			selector := chainselectors.TEST_90000001.Selector
			rt := newEVMGrantRoleRuntime(t, selector)
			refs := grantRoleRefsFromEnv(t, rt.Environment(), selector)
			chain := rt.Environment().BlockChains.EVMChains()[selector]
			grantee := common.HexToAddress("0x00000000000000000000000000000000000000aa").Hex()

			var mcmsInput *cldf.MCMSTimelockProposalInput
			if tt.useMCMS {
				mcmsInput = &cldf.MCMSTimelockProposalInput{
					TimelockAction: mcmstypes.TimelockActionBypass,
					ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
					TimelockDelay:  mcmstypes.NewDuration(0),
				}
			}

			out, err := runEVMGrantRole(
				rt.Environment().OperationsBundle,
				grantrole.Deps{
					BlockChains: rt.Environment().BlockChains,
					DataStore:   rt.Environment().DataStore,
				},
				grantrole.SeqInput{
					ChainSelector: selector,
					Grants: []grantrole.RoleGrant{{
						Role:      mcmssdk.TimelockRoleExecutor,
						Addresses: []string{grantee},
					}},
					MCMS: mcmsInput,
				},
			)
			require.NoError(t, err)

			if tt.useMCMS {
				require.Len(t, out.BatchOps, 1)
				require.Len(t, out.BatchOps[0].Transactions, 1)
				require.NoError(t, rt.Exec(
					newTimelockProposalTask(out.BatchOps, "grant role sequence test"),
					runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
				))
			} else {
				require.Empty(t, out.BatchOps)
			}

			executors, err := mcmsevm.NewTimelockInspector(chain.Client).GetExecutors(t.Context(), refs.Timelock.Hex())
			require.NoError(t, err)
			require.Contains(t, executors, grantee)
		})
	}
}

func TestRunEVMGrantRole_idempotent(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	rt := newEVMGrantRoleRuntime(t, selector)
	refs := grantRoleRefsFromEnv(t, rt.Environment(), selector)
	chain := rt.Environment().BlockChains.EVMChains()[selector]
	grantee := "0x00000000000000000000000000000000000000bb"
	deps := grantrole.Deps{
		BlockChains: rt.Environment().BlockChains,
		DataStore:   rt.Environment().DataStore,
	}
	input := grantrole.SeqInput{
		ChainSelector: selector,
		Grants: []grantrole.RoleGrant{{
			Role:      mcmssdk.TimelockRoleProposer,
			Addresses: []string{grantee},
		}},
	}

	_, err := runEVMGrantRole(rt.Environment().OperationsBundle, deps, input)
	require.NoError(t, err)

	out, err := runEVMGrantRole(rt.Environment().OperationsBundle, deps, input)
	require.NoError(t, err)
	require.Empty(t, out.BatchOps)

	proposers, err := mcmsevm.NewTimelockInspector(chain.Client).GetProposers(t.Context(), refs.Timelock.Hex())
	require.NoError(t, err)
	require.Contains(t, proposers, grantee)
}

func newEVMGrantRoleRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(legacymcms.DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	return rt
}

type evmGrantRoleRefs struct {
	Timelock common.Address
}

func grantRoleRefsFromEnv(t *testing.T, env cldf.Environment, selector uint64) evmGrantRoleRefs {
	t.Helper()

	reader := evmreaders.Reader{}
	timelock, err := reader.GetTimelockRef(env, selector, cldf.MCMSTimelockProposalInput{})
	require.NoError(t, err)

	return evmGrantRoleRefs{
		Timelock: common.HexToAddress(timelock.Address),
	}
}

type timelockProposalTask struct {
	id          string
	batchOps    []mcmstypes.BatchOperation
	description string
}

func newTimelockProposalTask(batchOps []mcmstypes.BatchOperation, description string) timelockProposalTask {
	return timelockProposalTask{
		id:          ksuid.New().String(),
		batchOps:    batchOps,
		description: description,
	}
}

func (t timelockProposalTask) ID() string {
	return t.id
}

func (t timelockProposalTask) Run(e cldf.Environment, state *runtime.State) error {
	out, err := cldf.NewOutputBuilder(e, datastore.NewMemoryDataStore()).
		WithTimelockProposal(cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionBypass,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			TimelockDelay:  mcmstypes.NewDuration(0),
			Description:    t.description,
		}, t.batchOps).
		Build()
	if err != nil {
		return err
	}

	return state.MergeChangesetOutput(t.id, out)
}
