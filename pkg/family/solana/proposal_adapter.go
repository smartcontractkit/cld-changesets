package solana

import (
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"

	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
)

// TimelockPrograms implements [cldfproposalutils.SolanaMCMSWithTimelock] for MCMS timelock proposal helpers.
func (s MCMSWithTimelockState) TimelockPrograms() cldfproposalutils.MCMSWithTimelockPrograms {

	p := s.MCMSWithTimelockPrograms

	return cldfproposalutils.MCMSWithTimelockPrograms{
		McmProgram:       p.McmProgram,
		ProposerMcmSeed:  mcmssolanasdk.PDASeed(p.ProposerMcmSeed),
		CancellerMcmSeed: mcmssolanasdk.PDASeed(p.CancellerMcmSeed),
		BypasserMcmSeed:  mcmssolanasdk.PDASeed(p.BypasserMcmSeed),
	}
}
