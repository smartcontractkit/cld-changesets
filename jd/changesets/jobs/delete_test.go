package jobs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
)

func TestDeleteJobsChangeset(t *testing.T) {
	t.Parallel()

	t.Run("deletes eligible jobs", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		nodeID := registerTestNode(t, rt)
		jobID := proposeJob(t, rt, nodeID)

		_, err = runtime.ExecChangeset(rt, DeleteJobsChangeset{}, DeleteJobsInput{
			JobIDs: []string{jobID},
		})
		require.NoError(t, err)
	})

	t.Run("precondition — empty job list rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, DeleteJobsChangeset{}, DeleteJobsInput{})
		require.ErrorContains(t, err, "no job_ids provided")
	})

	t.Run("precondition — duplicate job id rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, DeleteJobsChangeset{}, DeleteJobsInput{
			JobIDs: []string{"job_1", "job_1"},
		})
		require.ErrorContains(t, err, "duplicate job id")
	})

	t.Run("precondition — non-existent job rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, DeleteJobsChangeset{}, DeleteJobsInput{
			JobIDs: []string{"job_does_not_exist"},
		})
		require.Error(t, err)
	})

	t.Run("precondition — already-deleted job rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		nodeID := registerTestNode(t, rt)
		jobID := proposeJob(t, rt, nodeID)

		_, err = runtime.ExecChangeset(rt, DeleteJobsChangeset{}, DeleteJobsInput{
			JobIDs: []string{jobID},
		})
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, DeleteJobsChangeset{}, DeleteJobsInput{
			JobIDs: []string{jobID},
		})
		require.ErrorContains(t, err, "already deleted")
	})
}
