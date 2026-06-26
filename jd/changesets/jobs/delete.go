package jobs

import (
	"errors"
	"fmt"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	jobv1 "github.com/smartcontractkit/chainlink-protos/job-distributor/v1/job"

	jdops "github.com/smartcontractkit/cld-changesets/jd/operations"
)

var _ cldf.ChangeSetV2[DeleteJobsInput] = DeleteJobsChangeset{}

// DeleteJobsInput is the serializable input of DeleteJobsChangeset.
type DeleteJobsInput = jdops.DeleteJobsInput

// DeleteJobsChangeset deletes jobs.
// Only jobs with proposals in PROPOSED, APPROVED, or PENDING state are eligible.
type DeleteJobsChangeset struct{}

func (DeleteJobsChangeset) VerifyPreconditions(env cldf.Environment, cfg DeleteJobsInput) error {
	if len(cfg.JobIDs) == 0 {
		return errors.New("no job_ids provided")
	}
	seen := make(map[string]struct{}, len(cfg.JobIDs))
	for _, id := range cfg.JobIDs {
		if id == "" {
			return errors.New("job id cannot be empty")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate job id %q", id)
		}
		seen[id] = struct{}{}
	}

	jobs, err := env.Offchain.ListJobs(env.GetContext(), &jobv1.ListJobsRequest{
		Filter: &jobv1.ListJobsRequest_Filter{Ids: cfg.JobIDs, IncludeDeleted: true},
	})
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}
	for _, j := range jobs.Jobs {
		if j.DeletedAt != nil {
			return fmt.Errorf("job %q is already deleted", j.Id)
		}
	}
	if len(jobs.Jobs) != len(cfg.JobIDs) {
		found := make([]string, 0, len(jobs.Jobs))
		for _, j := range jobs.Jobs {
			found = append(found, j.Id)
		}

		return fmt.Errorf("not all jobs found: requested %v, found %v", cfg.JobIDs, found)
	}

	return nil
}

func (DeleteJobsChangeset) Apply(e cldf.Environment, input DeleteJobsInput) (cldf.ChangesetOutput, error) {
	deps := jdops.JDOpDeps{Offchain: e.Offchain, EnvName: e.Name}
	_, err := fwops.ExecuteSequence(e.OperationsBundle, jdops.SeqJDDeleteJobs, deps, input)

	return cldf.ChangesetOutput{}, err
}
