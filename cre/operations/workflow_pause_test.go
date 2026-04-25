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

func TestCREWorkflowPauseOp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    func(t *testing.T) CREWorkflowPauseInput
		setupCLI func(t *testing.T) *cremocks.MockCLIRunner
		assert   func(t *testing.T, out fwops.Report[CREWorkflowPauseInput, CREWorkflowPauseOutput], err error)
	}{
		{
			name: "success invokes CLI with pause args",
			input: func(t *testing.T) CREWorkflowPauseInput {
				t.Helper()

				return CREWorkflowPauseInput{
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
				m.EXPECT().Run(mock.Anything, (map[string]string)(nil), matchCLIArgs("workflow", "pause")).Return(
					&fcre.CallResult{ExitCode: 0, Stdout: []byte("paused")}, nil,
				).Once()

				return m
			},
			assert: func(t *testing.T, out fwops.Report[CREWorkflowPauseInput, CREWorkflowPauseOutput], err error) {
				t.Helper()
				require.NoError(t, err)
				require.Equal(t, 0, out.Output.ExitCode)
				require.Equal(t, "paused", out.Output.Stdout)
			},
		},
		{
			name: "missing project returns resolve error",
			input: func(t *testing.T) CREWorkflowPauseInput {
				t.Helper()
				missingProjectPath := filepath.Join(t.TempDir(), "project.yaml")

				return CREWorkflowPauseInput{
					WorkflowName:       "wf",
					Project:            creartifacts.NewConfigSourceLocal(missingProjectPath),
					DonFamily:          "feeds-zone-a",
					DeploymentRegistry: "private",
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()

				return cremocks.NewMockCLIRunner(t)
			},
			assert: func(t *testing.T, _ fwops.Report[CREWorkflowPauseInput, CREWorkflowPauseOutput], err error) {
				t.Helper()
				require.ErrorContains(t, err, "resolve project.yaml")
			},
		},
		{
			name: "CLI exit error propagates exit code and output",
			input: func(t *testing.T) CREWorkflowPauseInput {
				t.Helper()

				return CREWorkflowPauseInput{
					WorkflowName:       "wf",
					Project:            creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("cld-deploy:\n  rpcs: []\n"))),
					DonFamily:          "feeds-zone-a",
					DeploymentRegistry: "private",
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()
				exitErr := &fcre.ExitError{ExitCode: 9, Stdout: []byte("out"), Stderr: []byte("err")}
				m := cremocks.NewMockCLIRunner(t)
				m.EXPECT().ContextRegistries().Return(testRegistries()).Once()
				m.EXPECT().Run(mock.Anything, mock.Anything, mock.Anything).Return(
					&fcre.CallResult{ExitCode: 9, Stdout: exitErr.Stdout, Stderr: exitErr.Stderr}, exitErr,
				).Once()

				return m
			},
			assert: func(t *testing.T, out fwops.Report[CREWorkflowPauseInput, CREWorkflowPauseOutput], err error) {
				t.Helper()
				require.ErrorContains(t, err, "cre workflow pause")
				require.Equal(t, 9, out.Output.ExitCode)
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

			out, err := fwops.ExecuteOperation(bundle, CREWorkflowPauseOp, deps, tc.input(t))
			tc.assert(t, out, err)
		})
	}
}

func TestResolvePauseTargetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  CREWorkflowPauseInput
		expect string
	}{
		{name: "empty defaults", input: CREWorkflowPauseInput{}, expect: CREDeployTargetName},
		{name: "whitespace defaults", input: CREWorkflowPauseInput{TargetName: "   "}, expect: CREDeployTargetName},
		{name: "custom target returned trimmed", input: CREWorkflowPauseInput{TargetName: " staging-settings "}, expect: "staging-settings"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expect, tc.input.resolveTargetName())
		})
	}
}

func TestBuildWorkflowPauseArgs(t *testing.T) {
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
					"workflow", "pause", filepath.Join(workDir, creBundleSubdir),
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
			tc.check(t, BuildWorkflowPauseArgs(tc.targetName, workDir, tc.envPath, tc.extra))
		})
	}
}

func TestPauseInputValidate(t *testing.T) {
	t.Parallel()

	validProject := creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("staging-settings:\n  rpcs: []\n")))
	in := CREWorkflowPauseInput{
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

	bad := CREWorkflowPauseInput{}
	require.ErrorContains(t, bad.Validate(), "workflowName is required")

	bad = CREWorkflowPauseInput{WorkflowName: "wf", Project: validProject}
	require.ErrorContains(t, bad.Validate(), "deploymentRegistry is required")

	bad = CREWorkflowPauseInput{WorkflowName: "wf", Project: validProject, DeploymentRegistry: "private"}
	require.ErrorContains(t, bad.Validate(), "donFamily is required")

	bad = CREWorkflowPauseInput{WorkflowName: "wf", DonFamily: "zone-a", DeploymentRegistry: "private"}
	require.ErrorContains(t, bad.Validate(), "project:")
}
