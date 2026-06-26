package operations

import (
	"testing"

	jobv1 "github.com/smartcontractkit/chainlink-protos/job-distributor/v1/job"
	"github.com/stretchr/testify/require"

	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
)

func TestOpJDRevokeJob(t *testing.T) {
	t.Parallel()

	t.Run("revokes a proposed job", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		offchain := rt.Environment().Offchain
		deps := JDOpDeps{Offchain: offchain, EnvName: "test"}

		reg, err := fwops.ExecuteOperation(newBundle(t), OpJDRegisterNode, deps, RegisterNodeInput{
			Domain: "keystone",
			Name:   "oracle-1",
			CSAKey: "csa-key-1",
		})
		require.NoError(t, err)
		proposal, err := offchain.ProposeJob(t.Context(), &jobv1.ProposeJobRequest{
			NodeId: reg.Output.NodeID,
			Spec: `
type = "offchainreporting2"
name = "test-job"
contractID = "0x0000000000000000000000000000000000000001"
externalJobID = "00000000-0000-0000-0000-000000000001"
`,
		})
		require.NoError(t, err)
		jobID := proposal.GetProposal().GetJobId()

		report, err := fwops.ExecuteOperation(newBundle(t), OpJDRevokeJob, deps, RevokeJobInput{JobID: jobID})
		require.NoError(t, err)
		require.False(t, report.Output.AlreadyAbsent)
	})

	t.Run("job not found — AlreadyAbsent=true, no error", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		deps := JDOpDeps{Offchain: rt.Environment().Offchain, EnvName: "test"}
		report, err := fwops.ExecuteOperation(newBundle(t), OpJDRevokeJob, deps, RevokeJobInput{JobID: "job_does_not_exist"})
		require.NoError(t, err)
		require.True(t, report.Output.AlreadyAbsent, "expected not found revoke to be marked already-absent")
	})
}
