// Package jobs provides changesets for managing Job Distributor jobs.
//
// # Usage
//
//	import "github.com/smartcontractkit/cld-changesets/jd/changesets/jobs"
//
//	// Propose jobs to nodes matched by domain and label:
//	_, err := runtime.ExecChangeset(rt, jobs.ProposeJobsChangeset{}, jobs.ProposeJobsInput{
//		Domain: "keystone",
//		Jobs: []jobs.JobSpec{{
//			NodeLabels:  map[string]string{"target": "oracle-1"},
//			JobspecTOML: "...",
//		}},
//	})
//
//	// Revoke jobs by ID (idempotent — already-absent IDs are not an error):
//	_, err = runtime.ExecChangeset(rt, jobs.RevokeJobsChangeset{}, jobs.RevokeJobsInput{
//		JobIDs: []string{"job-id-1"},
//	})
//
//	// Delete jobs by ID (preconditions verify existence and eligibility):
//	_, err = runtime.ExecChangeset(rt, jobs.DeleteJobsChangeset{}, jobs.DeleteJobsInput{
//		JobIDs: []string{"job-id-1"},
//	})
package jobs
