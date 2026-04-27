package changesets

import (
	"errors"
	"fmt"

	jobv1 "github.com/smartcontractkit/chainlink-protos/job-distributor/v1/job"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

var (
	// RevokeJobsChangeset revokes job proposals with the given jobIDs through JD. It can only be used when
	// each proposal is in the proposed or cancelled state in JD.
	RevokeJobsChangeset = cldf.CreateChangeSet(revokeJobsLogic, revokeJobsPrecondition)

	// DeleteJobsChangeset sends a delete request to the node where each job is running and marks them as deleted in Job Distributor.
	// If the node is not connected or the delete request fails, the deletion process is halted.
	// Nodes are expected to cancel the job once the request is sent by JD.
	DeleteJobsChangeset = cldf.CreateChangeSet(deleteJobsLogic, deleteJobsPrecondition)
)

func revokeJobsPrecondition(env cldf.Environment, jobIDs []string) error {
	proposals, err := env.Offchain.ListProposals(env.GetContext(), &jobv1.ListProposalsRequest{
		Filter: &jobv1.ListProposalsRequest_Filter{
			JobIds: jobIDs,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list proposals for jobIDs %v: %w", jobIDs, err)
	}
	found := make(map[string]struct{}, len(proposals.Proposals))
	for _, proposal := range proposals.Proposals {
		if proposal.Status != jobv1.ProposalStatus_PROPOSAL_STATUS_PROPOSED && proposal.Status != jobv1.ProposalStatus_PROPOSAL_STATUS_CANCELLED {
			return fmt.Errorf("proposal %s is not in PROPOSED or CANCELLED state", proposal.Id)
		}
		found[proposal.JobId] = struct{}{}
	}
	for _, jobID := range jobIDs {
		if _, ok := found[jobID]; !ok {
			return fmt.Errorf("no proposal found for jobID %s", jobID)
		}
	}

	return nil
}

func revokeJobsLogic(env cldf.Environment, jobIDs []string) (cldf.ChangesetOutput, error) {
	var successfullyRevoked []string
	for _, jobID := range jobIDs {
		res, err := env.Offchain.RevokeJob(env.GetContext(), &jobv1.RevokeJobRequest{
			IdOneof: &jobv1.RevokeJobRequest_Id{Id: jobID},
		})
		if err != nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("failed to revoke job %s: %w", jobID, err)
		}
		if res == nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("revoke job response is nil for job %s", jobID)
		}
		if res.Proposal == nil || res.Proposal.Status != jobv1.ProposalStatus_PROPOSAL_STATUS_REVOKED {
			return cldf.ChangesetOutput{}, fmt.Errorf("revoke job %s response is not in revoked state, got %s", jobID, res.Proposal.GetStatus())
		}
		successfullyRevoked = append(successfullyRevoked, jobID)
	}
	env.Logger.Infof("successfully revoked jobs %v", successfullyRevoked)

	return cldf.ChangesetOutput{}, nil
}

func deleteJobsPrecondition(env cldf.Environment, jobIDs []string) error {
	jobs, err := env.Offchain.ListJobs(env.GetContext(), &jobv1.ListJobsRequest{
		Filter: &jobv1.ListJobsRequest_Filter{
			Ids: jobIDs,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to list jobs for jobIDs %v: %w", jobIDs, err)
	}
	for _, job := range jobs.Jobs {
		if job.DeletedAt != nil {
			return fmt.Errorf("job %s is already deleted", job.Id)
		}
	}
	if len(jobs.Jobs) != len(jobIDs) {
		found := make([]string, 0, len(jobs.Jobs))
		for _, job := range jobs.Jobs {
			found = append(found, job.Id)
		}

		return fmt.Errorf("not all jobs found in listJobs response, returned job ids %v, expected %v", found, jobIDs)
	}

	return nil
}

func deleteJobsLogic(env cldf.Environment, jobIDs []string) (cldf.ChangesetOutput, error) {
	jobIDsToDelete, err := jobsToDelete(env, jobIDs)
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("failed to get jobIDs to delete: %w", err)
	}
	if len(jobIDsToDelete) == 0 {
		return cldf.ChangesetOutput{}, errors.New("no jobs to delete: no proposals in PROPOSED, APPROVED, or PENDING state for the given job ids")
	}
	for _, jobID := range jobIDsToDelete {
		res, err := env.Offchain.DeleteJob(env.GetContext(), &jobv1.DeleteJobRequest{
			IdOneof: &jobv1.DeleteJobRequest_Id{Id: jobID},
		})
		if err != nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("failed to delete job %s: %w", jobID, err)
		}
		if res == nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("delete job response is nil for job %s", jobID)
		}
		if res.Job == nil || res.Job.DeletedAt == nil {
			return cldf.ChangesetOutput{}, fmt.Errorf("delete job response is not in deleted state for job %s", jobID)
		}
	}
	env.Logger.Infof("successfully deleted jobs %v", jobIDsToDelete)

	return cldf.ChangesetOutput{}, nil
}

func jobsToDelete(env cldf.Environment, jobIDs []string) ([]string, error) {
	proposalsResp, err := env.Offchain.ListProposals(env.GetContext(), &jobv1.ListProposalsRequest{
		Filter: &jobv1.ListProposalsRequest_Filter{
			JobIds: jobIDs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list proposals for jobIDs %v: %w", jobIDs, err)
	}
	if len(proposalsResp.Proposals) == 0 {
		return nil, fmt.Errorf("no proposals found for jobIDs %v", jobIDs)
	}
	seen := make(map[string]struct{})
	var jobIDsToDelete []string
	for _, proposal := range proposalsResp.Proposals {
		if proposal.Status == jobv1.ProposalStatus_PROPOSAL_STATUS_PROPOSED ||
			proposal.Status == jobv1.ProposalStatus_PROPOSAL_STATUS_APPROVED ||
			proposal.Status == jobv1.ProposalStatus_PROPOSAL_STATUS_PENDING {
			if _, ok := seen[proposal.JobId]; ok {
				continue
			}
			seen[proposal.JobId] = struct{}{}
			jobIDsToDelete = append(jobIDsToDelete, proposal.JobId)
		}
	}

	return jobIDsToDelete, nil
}
