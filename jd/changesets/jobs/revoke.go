package jobs

import (
	"errors"
	"fmt"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	jdops "github.com/smartcontractkit/cld-changesets/jd/operations"
)

var _ cldf.ChangeSetV2[RevokeJobsInput] = RevokeJobsChangeset{}

// RevokeJobsInput is the serializable input of RevokeJobsChangeset.
type RevokeJobsInput = jdops.RevokeJobsInput

// RevokeJobsChangeset revokes jobs.
type RevokeJobsChangeset struct{}

func (RevokeJobsChangeset) VerifyPreconditions(_ cldf.Environment, cfg RevokeJobsInput) error {
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

	return nil
}

func (RevokeJobsChangeset) Apply(e cldf.Environment, input RevokeJobsInput) (cldf.ChangesetOutput, error) {
	deps := jdops.JDOpDeps{Offchain: e.Offchain, EnvName: e.Name}
	_, err := fwops.ExecuteSequence(e.OperationsBundle, jdops.SeqJDRevokeJobs, deps, input)

	return cldf.ChangesetOutput{}, err
}
