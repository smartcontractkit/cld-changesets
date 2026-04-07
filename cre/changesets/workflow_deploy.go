// Package changesets provides CRE workflow changesets that can be applied to deployment environments.
package changesets

import (
	"fmt"
	"strings"

	creops "github.com/smartcontractkit/cld-changesets/cre/operations"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cfgenv "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/config/env"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// CREWorkflowDeployChangeset resolves workflow artifacts and runs the CRE CLI workflow deploy command.
type CREWorkflowDeployChangeset struct{}

// VerifyPreconditions ensures the environment can run CRE and input is valid.
func (CREWorkflowDeployChangeset) VerifyPreconditions(e cldf.Environment, input creops.CREWorkflowDeployInput) error {
	if e.CRERunner == nil {
		return fmt.Errorf("cre runner is not available in this environment")
	}
	if e.CRERunner.CLI() == nil {
		return fmt.Errorf("cre CLI runner is not configured")
	}
	if err := input.Validate(); err != nil {
		return err
	}
	if err := input.Project.Validate(); err != nil {
		return fmt.Errorf("project: %w", err)
	}
	if strings.TrimSpace(input.DeploymentRegistry) == "" {
		return fmt.Errorf("deploymentRegistry is required")
	}
	if strings.TrimSpace(input.DonFamily) == "" {
		return fmt.Errorf("donFamily is required")
	}
	return nil
}

// Apply loads CRE config and runs the CRE workflow deploy operation.
func (CREWorkflowDeployChangeset) Apply(e cldf.Environment, input creops.CREWorkflowDeployInput) (cldf.ChangesetOutput, error) {
	envCfg, err := cfgenv.LoadEnv()
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("load CRE env config: %w", err)
	}

	deps := creops.CREDeployDeps{
		CLI:            e.CRERunner.CLI(),
		CRECfg:         envCfg.CRE,
		EVMDeployerKey: envCfg.Onchain.EVM.DeployerKey,
	}

	report, err := fwops.ExecuteOperation(e.OperationsBundle, creops.CREWorkflowDeployOp, deps, input)
	out := cldf.ChangesetOutput{
		Reports: []fwops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}
	return out, nil
}
