package changesets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/cre/operations"

	fcre "github.com/smartcontractkit/chainlink-deployments-framework/cre"
	creartifacts "github.com/smartcontractkit/chainlink-deployments-framework/cre/artifacts"
	cremocks "github.com/smartcontractkit/chainlink-deployments-framework/cre/mocks"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	testenv "github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	focr "github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
)

func newDeleteTestEnv(t *testing.T, opts ...testenv.LoadOpt) *cldf.Environment {
	t.Helper()
	env, err := testenv.New(t.Context(), opts...)
	require.NoError(t, err)
	if env.OCRSecrets.IsEmpty() {
		env.OCRSecrets = focr.XXXGenerateTestOCRSecrets()
	}

	return env
}

func validDeleteInput(t *testing.T) operations.CREWorkflowDeleteInput {
	t.Helper()
	projectPath := filepath.Join(t.TempDir(), "project.yaml")
	require.NoError(t, os.WriteFile(projectPath, []byte("cld-deploy:\n  rpcs: []\n"), 0o600))

	return operations.CREWorkflowDeleteInput{
		WorkflowName:       "wf",
		Project:            creartifacts.NewConfigSourceLocal(projectPath),
		DonFamily:          "zone",
		DeploymentRegistry: "private",
	}
}

func TestCREWorkflowDeleteChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	mockCLI := cremocks.NewMockCLIRunner(t)
	envNoCLI := newDeleteTestEnv(t, testenv.WithCRERunner(fcre.NewRunner()))
	envWithCLI := newDeleteTestEnv(t, testenv.WithCRERunner(fcre.NewRunner(fcre.WithCLI(mockCLI))))
	envNoCRE := newDeleteTestEnv(t)

	good := validDeleteInput(t)

	tests := []struct {
		name    string
		env     cldf.Environment
		input   func() operations.CREWorkflowDeleteInput
		wantErr string
	}{
		{name: "no CRERunner", env: *envNoCRE, wantErr: "cre runner is not available in this environment"},
		{name: "CRERunner without CLI", env: *envNoCLI, wantErr: "CLI runner is not configured"},
		{
			name: "missing project",
			env:  *envWithCLI,
			input: func() operations.CREWorkflowDeleteInput {
				in := good
				in.Project = creartifacts.ConfigSource{}

				return in
			},
			wantErr: "project:",
		},
		{
			name: "missing deploymentRegistry",
			env:  *envWithCLI,
			input: func() operations.CREWorkflowDeleteInput {
				in := good
				in.DeploymentRegistry = ""

				return in
			},
			wantErr: "deploymentRegistry is required",
		},
		{
			name: "missing donFamily",
			env:  *envWithCLI,
			input: func() operations.CREWorkflowDeleteInput {
				in := good
				in.DonFamily = ""

				return in
			},
			wantErr: "donFamily is required",
		},
		{name: "valid input passes", env: *envWithCLI},
	}

	cs := CREWorkflowDeleteChangeset{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := good
			if tc.input != nil {
				input = tc.input()
			}
			err := cs.VerifyPreconditions(tc.env, input)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestCREWorkflowDeleteChangeset_Apply(t *testing.T) {
	t.Parallel()
	cs := CREWorkflowDeleteChangeset{}
	input := validDeleteInput(t)

	t.Run("success returns report", func(t *testing.T) {
		t.Parallel()
		mockCLI := cremocks.NewMockCLIRunner(t)
		mockCLI.EXPECT().ContextRegistries().Return([]fcre.ContextRegistryEntry{{ID: "private", Type: "off-chain"}}).Once()
		mockCLI.EXPECT().
			Run(mock.Anything, (map[string]string)(nil), matchCLIArgs("workflow", "delete")).
			Return(&fcre.CallResult{ExitCode: 0, Stdout: []byte("ok")}, nil).
			Once()
		env := newDeleteTestEnv(t, testenv.WithCRERunner(fcre.NewRunner(fcre.WithCLI(mockCLI))))

		out, err := cs.Apply(*env, input)
		require.NoError(t, err)
		require.Len(t, out.Reports, 1)
		output, ok := out.Reports[0].Output.(operations.CREWorkflowDeleteOutput)
		require.True(t, ok)
		require.Equal(t, 0, output.ExitCode)
		require.Equal(t, "ok", output.Stdout)
	})

	t.Run("operation error returns report and error", func(t *testing.T) {
		t.Parallel()
		mockCLI := cremocks.NewMockCLIRunner(t)
		mockCLI.EXPECT().ContextRegistries().Return([]fcre.ContextRegistryEntry{{ID: "private", Type: "off-chain"}}).Once()
		mockCLI.EXPECT().Run(mock.Anything, mock.Anything, mock.Anything).
			Return((*fcre.CallResult)(nil), errors.New("op failed")).
			Once()
		env := newDeleteTestEnv(t, testenv.WithCRERunner(fcre.NewRunner(fcre.WithCLI(mockCLI))))

		out, err := cs.Apply(*env, input)
		require.ErrorContains(t, err, "cre workflow delete: op failed")
		require.Len(t, out.Reports, 1)
		output, ok := out.Reports[0].Output.(operations.CREWorkflowDeleteOutput)
		require.True(t, ok)
		require.Empty(t, output.Stdout)
	})
}
