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

func newTestEnv(t *testing.T, opts ...testenv.LoadOpt) *cldf.Environment {
	t.Helper()
	env, err := testenv.New(t.Context(), opts...)
	require.NoError(t, err)
	if env.OCRSecrets.IsEmpty() {
		env.OCRSecrets = focr.XXXGenerateTestOCRSecrets()
	}
	return env
}

func validInput(t *testing.T) operations.CREWorkflowDeployInput {
	t.Helper()
	wasmPath := filepath.Join(t.TempDir(), "x.wasm")
	require.NoError(t, os.WriteFile(wasmPath, []byte("wasm"), 0o600))
	cfgPath := filepath.Join(t.TempDir(), "c.json")
	require.NoError(t, os.WriteFile(cfgPath, []byte("{}"), 0o600))
	projectPath := filepath.Join(t.TempDir(), "project.yaml")
	require.NoError(t, os.WriteFile(projectPath, []byte("cld-deploy:\n  cre-cli:\n    don-family: zone\n"), 0o600))

	return operations.CREWorkflowDeployInput{
		WorkflowBundle: creartifacts.WorkflowBundle{
			WorkflowName:       "wf",
			Binary:             creartifacts.NewBinarySourceLocal(wasmPath),
			Config:             creartifacts.NewConfigSourceLocal(cfgPath),
			DonFamily:          "zone",
			DeploymentRegistry: "private",
		},
		Project: creartifacts.NewConfigSourceLocal(projectPath),
	}
}

func TestCREWorkflowDeployChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	mockCLI := cremocks.NewMockCLIRunner(t)
	envNoCLI := newTestEnv(t, testenv.WithCRERunner(fcre.NewRunner()))
	envWithCLI := newTestEnv(t, testenv.WithCRERunner(fcre.NewRunner(fcre.WithCLI(mockCLI))))
	envNoCRE := newTestEnv(t)

	good := validInput(t)

	tests := []struct {
		name    string
		env     cldf.Environment
		modify  func(operations.CREWorkflowDeployInput) operations.CREWorkflowDeployInput
		wantErr string
	}{
		{
			name:    "no CRERunner",
			env:     *envNoCRE,
			modify:  func(in operations.CREWorkflowDeployInput) operations.CREWorkflowDeployInput { return in },
			wantErr: "CRERunner is not available",
		},
		{
			name:    "CRERunner without CLI",
			env:     *envNoCLI,
			modify:  func(in operations.CREWorkflowDeployInput) operations.CREWorkflowDeployInput { return in },
			wantErr: "CLI runner is not configured",
		},
		{
			name: "missing project",
			env:  *envWithCLI,
			modify: func(in operations.CREWorkflowDeployInput) operations.CREWorkflowDeployInput {
				in.Project = creartifacts.ConfigSource{}
				return in
			},
			wantErr: "project:",
		},
		{
			name: "missing deploymentRegistry",
			env:  *envWithCLI,
			modify: func(in operations.CREWorkflowDeployInput) operations.CREWorkflowDeployInput {
				in.DeploymentRegistry = ""
				return in
			},
			wantErr: "deploymentRegistry is required",
		},
		{
			name: "missing donFamily",
			env:  *envWithCLI,
			modify: func(in operations.CREWorkflowDeployInput) operations.CREWorkflowDeployInput {
				in.DonFamily = ""
				return in
			},
			wantErr: "donFamily is required",
		},
		{
			name:    "valid input passes",
			env:     *envWithCLI,
			modify:  func(in operations.CREWorkflowDeployInput) operations.CREWorkflowDeployInput { return in },
			wantErr: "",
		},
	}

	cs := CREWorkflowDeployChangeset{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := cs.VerifyPreconditions(tc.env, tc.modify(good))
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestCREWorkflowDeployChangeset_Apply(t *testing.T) {
	t.Setenv("ONCHAIN_EVM_DEPLOYER_KEY", "abc123")

	cs := CREWorkflowDeployChangeset{}
	input := validInput(t)

	t.Run("success returns report", func(t *testing.T) {
		mockCLI := cremocks.NewMockCLIRunner(t)
		mockCLI.EXPECT().ContextRegistries().Return([]fcre.ContextRegistryEntry{
			{ID: "private", Type: "off-chain"},
		}).Once()
		mockCLI.EXPECT().
			Run(mock.Anything, (map[string]string)(nil), matchCLIArgs("workflow", "deploy")).
			Return(&fcre.CallResult{
				ExitCode: 0,
				Stdout:   []byte("ok"),
			}, nil).
			Once()
		env := newTestEnv(t, testenv.WithCRERunner(fcre.NewRunner(fcre.WithCLI(mockCLI))))

		out, err := cs.Apply(*env, input)
		require.NoError(t, err)
		require.Len(t, out.Reports, 1)
		output, ok := out.Reports[0].Output.(operations.CREWorkflowDeployOutput)
		require.True(t, ok)
		require.Equal(t, 0, output.ExitCode)
		require.Equal(t, "ok", output.Stdout)
	})

	t.Run("operation error returns report and error", func(t *testing.T) {
		mockCLI := cremocks.NewMockCLIRunner(t)
		mockCLI.EXPECT().ContextRegistries().Return([]fcre.ContextRegistryEntry{
			{ID: "private", Type: "off-chain"},
		}).Once()
		mockCLI.EXPECT().Run(mock.Anything, mock.Anything, mock.Anything).
			Return((*fcre.CallResult)(nil), errors.New("op failed")).
			Once()
		env := newTestEnv(t, testenv.WithCRERunner(fcre.NewRunner(fcre.WithCLI(mockCLI))))

		out, err := cs.Apply(*env, input)
		require.ErrorContains(t, err, "cre workflow deploy: op failed")
		require.Len(t, out.Reports, 1)
		output, ok := out.Reports[0].Output.(operations.CREWorkflowDeployOutput)
		require.True(t, ok)
		require.Empty(t, output.Stdout)
	})

	t.Run("on-chain registry injects deployer key env", func(t *testing.T) {
		mockCLI := cremocks.NewMockCLIRunner(t)
		mockCLI.EXPECT().ContextRegistries().Return([]fcre.ContextRegistryEntry{
			{ID: "onchain-reg", Type: "on-chain"},
		}).Once()
		mockCLI.EXPECT().
			Run(mock.Anything, mock.MatchedBy(func(env map[string]string) bool {
				return env != nil && env["CRE_ETH_PRIVATE_KEY"] == "abc123"
			}), mock.Anything).
			Return(&fcre.CallResult{ExitCode: 0}, nil).
			Once()
		env := newTestEnv(t, testenv.WithCRERunner(fcre.NewRunner(fcre.WithCLI(mockCLI))))

		onChainInput := input
		onChainInput.DeploymentRegistry = "onchain-reg"
		out, err := cs.Apply(*env, onChainInput)
		require.NoError(t, err)
		require.Len(t, out.Reports, 1)
	})
}

func matchCLIArgs(wantArgs ...string) any {
	return mock.MatchedBy(func(args []string) bool {
		if len(wantArgs) > len(args) {
			return false
		}
		for i := range wantArgs {
			if wantArgs[i] != args[i] {
				return false
			}
		}
		return true
	})
}
