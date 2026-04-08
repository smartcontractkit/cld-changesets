package operations

import (
	"context"
	"os"
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

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, data, 0o600))

	return p
}

// ---------------------------------------------------------------------------
// CREWorkflowDeployOp
// ---------------------------------------------------------------------------

func TestCREWorkflowDeployOp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    func(t *testing.T) CREWorkflowDeployInput
		setupCLI func(t *testing.T) *cremocks.MockCLIRunner
		assert   func(t *testing.T, out fwops.Report[CREWorkflowDeployInput, CREWorkflowDeployOutput], err error)
	}{
		{
			name: "success invokes CLI with deploy args",
			input: func(t *testing.T) CREWorkflowDeployInput {
				t.Helper()

				return CREWorkflowDeployInput{
					WorkflowBundle: creartifacts.WorkflowBundle{
						WorkflowName:       "wf",
						Binary:             creartifacts.NewBinarySourceLocal(writeFile(t, "x.wasm", []byte("wasm"))),
						Config:             creartifacts.NewConfigSourceLocal(writeFile(t, "cfg.json", []byte(`{"a":1}`))),
						DonFamily:          "feeds-zone-a",
						DeploymentRegistry: "private",
					},
					Project: creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("cld-deploy:\n  cre-cli:\n    don-family: feeds-zone-a\n"))),
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()
				m := cremocks.NewMockCLIRunner(t)
				m.EXPECT().ContextRegistries().Return(testRegistries()).Once()
				m.EXPECT().Run(mock.Anything, mock.Anything, matchCLIArgs("workflow", "deploy")).Return(
					&fcre.CallResult{ExitCode: 0, Stdout: []byte("ok"), Stderr: nil}, nil,
				).Once()

				return m
			},
			assert: func(t *testing.T, _ fwops.Report[CREWorkflowDeployInput, CREWorkflowDeployOutput], err error) {
				t.Helper()
				require.NoError(t, err)
			},
		},
		{
			name: "custom target name is forwarded to CLI",
			input: func(t *testing.T) CREWorkflowDeployInput {
				t.Helper()

				return CREWorkflowDeployInput{
					WorkflowBundle: creartifacts.WorkflowBundle{
						WorkflowName:       "wf",
						Binary:             creartifacts.NewBinarySourceLocal(writeFile(t, "x.wasm", []byte("wasm"))),
						Config:             creartifacts.NewConfigSourceLocal(writeFile(t, "cfg.json", []byte(`{"a":1}`))),
						DonFamily:          "feeds-zone-a",
						DeploymentRegistry: "private",
					},
					Project:    creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("production-settings:\n  rpcs: []\n"))),
					TargetName: "production-settings",
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()
				m := cremocks.NewMockCLIRunner(t)
				m.EXPECT().ContextRegistries().Return(testRegistries()).Once()
				m.EXPECT().Run(mock.Anything, mock.Anything, mock.MatchedBy(func(args []string) bool {
					tIdx := indexOf(args, "-T")
					return tIdx >= 0 && tIdx+1 < len(args) && args[tIdx+1] == "production-settings"
				})).Return(
					&fcre.CallResult{ExitCode: 0, Stdout: []byte("ok"), Stderr: nil}, nil,
				).Once()

				return m
			},
			assert: func(t *testing.T, _ fwops.Report[CREWorkflowDeployInput, CREWorkflowDeployOutput], err error) {
				t.Helper()
				require.NoError(t, err)
			},
		},
		{
			name: "missing binary returns resolve error",
			input: func(t *testing.T) CREWorkflowDeployInput {
				t.Helper()

				return CREWorkflowDeployInput{
					WorkflowBundle: creartifacts.WorkflowBundle{
						WorkflowName:       "wf",
						Binary:             creartifacts.NewBinarySourceLocal(filepath.Join(t.TempDir(), "missing.wasm")),
						Config:             creartifacts.NewConfigSourceLocal(writeFile(t, "cfg.json", []byte(`{}`))),
						DonFamily:          "zone",
						DeploymentRegistry: "private",
					},
					Project: creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("cld-deploy: {}\n"))),
				}
			},
			setupCLI: func(t *testing.T) *cremocks.MockCLIRunner {
				t.Helper()

				return cremocks.NewMockCLIRunner(t)
			},
			assert: func(t *testing.T, _ fwops.Report[CREWorkflowDeployInput, CREWorkflowDeployOutput], err error) {
				t.Helper()
				require.ErrorContains(t, err, "resolve workflow binary")
			},
		},
		{
			name: "CLI exit error propagates exit code and output",
			input: func(t *testing.T) CREWorkflowDeployInput {
				t.Helper()

				return CREWorkflowDeployInput{
					WorkflowBundle: creartifacts.WorkflowBundle{
						WorkflowName:       "wf",
						Binary:             creartifacts.NewBinarySourceLocal(writeFile(t, "x.wasm", []byte("wasm"))),
						Config:             creartifacts.NewConfigSourceLocal(writeFile(t, "cfg.json", []byte(`{}`))),
						DonFamily:          "feeds-zone-a",
						DeploymentRegistry: "private",
					},
					Project: creartifacts.NewConfigSourceLocal(writeFile(t, "project.yaml", []byte("cld-deploy: {}\n"))),
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
			assert: func(t *testing.T, out fwops.Report[CREWorkflowDeployInput, CREWorkflowDeployOutput], err error) {
				t.Helper()
				require.ErrorContains(t, err, "cre workflow deploy")
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

			out, err := fwops.ExecuteOperation(bundle, CREWorkflowDeployOp, deps, tc.input(t))
			tc.assert(t, out, err)
		})
	}
}

// ---------------------------------------------------------------------------
// resolveTargetName
// ---------------------------------------------------------------------------

func TestResolveTargetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  CREWorkflowDeployInput
		expect string
	}{
		{
			name:   "empty defaults to CREDeployTargetName",
			input:  CREWorkflowDeployInput{},
			expect: CREDeployTargetName,
		},
		{
			name:   "whitespace defaults to CREDeployTargetName",
			input:  CREWorkflowDeployInput{TargetName: "   "},
			expect: CREDeployTargetName,
		},
		{
			name:   "custom target is returned trimmed",
			input:  CREWorkflowDeployInput{TargetName: " production-settings "},
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

// ---------------------------------------------------------------------------
// BuildWorkflowDeployArgs
// ---------------------------------------------------------------------------

func TestBuildWorkflowDeployArgs(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	bundleDir := filepath.Join(workDir, creBundleSubdir)
	require.NoError(t, os.MkdirAll(bundleDir, 0o700))
	wasm := filepath.Join(workDir, "x.wasm")
	cfg := filepath.Join(workDir, "c.json")

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
					"workflow", "deploy", bundleDir,
					"-R", workDir, "-T", CREDeployTargetName,
					"--wasm", wasm, "--config", cfg, "--yes",
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
				tIdx := indexOf(args, "-T")
				require.NotEqual(t, -1, tIdx)
				require.Greater(t, len(args), tIdx+1)
				require.Equal(t, "production-settings", args[tIdx+1])
				require.Len(t, args, 12)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.check(t, BuildWorkflowDeployArgs(tc.targetName, workDir, tc.envPath, wasm, cfg, tc.extra))
		})
	}
}

func testRegistries() []fcre.ContextRegistryEntry {
	return []fcre.ContextRegistryEntry{
		{
			ID:               "private",
			Label:            "Private (Chainlink-hosted)",
			Type:             "off-chain",
			SecretsAuthFlows: []string{"browser", "owner-key-signing"},
		},
	}
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

func indexOf(sl []string, s string) int {
	for i, v := range sl {
		if v == s {
			return i
		}
	}
	return -1
}
