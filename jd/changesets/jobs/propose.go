package jobs

import (
	"errors"
	"fmt"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	jdops "github.com/smartcontractkit/cld-changesets/jd/operations"
)

var _ cldf.ChangeSetV2[ProposeJobsInput] = ProposeJobsChangeset{}

// JobSpec is the per-job input for ProposeJobsInput.
type JobSpec = jdops.JobSpec

// ProposeJobsInput is the serializable input of ProposeJobsChangeset.
type ProposeJobsInput = jdops.ProposeJobsInput

// ProposeJobsChangeset proposes a batch of job specs to nodes matched by label.
type ProposeJobsChangeset struct{}

func (ProposeJobsChangeset) VerifyPreconditions(_ cldf.Environment, cfg ProposeJobsInput) error {
	if cfg.Domain == "" {
		return errors.New("domain is required")
	}
	if len(cfg.Jobs) == 0 {
		return errors.New("no jobs provided")
	}
	for i, j := range cfg.Jobs {
		if j.JobspecTOML == "" {
			return fmt.Errorf("job[%d]: jobspec_toml is required", i)
		}
		if len(j.NodeLabels) == 0 {
			return fmt.Errorf("job[%d]: node_labels is required", i)
		}
		for k, v := range j.NodeLabels {
			if k == "" || v == "" {
				return fmt.Errorf("job[%d]: node_labels key and value must be non-empty (key=%q value=%q)", i, k, v)
			}
		}
	}

	return nil
}

func (ProposeJobsChangeset) Apply(e cldf.Environment, input ProposeJobsInput) (cldf.ChangesetOutput, error) {
	deps := jdops.JDOpDeps{Offchain: e.Offchain, EnvName: e.Name}
	_, err := fwops.ExecuteSequence(e.OperationsBundle, jdops.SeqJDProposeJobs, deps, input)

	return cldf.ChangesetOutput{}, err
}
