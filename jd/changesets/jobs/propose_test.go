package jobs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
)

func TestProposeJobsChangeset(t *testing.T) {
	t.Parallel()

	t.Run("proposes job from inline TOML", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		registerTestNode(t, rt)

		_, err = runtime.ExecChangeset(rt, ProposeJobsChangeset{}, ProposeJobsInput{
			Domain: "keystone",
			Jobs: []JobSpec{{
				NodeLabels:  map[string]string{"target": "oracle-1"},
				JobspecTOML: testJobTOML,
			}},
		})
		require.NoError(t, err)
	})

	t.Run("precondition — missing domain rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, ProposeJobsChangeset{}, ProposeJobsInput{
			Jobs: []JobSpec{{NodeLabels: map[string]string{"k": "v"}, JobspecTOML: testJobTOML}},
		})
		require.ErrorContains(t, err, "domain is required")
	})

	t.Run("precondition — missing node_labels rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, ProposeJobsChangeset{}, ProposeJobsInput{
			Domain: "keystone",
			Jobs:   []JobSpec{{JobspecTOML: testJobTOML}},
		})
		require.ErrorContains(t, err, "node_labels is required")
	})

	t.Run("precondition — missing jobspec rejected", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		_, err = runtime.ExecChangeset(rt, ProposeJobsChangeset{}, ProposeJobsInput{
			Domain: "keystone",
			Jobs:   []JobSpec{{NodeLabels: map[string]string{"k": "v"}}},
		})
		require.ErrorContains(t, err, "jobspec_toml is required")
	})
}
