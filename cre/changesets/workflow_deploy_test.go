package changesets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/cre/operations"

	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	fcre "github.com/smartcontractkit/chainlink-deployments-framework/cre"
	creartifacts "github.com/smartcontractkit/chainlink-deployments-framework/cre/artifacts"
	cremocks "github.com/smartcontractkit/chainlink-deployments-framework/cre/mocks"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	focr "github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
)

func newTestEnv(t *testing.T, opts ...cldf.EnvironmentOption) *cldf.Environment {
	t.Helper()
	return cldf.NewEnvironment("t", logger.Test(t), cldf.NewMemoryAddressBook(),
		datastore.NewMemoryDataStore().Seal(), nil, nil, t.Context,
		focr.XXXGenerateTestOCRSecrets(), chain.NewBlockChains(map[uint64]chain.BlockChain{}),
		opts...)
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
	envNoCLI := newTestEnv(t, cldf.WithCRERunner(fcre.NewRunner()))
	envWithCLI := newTestEnv(t, cldf.WithCRERunner(fcre.NewRunner(fcre.WithCLI(mockCLI))))
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
