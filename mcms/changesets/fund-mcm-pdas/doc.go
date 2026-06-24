// Package fundmcmpdas provides the FundMCMPDAs changeset for funding MCMS signer
// PDAs on Solana chains.
//
// # Usage
//
// Import the changeset and blank-import the Solana MCMS reader:
//
//	import (
//		fundmcmpdas "github.com/smartcontractkit/cld-changesets/mcms/changesets/fund-mcm-pdas"
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
//
// Solana-specific sequences and operations live in mcms/solana/fund-mcm-pdas.
package fundmcmpdas
