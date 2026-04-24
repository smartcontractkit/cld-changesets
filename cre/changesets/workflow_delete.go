// Package changesets provides CRE workflow changesets that can be applied to deployment environments.
package changesets

import (
	"errors"
	"fmt"

	creops "github.com/smartcontractkit/cld-changesets/cre/operations"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cfgenv "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/config/env"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// CREWorkflowDeleteChangeset runs the CRE CLI workflow delete command.
type CREWorkflowDeleteChangeset struct{}

// VerifyPreconditions ensures the environment can run CRE and input is valid.
func (CREWorkflowDeleteChangeset) VerifyPreconditions(e cldf.Environment, input creops.CREWorkflowDeleteInput) error {
	if e.CRERunner == nil {
		return errors.New("cre runner is not available in this environment")
	}
	if e.CRERunner.CLI() == nil {
		return errors.New("cre CLI runner is not configured")
	}
	if err := input.Validate(); err != nil {
		return err
	}

	return nil
}

// Apply loads CRE config and runs the CRE workflow delete operation.
func (CREWorkflowDeleteChangeset) Apply(e cldf.Environment, input creops.CREWorkflowDeleteInput) (cldf.ChangesetOutput, error) {
	if err := input.Validate(); err != nil {
		return cldf.ChangesetOutput{}, err
	}

	envCfg, err := cfgenv.LoadEnv()
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("load CRE env config: %w", err)
	}

	deps := creops.CREDeployDeps{
		CLI:            e.CRERunner.CLI(),
		CRECfg:         envCfg.CRE,
		EVMDeployerKey: envCfg.Onchain.EVM.DeployerKey,
	}

	report, err := fwops.ExecuteOperation(e.OperationsBundle, creops.CREWorkflowDeleteOp, deps, input)
	out := cldf.ChangesetOutput{
		Reports: []fwops.Report[any, any]{report.ToGenericReport()},
	}
	if err != nil {
		return out, err
	}

	return out, nil
}
