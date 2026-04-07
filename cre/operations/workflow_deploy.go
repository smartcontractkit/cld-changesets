// Package operations provides CRE workflow operations that execute side effects via the CRE CLI.
package operations

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	cfgenv "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/config/env"

	fcre "github.com/smartcontractkit/chainlink-deployments-framework/cre"
	creartifacts "github.com/smartcontractkit/chainlink-deployments-framework/cre/artifacts"
	crecli "github.com/smartcontractkit/chainlink-deployments-framework/cre/cli"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"gopkg.in/yaml.v3"
)

const (
	// CREDeployTargetName is the workflow.yaml / project.yaml target key used for this layout.
	CREDeployTargetName = "cld-deploy"
	creBundleSubdir     = "bundle"
)

// CREDeployDeps holds non-serializable dependencies for the workflow deploy operation.
type CREDeployDeps struct {
	CLI    fcre.CLIRunner
	CRECfg cfgenv.CREConfig
	// EVMDeployerKey is the raw hex EVM private key from Onchain.EVM.DeployerKey.
	// Injected into the child process environment as CRE_ETH_PRIVATE_KEY only for on-chain registries.
	EVMDeployerKey string
}

// CREWorkflowDeployOutput is the serializable result of a CRE CLI deploy invocation.
type CREWorkflowDeployOutput struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// CREWorkflowDeployInput is the resolved input for a CRE workflow deploy.
// Binary, config, and project are resolved via [creartifacts.ArtifactsResolver].
// DeploymentRegistry is inherited from [creartifacts.WorkflowBundle].
type CREWorkflowDeployInput struct {
	creartifacts.WorkflowBundle `yaml:",inline"`
	// Project is the path to CRE CLI project.yaml (RPCs, don-family, etc.).
	// Resolved the same way as Binary and Config: local path or GitHub ref.
	Project creartifacts.ConfigSource `json:"project" yaml:"project"`
	// Optional - Context overrides CRE_* process env defaults for the generated context.yaml.
	Context crecli.ContextOverrides `json:"context" yaml:"context"`
	// Optional - ExtraCREArgs are appended after built-in workflow deploy arguments (e.g. org/tenant flags).
	ExtraCREArgs []string `json:"extraCreArgs,omitempty" yaml:"extraCreArgs,omitempty"`
}

// CREWorkflowDeployOp deploys a workflow via the CRE CLI (single side effect: CLI invocation).
var CREWorkflowDeployOp = fwops.NewOperation(
	"cre-workflow-deploy",
	semver.MustParse("1.0.0"),
	"Deploys a CRE workflow via the CRE CLI subprocess",
	func(b fwops.Bundle, deps CREDeployDeps, input CREWorkflowDeployInput) (CREWorkflowDeployOutput, error) {
		ctx := b.GetContext()
		if deps.CLI == nil {
			return CREWorkflowDeployOutput{}, errors.New("cre CLIRunner is nil")
		}

		workDir, err := os.MkdirTemp("", "cre-workflow-artifacts-*")
		if err != nil {
			return CREWorkflowDeployOutput{}, fmt.Errorf("mkdir temp workflow artifacts: %w", err)
		}
		defer func() { _ = os.RemoveAll(workDir) }()

		resolver, err := creartifacts.NewArtifactsResolver(workDir)
		if err != nil {
			return CREWorkflowDeployOutput{}, err
		}

		binaryPath, err := resolver.ResolveBinary(ctx, input.Binary)
		if err != nil {
			return CREWorkflowDeployOutput{}, fmt.Errorf("resolve workflow binary: %w", err)
		}

		configPath, err := resolver.ResolveConfig(ctx, input.Config)
		if err != nil {
			return CREWorkflowDeployOutput{}, fmt.Errorf("resolve workflow config: %w", err)
		}

		bundleDir := filepath.Join(workDir, creBundleSubdir)
		if err = os.MkdirAll(bundleDir, 0o700); err != nil {
			return CREWorkflowDeployOutput{}, err
		}

		projectSrc, err := resolver.ResolveConfig(ctx, input.Project)
		if err != nil {
			return CREWorkflowDeployOutput{}, fmt.Errorf("resolve project.yaml: %w", err)
		}
		projectDest := filepath.Join(workDir, "project.yaml")
		if err = copyFile(projectSrc, projectDest); err != nil {
			return CREWorkflowDeployOutput{}, fmt.Errorf("copy project.yaml: %w", err)
		}

		workflowCfg := crecli.WorkflowConfig{
			CREDeployTargetName: {
				UserWorkflow: crecli.UserWorkflow{
					DeploymentRegistry: input.DeploymentRegistry,
					WorkflowName:       input.WorkflowName,
				},
				WorkflowArtifacts: crecli.WorkflowArtifacts{
					WorkflowPath: ".",
					ConfigPath:   filepath.Base(configPath),
					SecretsPath:  "",
				},
			},
		}
		workflowYAMLPath, err := crecli.WriteWorkflowYAML(bundleDir, workflowCfg)
		if err != nil {
			return CREWorkflowDeployOutput{}, fmt.Errorf("write workflow.yaml: %w", err)
		}

		ctxCfg, err := crecli.BuildContextConfig(input.DonFamily, input.Context, deps.CRECfg, deps.CLI.ContextRegistries())
		if err != nil {
			return CREWorkflowDeployOutput{}, err
		}
		contextPath, err := crecli.WriteContextYAML(workDir, ctxCfg)
		if err != nil {
			return CREWorkflowDeployOutput{}, fmt.Errorf("write context.yaml: %w", err)
		}
		logResolvedFile(b.Logger, "workflow.yaml", workflowYAMLPath, prettyYAML)
		logResolvedFile(b.Logger, "project.yaml", projectDest, prettyYAML)
		logResolvedFile(b.Logger, "context.yaml", contextPath, prettyYAML)
		logResolvedFile(b.Logger, "config.json", configPath, prettyJSON)

		envPath, err := crecli.WriteCREEnvFile(workDir, contextPath, deps.CRECfg, input.DonFamily)
		if err != nil {
			return CREWorkflowDeployOutput{}, fmt.Errorf("write CRE .env file: %w", err)
		}

		args := BuildWorkflowDeployArgs(workDir, envPath, binaryPath, configPath, input.ExtraCREArgs)
		b.Logger.Infow("Running CRE workflow deploy", "args", args)

		var runEnv map[string]string
		if crecli.IsOnChainRegistry(input.DeploymentRegistry, crecli.FlatRegistries(ctxCfg)) {
			runEnv = map[string]string{
				"CRE_ETH_PRIVATE_KEY": deps.EVMDeployerKey,
			}
		}
		res, runErr := deps.CLI.Run(ctx, runEnv, args...)
		if runErr != nil {
			var exitErr *fcre.ExitError
			if errors.As(runErr, &exitErr) {
				return CREWorkflowDeployOutput{
					ExitCode: exitErr.ExitCode,
					Stdout:   string(exitErr.Stdout),
					Stderr:   string(exitErr.Stderr),
				}, fmt.Errorf("cre workflow deploy: %w", runErr)
			}

			return CREWorkflowDeployOutput{}, fmt.Errorf("cre workflow deploy: %w", runErr)
		}
		if res == nil {
			return CREWorkflowDeployOutput{}, errors.New("cre workflow deploy: CLI returned nil result without error")
		}

		b.Logger.Infow("CRE workflow deploy finished",
			"exitCode", res.ExitCode,
			"stdout", string(res.Stdout),
			"stderr", string(res.Stderr),
		)

		return CREWorkflowDeployOutput{
			ExitCode: res.ExitCode,
			Stdout:   string(res.Stdout),
			Stderr:   string(res.Stderr),
		}, nil
	},
)

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)

	return err
}

// BuildWorkflowDeployArgs constructs the CRE CLI argument list for `cre workflow deploy`.
func BuildWorkflowDeployArgs(workDir, envPath, binaryPath, configPath string, extra []string) []string {
	bundleDir := filepath.Join(workDir, creBundleSubdir)
	args := []string{
		"workflow", "deploy",
		bundleDir,
		"-R", workDir,
		"-T", CREDeployTargetName,
		"--wasm", binaryPath,
		"--config", configPath,
		"--yes",
	}
	if envPath != "" {
		args = append(args, "-e", envPath)
	}
	args = append(args, extra...)

	return args
}

func logResolvedFile(lggr logger.Logger, name, path string, formatter func([]byte) string) {
	content, err := os.ReadFile(path)
	if err != nil {
		lggr.Infow("Resolved artifact file", "name", name, "path", path, "error", err)
		return
	}

	lggr.Infof("--- Resolved %s (%s) ---\n%s", name, path, strings.TrimRight(formatter(content), "\n"))
}

func prettyYAML(content []byte) string {
	var v any
	if err := yaml.Unmarshal(content, &v); err != nil {
		return string(content)
	}

	out, err := yaml.Marshal(v)
	if err != nil {
		return string(content)
	}

	return string(out)
}

func prettyJSON(content []byte) string {
	var out bytes.Buffer
	if err := json.Indent(&out, content, "", "  "); err != nil {
		return string(content)
	}

	return out.String()
}
