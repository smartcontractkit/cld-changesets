package evm

import (
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
)

// TimelockContracts implements [cldfproposalutils.EVMMCMSWithTimelock] for MCMS timelock proposal helpers.
func (s MCMSWithTimelockState) TimelockContracts() cldfproposalutils.MCMSWithTimelockContracts {

	return cldfproposalutils.MCMSWithTimelockContracts{
		CancellerMcm: s.CancellerMcm,
		BypasserMcm:  s.BypasserMcm,
		ProposerMcm:  s.ProposerMcm,
		Timelock:     s.Timelock,
		CallProxy:    s.CallProxy,
	}
}
