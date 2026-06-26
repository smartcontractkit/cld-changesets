// Package fundmcmpdas funds MCMS signer PDAs on Solana chains.
//
// This is a Solana-only changeset: PDAs and signer funding are Solana-specific
// concepts. The changeset entrypoint lives here under mcms/solana/changesets;
// sequences and operations are in mcms/solana/fund-mcm-pdas.
//
// # Usage
//
// Import the MCMS reader so datastore refs can be resolved:
//
//	import (
//		fundmcmpdas "github.com/smartcontractkit/cld-changesets/mcms/solana/changesets/fund-mcm-pdas"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
//	)
//
// Testing:
//
//	rt.Exec(runtime.ChangesetTask(fundmcmpdas.Changeset{}, fundmcmpdas.Config{
//		FundingPerChain: map[uint64]fundmcmpdas.FundingConfig{
//			selector: {
//				ProposeMCM:   100,
//				CancellerMCM: 100,
//				BypasserMCM:  100,
//				Timelock:     100,
//			},
//		},
//	}))
//
// CLD:
//
//	registry.Add("fund_mcm_pdas", Configure(fundmcmpdas.Changeset{}).WithEnvInput())
package fundmcmpdas
