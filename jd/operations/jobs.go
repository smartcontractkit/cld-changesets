package operations

import (
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	jobv1 "github.com/smartcontractkit/chainlink-protos/job-distributor/v1/job"
	nodev1 "github.com/smartcontractkit/chainlink-protos/job-distributor/v1/node"
	"github.com/smartcontractkit/chainlink-protos/job-distributor/v1/shared/ptypes"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/domain"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

// JobSpec is the per-job input for ProposeJobsInput.
type JobSpec struct {
	NodeLabels  map[string]string `json:"node_labels"            yaml:"node_labels"`
	JobLabels   map[string]string `json:"job_labels,omitempty"   yaml:"job_labels,omitempty"`
	JobspecTOML string            `json:"jobspec_toml"           yaml:"jobspec_toml"`
}

// ProposeJobsInput is the serializable input of SeqJDProposeJobs.
type ProposeJobsInput struct {
	Domain string    `json:"domain" yaml:"domain"`
	Jobs   []JobSpec `json:"jobs"   yaml:"jobs"`
}

// RevokeJobsInput is the serializable input of SeqJDRevokeJobs.
type RevokeJobsInput struct {
	JobIDs []string `json:"job_ids" yaml:"job_ids"`
}

// DeleteJobsInput is the serializable input of SeqJDDeleteJobs.
type DeleteJobsInput struct {
	JobIDs []string `json:"job_ids" yaml:"job_ids"`
}

// ProposeJobInput is the serializable input of OpJDProposeJob.
type ProposeJobInput struct {
	Domain      string            `json:"domain"`
	NodeLabels  map[string]string `json:"node_labels"`
	JobLabels   map[string]string `json:"job_labels,omitempty"`
	JobspecTOML string            `json:"jobspec_toml"`
}

// ProposeJobOutput is the serializable output of OpJDProposeJob.
// Empty: ProposeJob targets nodes by label and returns no proposal IDs.
type ProposeJobOutput struct{}

// RevokeJobInput is the serializable input of OpJDRevokeJob.
type RevokeJobInput struct {
	JobID string `json:"job_id"`
}

// RevokeJobOutput is the serializable output of OpJDRevokeJob.
type RevokeJobOutput struct {
	AlreadyAbsent bool `json:"already_absent"`
}

// DeleteJobInput is the serializable input of OpJDDeleteJob.
type DeleteJobInput struct {
	JobID string `json:"job_id"`
}

// DeleteJobOutput is the serializable output of OpJDDeleteJob.
type DeleteJobOutput struct{}

// ProposeJobsOutput is the serializable output of SeqJDProposeJobs.
type ProposeJobsOutput struct{}

// RevokeJobsOutput is the serializable output of SeqJDRevokeJobs.
type RevokeJobsOutput struct {
	RevokedJobIDs       []string `json:"revoked_job_ids"`
	AlreadyAbsentJobIDs []string `json:"already_absent_job_ids"`
}

// DeleteJobsOutput is the serializable output of SeqJDDeleteJobs.
type DeleteJobsOutput struct {
	DeletedJobIDs []string `json:"deleted_job_ids"`
}

// OpJDProposeJob proposes a single job spec to nodes matched by label.
// Fails if no nodes match the domain/environment/label selectors.
var OpJDProposeJob = fwops.NewOperation(
	"jd-propose-job",
	semver.MustParse("1.0.0"),
	"Propose a job spec to nodes matched by label",
	func(b fwops.Bundle, deps JDOpDeps, in ProposeJobInput) (ProposeJobOutput, error) {
		ctx := b.GetContext()
		d := domain.NewDomain("", in.Domain)
		domainKey := d.Key()
		envName := deps.EnvName

		selectors := make([]*ptypes.Selector, 0, 2+len(in.NodeLabels))
		selectors = append(selectors,
			&ptypes.Selector{Key: "product", Op: ptypes.SelectorOp_EQ, Value: &domainKey},
			&ptypes.Selector{Key: "environment", Op: ptypes.SelectorOp_EQ, Value: &envName},
		)
		for k, v := range in.NodeLabels {
			selectors = append(selectors, &ptypes.Selector{Key: k, Op: ptypes.SelectorOp_EQ, Value: &v})
		}

		resp, err := deps.Offchain.ListNodes(ctx, &nodev1.ListNodesRequest{
			Filter: &nodev1.ListNodesRequest_Filter{Enabled: 1, Selectors: selectors},
		})
		if err != nil {
			return ProposeJobOutput{}, fmt.Errorf("failed to list nodes: %w", err)
		}
		nodes := resp.GetNodes()
		if len(nodes) == 0 {
			return ProposeJobOutput{}, fmt.Errorf("no nodes matched domain=%q env=%q labels=%v", in.Domain, deps.EnvName, in.NodeLabels)
		}

		var merr error
		for _, node := range nodes {
			if _, err = deps.Offchain.ProposeJob(ctx, &jobv1.ProposeJobRequest{
				NodeId: node.GetId(),
				Spec:   in.JobspecTOML,
				Labels: labelsFromMap(in.JobLabels),
			}); err != nil {
				merr = errors.Join(merr, fmt.Errorf("failed to propose job to node %q: %w", node.GetId(), err))
			}
		}
		if merr != nil {
			return ProposeJobOutput{}, merr
		}
		b.Logger.Infow("proposed job", "node_labels", in.NodeLabels, "node_count", len(nodes))

		return ProposeJobOutput{}, nil
	},
)

// OpJDRevokeJob revokes a single job.
var OpJDRevokeJob = fwops.NewOperation(
	"jd-revoke-job",
	semver.MustParse("1.0.0"),
	"Revoke a job",
	func(b fwops.Bundle, deps JDOpDeps, in RevokeJobInput) (RevokeJobOutput, error) {
		_, err := deps.Offchain.RevokeJob(b.GetContext(), &jobv1.RevokeJobRequest{
			IdOneof: &jobv1.RevokeJobRequest_Id{Id: in.JobID},
		})
		if err != nil {
			if isJDNotFound(err) {
				b.Logger.Infow("job already absent or revoked", "job_id", in.JobID)

				return RevokeJobOutput{AlreadyAbsent: true}, nil
			}

			return RevokeJobOutput{}, fmt.Errorf("failed to revoke job %q: %w", in.JobID, err)
		}
		b.Logger.Infow("revoked job", "job_id", in.JobID)

		return RevokeJobOutput{}, nil
	},
)

// OpJDDeleteJob deletes a single job.
var OpJDDeleteJob = fwops.NewOperation(
	"jd-delete-job",
	semver.MustParse("1.0.0"),
	"Delete a job",
	func(b fwops.Bundle, deps JDOpDeps, in DeleteJobInput) (DeleteJobOutput, error) {
		res, err := deps.Offchain.DeleteJob(b.GetContext(), &jobv1.DeleteJobRequest{
			IdOneof: &jobv1.DeleteJobRequest_Id{Id: in.JobID},
		})
		if err != nil {
			return DeleteJobOutput{}, fmt.Errorf("failed to delete job %q: %w", in.JobID, err)
		}
		if res.GetJob().GetDeletedAt() == nil {
			return DeleteJobOutput{}, fmt.Errorf("delete job %q response did not confirm deletion", in.JobID)
		}
		b.Logger.Infow("deleted job", "job_id", in.JobID)

		return DeleteJobOutput{}, nil
	},
)

// SeqJDProposeJobs proposes multiple job specs. Failures are collected.
var SeqJDProposeJobs = fwops.NewSequence(
	"seq-jd-propose-jobs",
	semver.MustParse("1.0.0"),
	"Propose multiple job specs",
	func(b fwops.Bundle, deps JDOpDeps, in ProposeJobsInput) (ProposeJobsOutput, error) {
		var failed int
		var merr error
		for i, j := range in.Jobs {
			_, err := fwops.ExecuteOperation(b, OpJDProposeJob, deps, ProposeJobInput{
				Domain:      in.Domain,
				NodeLabels:  j.NodeLabels,
				JobLabels:   j.JobLabels,
				JobspecTOML: j.JobspecTOML,
			})
			if err != nil {
				failed++
				merr = errors.Join(merr, fmt.Errorf("job[%d]: propose failed: %w", i, err))
				b.Logger.Errorw("failed to propose job", "index", i, "node_labels", j.NodeLabels, "error", err)
			}
		}
		b.Logger.Infow("propose_jobs complete",
			"total", len(in.Jobs),
			"succeeded", len(in.Jobs)-failed,
			"failed", failed)

		return ProposeJobsOutput{}, merr
	},
)

// SeqJDRevokeJobs revokes multiple jobs. Failures are collected.
var SeqJDRevokeJobs = fwops.NewSequence(
	"seq-jd-revoke-jobs",
	semver.MustParse("1.0.0"),
	"Revoke multiple jobs",
	func(b fwops.Bundle, deps JDOpDeps, in RevokeJobsInput) (RevokeJobsOutput, error) {
		var revokedIDs, alreadyAbsentIDs []string
		var failed int
		var merr error
		for _, jobID := range in.JobIDs {
			report, err := fwops.ExecuteOperation(b, OpJDRevokeJob, deps, RevokeJobInput{JobID: jobID})
			if err != nil {
				failed++
				merr = errors.Join(merr, err)
				b.Logger.Errorw("failed to revoke job", "job_id", jobID, "error", err)

				continue
			}
			if report.Output.AlreadyAbsent {
				alreadyAbsentIDs = append(alreadyAbsentIDs, jobID)
			} else {
				revokedIDs = append(revokedIDs, jobID)
			}
		}
		b.Logger.Infow("revoke_jobs complete",
			"total", len(in.JobIDs),
			"revoked", len(revokedIDs),
			"already_absent", len(alreadyAbsentIDs),
			"failed", failed)

		return RevokeJobsOutput{RevokedJobIDs: revokedIDs, AlreadyAbsentJobIDs: alreadyAbsentIDs}, merr
	},
)

// SeqJDDeleteJobs deletes multiple jobs.
var SeqJDDeleteJobs = fwops.NewSequence(
	"seq-jd-delete-jobs",
	semver.MustParse("1.0.0"),
	"Delete multiple jobs",
	func(b fwops.Bundle, deps JDOpDeps, in DeleteJobsInput) (DeleteJobsOutput, error) {
		resp, err := deps.Offchain.ListProposals(b.GetContext(), &jobv1.ListProposalsRequest{
			Filter: &jobv1.ListProposalsRequest_Filter{JobIds: in.JobIDs},
		})
		if err != nil {
			return DeleteJobsOutput{}, fmt.Errorf("failed to list proposals: %w", err)
		}
		if len(resp.Proposals) == 0 {
			return DeleteJobsOutput{}, fmt.Errorf("no proposals found for job ids %v", in.JobIDs)
		}

		eligible := eligibleJobIDsForDeletion(resp.Proposals)
		if len(eligible) == 0 {
			return DeleteJobsOutput{}, errors.New("no jobs eligible for deletion: no proposals in PROPOSED, APPROVED, or PENDING state")
		}

		var deletedIDs []string
		var failed int
		var merr error
		for _, jobID := range eligible {
			_, err := fwops.ExecuteOperation(b, OpJDDeleteJob, deps, DeleteJobInput{JobID: jobID})
			if err != nil {
				failed++
				merr = errors.Join(merr, err)
				b.Logger.Errorw("failed to delete job", "job_id", jobID, "error", err)

				continue
			}
			deletedIDs = append(deletedIDs, jobID)
		}
		b.Logger.Infow("delete_jobs complete",
			"total", len(eligible),
			"deleted", len(deletedIDs),
			"failed", failed)

		return DeleteJobsOutput{DeletedJobIDs: deletedIDs}, merr
	},
)

func eligibleJobIDsForDeletion(proposals []*jobv1.Proposal) []string {
	seen := make(map[string]struct{})
	var eligible []string
	for _, p := range proposals {
		if p.Status == jobv1.ProposalStatus_PROPOSAL_STATUS_PROPOSED ||
			p.Status == jobv1.ProposalStatus_PROPOSAL_STATUS_APPROVED ||
			p.Status == jobv1.ProposalStatus_PROPOSAL_STATUS_PENDING {
			if _, ok := seen[p.JobId]; !ok {
				seen[p.JobId] = struct{}{}
				eligible = append(eligible, p.JobId)
			}
		}
	}

	return eligible
}
