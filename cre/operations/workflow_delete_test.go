package operations

import (
	"context"
	"os"
	"path/filepath"
	"slices"
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
					DonFamily:          "feeds-zone-a",
					DeploymentRegistry: "private",
					Project:            creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("cld-deploy:\n  cre-cli:\n    don-family: feeds-zone-a\n"))),
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()
				m := cremocks.NewMockCLIRunner(t)
				m.EXPECT().ContextRegistries().Return(testRegistries()).Once()
				m.EXPECT().Run(mock.Anything, mock.Anything, matchCLIArgs("workflow", "delete")).Return(
					&fcre.CallResult{ExitCode: 0, Stdout: []byte("ok"), Stderr: nil}, nil,
				).Once()

				return m
			},
			assert: func(t *testing.T, _ fwops.Report[CREWorkflowDeleteInput, CREWorkflowDeleteOutput], err error) {
				t.Helper()
				require.NoError(t, err)
			},
		},
		{
			name: "custom target name is forwarded to CLI",
			input: func(t *testing.T) CREWorkflowDeleteInput {
				t.Helper()

				return CREWorkflowDeleteInput{
					WorkflowName:       "wf",
					DonFamily:          "feeds-zone-a",
					DeploymentRegistry: "private",
					Project:            creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("production-settings:\n  rpcs: []\n"))),
					TargetName:         "production-settings",
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()
				m := cremocks.NewMockCLIRunner(t)
				m.EXPECT().ContextRegistries().Return(testRegistries()).Once()
				m.EXPECT().Run(mock.Anything, mock.Anything, mock.MatchedBy(func(args []string) bool {
					tIdx := slices.Index(args, "-T")
					return tIdx >= 0 && tIdx+1 < len(args) && args[tIdx+1] == "production-settings"
				})).Return(
					&fcre.CallResult{ExitCode: 0, Stdout: []byte("ok"), Stderr: nil}, nil,
				).Once()

				return m
			},
			assert: func(t *testing.T, _ fwops.Report[CREWorkflowDeleteInput, CREWorkflowDeleteOutput], err error) {
				t.Helper()
				require.NoError(t, err)
			},
		},
		{
			name: "CLI exit error propagates exit code and output",
			input: func(t *testing.T) CREWorkflowDeleteInput {
				t.Helper()

				return CREWorkflowDeleteInput{
					WorkflowName:       "wf",
					DonFamily:          "feeds-zone-a",
					DeploymentRegistry: "private",
					Project:            creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("cld-deploy: {}\n"))),
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()
				exitErr := &fcre.ExitError{ExitCode: 7, Stdout: []byte("out"), Stderr: []byte("err")}
				m := cremocks.NewMockCLIRunner(t)
				m.EXPECT().ContextRegistries().Return(testRegistries()).Once()
				m.EXPECT().Run(mock.Anything, mock.Anything, mock.Anything).Return(
					&fcre.CallResult{ExitCode: 7, Stdout: exitErr.Stdout, Stderr: exitErr.Stderr}, exitErr,
				).Once()

				return m
			},
			assert: func(t *testing.T, out fwops.Report[CREWorkflowDeleteInput, CREWorkflowDeleteOutput], err error) {
				t.Helper()
				require.ErrorContains(t, err, "cre workflow delete")
				require.Equal(t, 7, out.Output.ExitCode)
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

			out, err := fwops.ExecuteOperation[CREWorkflowDeleteInput, CREWorkflowDeleteOutput, CREDeployDeps](
				bundle, CREWorkflowDeleteOp, deps, tc.input(t))
			tc.assert(t, out, err)
		})
	}
}

func TestCREWorkflowDeleteInput_resolveTargetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  CREWorkflowDeleteInput
		expect string
	}{
		{
			name:   "empty defaults to CREDeployTargetName",
			input:  CREWorkflowDeleteInput{},
			expect: CREDeployTargetName,
		},
		{
			name:   "whitespace defaults to CREDeployTargetName",
			input:  CREWorkflowDeleteInput{TargetName: "   "},
			expect: CREDeployTargetName,
		},
		{
			name:   "custom target is returned trimmed",
			input:  CREWorkflowDeleteInput{TargetName: " production-settings "},
			expect: "production-settings",
		},
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
	bundleDir := filepath.Join(workDir, creBundleSubdir)
	require.NoError(t, os.MkdirAll(bundleDir, 0o700))

	tests := []struct {
		name       string
		targetName string
		envPath    string
		extra      []string
		check      func(t *testing.T, args []string)
	}{
		{
			name:       "default target with env and extra args",
			targetName: CREDeployTargetName,
			envPath:    filepath.Join(workDir, ".env"),
			extra:      []string{"--extra"},
			check: func(t *testing.T, args []string) {
				t.Helper()
				require.Equal(t, []string{
					"workflow", "delete", bundleDir,
					"-R", workDir, "-T", CREDeployTargetName,
					"--yes",
					"-e", filepath.Join(workDir, ".env"),
					"--extra",
				}, args)
			},
		},
		{
			name:       "custom target without env or extra",
			targetName: "production-settings",
			check: func(t *testing.T, args []string) {
				t.Helper()
				require.NotContains(t, args, "-e")
				tIdx := slices.Index(args, "-T")
				require.NotEqual(t, -1, tIdx)
				require.Greater(t, len(args), tIdx+1)
				require.Equal(t, "production-settings", args[tIdx+1])
				require.Len(t, args, 8)
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
