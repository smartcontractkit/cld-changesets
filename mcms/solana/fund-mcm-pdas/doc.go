// Package solfundmcmpdas funds MCMS signer PDAs on Solana chains.
//
// This is a Solana-only changeset: PDAs and signer funding are Solana-specific
// concepts. Unlike set-config or deploy, there is no chain-agnostic layer under
// mcms/changesets — import this package directly.
//
// # Usage
//
// Import the MCMS reader so datastore refs can be resolved:
//
//	import (
//		solfundmcmpdas "github.com/smartcontractkit/cld-changesets/mcms/solana/fund-mcm-pdas"
//		_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
//	)
//
// Testing:
//
//	rt.Exec(runtime.ChangesetTask(solfundmcmpdas.Changeset{}, solfundmcmpdas.Config{
//		FundingPerChain: map[uint64]solfundmcmpdas.FundingConfig{
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
//	registry.Add("fund_mcm_pdas", Configure(solfundmcmpdas.Changeset{}).WithEnvInput())
package solfundmcmpdas
