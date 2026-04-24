// Package operations provides CRE workflow operations that execute side effects via the CRE CLI.
package operations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"

	fcre "github.com/smartcontractkit/chainlink-deployments-framework/cre"
	creartifacts "github.com/smartcontractkit/chainlink-deployments-framework/cre/artifacts"
	crecli "github.com/smartcontractkit/chainlink-deployments-framework/cre/cli"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// CREWorkflowDeleteOutput is the serializable result of a CRE CLI workflow delete invocation.
type CREWorkflowDeleteOutput struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

// CREWorkflowDeleteInput is the resolved input for a CRE workflow delete command.
type CREWorkflowDeleteInput struct {
	WorkflowName       string                    `json:"workflowName" yaml:"workflowName"`
	Project            creartifacts.ConfigSource `json:"project" yaml:"project"`
	DonFamily          string                    `json:"donFamily,omitempty" yaml:"donFamily,omitempty"`
	DeploymentRegistry string                    `json:"deploymentRegistry,omitempty" yaml:"deploymentRegistry,omitempty"`
	Context            crecli.ContextOverrides   `json:"context" yaml:"context"`
	ExtraCREArgs       []string                  `json:"extraCreArgs,omitempty" yaml:"extraCreArgs,omitempty"`
	TargetName         string                    `json:"targetName,omitempty" yaml:"targetName,omitempty"`
}

// Validate trims string fields and validates the delete input.
func (in *CREWorkflowDeleteInput) Validate() error {
	in.WorkflowName = strings.TrimSpace(in.WorkflowName)
	in.DonFamily = strings.TrimSpace(in.DonFamily)
	in.DeploymentRegistry = strings.TrimSpace(in.DeploymentRegistry)
	in.TargetName = strings.TrimSpace(in.TargetName)

	if in.WorkflowName == "" {
		return errors.New("workflowName is required")
	}
	if err := in.Project.Validate(); err != nil {
		return fmt.Errorf("project: %w", err)
	}
	if in.DeploymentRegistry == "" {
		return errors.New("deploymentRegistry is required")
	}
	if in.DonFamily == "" {
		return errors.New("donFamily is required")
	}

	return nil
}

func (in CREWorkflowDeleteInput) resolveTargetName() string {
	target := strings.TrimSpace(in.TargetName)
	if target != "" {
		return target
	}

	return CREDeployTargetName
}

// CREWorkflowDeleteOp deletes a workflow via the CRE CLI.
var CREWorkflowDeleteOp = fwops.NewOperation(
	"cre-workflow-delete",
	semver.MustParse("1.0.0"),
	"Deletes a CRE workflow via the CRE CLI subprocess",
	func(b fwops.Bundle, deps CREDeployDeps, input CREWorkflowDeleteInput) (CREWorkflowDeleteOutput, error) {
		ctx := b.GetContext()
		if deps.CLI == nil {
			return CREWorkflowDeleteOutput{}, errors.New("cre CLIRunner is nil")
		}

		workDir, err := os.MkdirTemp("", "cre-workflow-delete-*")
		if err != nil {
			return CREWorkflowDeleteOutput{}, fmt.Errorf("mkdir temp workflow artifacts: %w", err)
		}
		defer func() { _ = os.RemoveAll(workDir) }()

		resolver, err := creartifacts.NewArtifactsResolver(workDir)
		if err != nil {
			return CREWorkflowDeleteOutput{}, err
		}

		projectSrc, err := resolver.ResolveConfig(ctx, input.Project)
		if err != nil {
			return CREWorkflowDeleteOutput{}, fmt.Errorf("resolve project.yaml: %w", err)
		}

		projectDest := filepath.Join(workDir, "project.yaml")
		if err = copyFile(projectSrc, projectDest); err != nil {
			return CREWorkflowDeleteOutput{}, fmt.Errorf("copy project.yaml: %w", err)
		}

		bundleDir := filepath.Join(workDir, creBundleSubdir)
		if err = os.MkdirAll(bundleDir, 0o700); err != nil {
			return CREWorkflowDeleteOutput{}, err
		}

		target := input.resolveTargetName()
		workflowCfg := crecli.WorkflowConfig{
			target: {
				UserWorkflow: crecli.UserWorkflow{
					DeploymentRegistry: input.DeploymentRegistry,
					WorkflowName:       input.WorkflowName,
				},
			},
		}
		workflowYAMLPath, err := crecli.WriteWorkflowYAML(bundleDir, workflowCfg)
		if err != nil {
			return CREWorkflowDeleteOutput{}, fmt.Errorf("write workflow.yaml: %w", err)
		}

		ctxCfg, err := crecli.BuildContextConfig(input.DonFamily, input.Context, deps.CRECfg, deps.CLI.ContextRegistries())
		if err != nil {
			return CREWorkflowDeleteOutput{}, err
		}
		contextPath, err := crecli.WriteContextYAML(workDir, ctxCfg)
		if err != nil {
			return CREWorkflowDeleteOutput{}, fmt.Errorf("write context.yaml: %w", err)
		}

		logResolvedFile(b.Logger, "workflow.yaml", workflowYAMLPath, prettyYAML)
		logResolvedFile(b.Logger, "project.yaml", projectDest, prettyYAML)
		logResolvedFile(b.Logger, "context.yaml", contextPath, prettyYAML)

		envPath, err := crecli.WriteCREEnvFile(workDir, contextPath, deps.CRECfg, input.DonFamily)
		if err != nil {
			return CREWorkflowDeleteOutput{}, fmt.Errorf("write CRE .env file: %w", err)
		}

		args := BuildWorkflowDeleteArgs(target, workDir, envPath, input.ExtraCREArgs)
		b.Logger.Infow("Running CRE workflow delete", "args", args)

		res, runErr := deps.CLI.Run(ctx, nil, args...)
		if runErr != nil {
			var exitErr *fcre.ExitError
			if errors.As(runErr, &exitErr) {
				return CREWorkflowDeleteOutput{
					ExitCode: exitErr.ExitCode,
					Stdout:   string(exitErr.Stdout),
					Stderr:   string(exitErr.Stderr),
				}, fmt.Errorf("cre workflow delete: %w", runErr)
			}

			return CREWorkflowDeleteOutput{}, fmt.Errorf("cre workflow delete: %w", runErr)
		}
		if res == nil {
			return CREWorkflowDeleteOutput{}, errors.New("cre workflow delete: CLI returned nil result without error")
		}

		b.Logger.Infow("CRE workflow delete finished",
			"exitCode", res.ExitCode,
			"stdout", string(res.Stdout),
			"stderr", string(res.Stderr),
		)

		return CREWorkflowDeleteOutput{
			ExitCode: res.ExitCode,
			Stdout:   string(res.Stdout),
			Stderr:   string(res.Stderr),
		}, nil
	},
)

// BuildWorkflowDeleteArgs constructs the CRE CLI argument list for `cre workflow delete`.
func BuildWorkflowDeleteArgs(targetName, workDir, envPath string, extra []string) []string {
	bundleDir := filepath.Join(workDir, creBundleSubdir)
	args := []string{
		"workflow", "delete",
		bundleDir,
		"-R", workDir,
		"-T", targetName,
		"--yes",
	}
	if envPath != "" {
		args = append(args, "-e", envPath)
	}
	args = append(args, extra...)

	return args
}
