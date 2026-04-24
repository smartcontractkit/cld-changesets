package operations

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	fcre "github.com/smartcontractkit/chainlink-deployments-framework/cre"
	creartifacts "github.com/smartcontractkit/chainlink-deployments-framework/cre/artifacts"
	cremocks "github.com/smartcontractkit/chainlink-deployments-framework/cre/mocks"
	cfgenv "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/config/env"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
)

func TestCREWorkflowDeleteOp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    func(t *testing.T) CREWorkflowDeleteInput
		setupCLI func(t *testing.T) *cremocks.MockCLIRunner
		assert   func(t *testing.T, out fwops.Report[CREWorkflowDeleteInput, CREWorkflowDeleteOutput], err error)
	}{
		{
			name: "success invokes CLI with delete args",
			input: func(t *testing.T) CREWorkflowDeleteInput {
				t.Helper()

				return CREWorkflowDeleteInput{
					WorkflowName:       "wf",
					Project:            creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("cld-deploy:\n  rpcs: []\n"))),
					DonFamily:          "feeds-zone-a",
					DeploymentRegistry: "private",
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()
				m := cremocks.NewMockCLIRunner(t)
				m.EXPECT().ContextRegistries().Return(testRegistries()).Once()
				m.EXPECT().Run(mock.Anything, (map[string]string)(nil), matchCLIArgs("workflow", "delete")).Return(
					&fcre.CallResult{ExitCode: 0, Stdout: []byte("deleted")}, nil,
				).Once()

				return m
			},
			assert: func(t *testing.T, out fwops.Report[CREWorkflowDeleteInput, CREWorkflowDeleteOutput], err error) {
				t.Helper()
				require.NoError(t, err)
				require.Equal(t, 0, out.Output.ExitCode)
				require.Equal(t, "deleted", out.Output.Stdout)
			},
		},
		{
			name: "missing project returns resolve error",
			input: func(t *testing.T) CREWorkflowDeleteInput {
				t.Helper()

				return CREWorkflowDeleteInput{
					WorkflowName:       "wf",
					Project:            creartifacts.NewConfigSourceLocal("/tmp/definitely-missing-project.yaml"),
					DonFamily:          "feeds-zone-a",
					DeploymentRegistry: "private",
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()

				return cremocks.NewMockCLIRunner(t)
			},
			assert: func(t *testing.T, _ fwops.Report[CREWorkflowDeleteInput, CREWorkflowDeleteOutput], err error) {
				t.Helper()
				require.ErrorContains(t, err, "resolve project.yaml")
			},
		},
		{
			name: "CLI exit error propagates exit code and output",
			input: func(t *testing.T) CREWorkflowDeleteInput {
				t.Helper()

				return CREWorkflowDeleteInput{
					WorkflowName:       "wf",
					Project:            creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("cld-deploy:\n  rpcs: []\n"))),
					DonFamily:          "feeds-zone-a",
					DeploymentRegistry: "private",
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()
				exitErr := &fcre.ExitError{ExitCode: 11, Stdout: []byte("out"), Stderr: []byte("err")}
				m := cremocks.NewMockCLIRunner(t)
				m.EXPECT().ContextRegistries().Return(testRegistries()).Once()
				m.EXPECT().Run(mock.Anything, mock.Anything, mock.Anything).Return(
					&fcre.CallResult{ExitCode: 11, Stdout: exitErr.Stdout, Stderr: exitErr.Stderr}, exitErr,
				).Once()

				return m
			},
			assert: func(t *testing.T, out fwops.Report[CREWorkflowDeleteInput, CREWorkflowDeleteOutput], err error) {
				t.Helper()
				require.ErrorContains(t, err, "cre workflow delete")
				require.Equal(t, 11, out.Output.ExitCode)
				require.Equal(t, "out", out.Output.Stdout)
				require.Equal(t, "err", out.Output.Stderr)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockCLI := tc.setupCLI(t)
			bundle := fwops.NewBundle(func() context.Context { return t.Context() }, logger.Test(t), fwops.NewMemoryReporter())
			deps := CREDeployDeps{
				CLI:    mockCLI,
				CRECfg: cfgenv.CREConfig{},
			}

			out, err := fwops.ExecuteOperation(bundle, CREWorkflowDeleteOp, deps, tc.input(t))
			tc.assert(t, out, err)
		})
	}
}

func TestResolveDeleteTargetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  CREWorkflowDeleteInput
		expect string
	}{
		{name: "empty defaults", input: CREWorkflowDeleteInput{}, expect: CREDeployTargetName},
		{name: "whitespace defaults", input: CREWorkflowDeleteInput{TargetName: "   "}, expect: CREDeployTargetName},
		{name: "custom target returned trimmed", input: CREWorkflowDeleteInput{TargetName: " staging-settings "}, expect: "staging-settings"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expect, tc.input.resolveTargetName())
		})
	}
}

func TestBuildWorkflowDeleteArgs(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()

	tests := []struct {
		name       string
		targetName string
		envPath    string
		extra      []string
		check      func(t *testing.T, args []string)
	}{
		{
			name:       "with env and extra args",
			targetName: "staging-settings",
			envPath:    filepath.Join(workDir, ".env"),
			extra:      []string{"--extra"},
			check: func(t *testing.T, args []string) {
				t.Helper()
				require.Equal(t, []string{
					"workflow", "delete", filepath.Join(workDir, creBundleSubdir),
					"-R", workDir, "-T", "staging-settings",
					"--yes",
					"-e", filepath.Join(workDir, ".env"),
					"--extra",
				}, args)
			},
		},
		{
			name:       "without env",
			targetName: CREDeployTargetName,
			check: func(t *testing.T, args []string) {
				t.Helper()
				require.NotContains(t, args, "-e")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.check(t, BuildWorkflowDeleteArgs(tc.targetName, workDir, tc.envPath, tc.extra))
		})
	}
}

func TestDeleteInputValidate(t *testing.T) {
	t.Parallel()

	validProject := creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("staging-settings:\n  rpcs: []\n")))
	in := CREWorkflowDeleteInput{
		WorkflowName:       " wf ",
		Project:            validProject,
		DonFamily:          " zone-a ",
		DeploymentRegistry: " private ",
		TargetName:         " staging-settings ",
	}
	require.NoError(t, in.Validate())
	require.Equal(t, "wf", in.WorkflowName)
	require.Equal(t, "zone-a", in.DonFamily)
	require.Equal(t, "private", in.DeploymentRegistry)
	require.Equal(t, "staging-settings", in.TargetName)

	bad := CREWorkflowDeleteInput{}
	require.ErrorContains(t, bad.Validate(), "workflowName is required")

	bad = CREWorkflowDeleteInput{WorkflowName: "wf", Project: validProject}
	require.ErrorContains(t, bad.Validate(), "deploymentRegistry is required")

	bad = CREWorkflowDeleteInput{WorkflowName: "wf", Project: validProject, DeploymentRegistry: "private"}
	require.ErrorContains(t, bad.Validate(), "donFamily is required")

	bad = CREWorkflowDeleteInput{WorkflowName: "wf", DonFamily: "zone-a", DeploymentRegistry: "private"}
	require.ErrorContains(t, bad.Validate(), "project:")
}
