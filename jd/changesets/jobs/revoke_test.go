package jobs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
)

func TestRevokeJobsChangeset(t *testing.T) {
	t.Parallel()

	t.Run("revokes a proposed job", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		nodeID := registerTestNode(t, rt)
		jobID := proposeJob(t, rt, nodeID)

		_, err = runtime.ExecChangeset(rt, RevokeJobsChangeset{}, RevokeJobsInput{
			JobIDs: []string{jobID},
		})
		require.NoError(t, err)
	})

	t.Run("non-existent job treated as already absent — no error", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, RevokeJobsChangeset{}, RevokeJobsInput{
			JobIDs: []string{"job_nonexistent"},
		})
		require.NoError(t, err)
	})

	t.Run("precondition — empty job list rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, RevokeJobsChangeset{}, RevokeJobsInput{})
		require.ErrorContains(t, err, "no job_ids provided")
	})

	t.Run("precondition — duplicate job id rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, RevokeJobsChangeset{}, RevokeJobsInput{
			JobIDs: []string{"job_1", "job_1"},
		})
		require.ErrorContains(t, err, "duplicate job id")
	})
}
