package solana

import cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

// Contract types for Solana MCMS / timelock entries in the datastore.
const (
	ManyChainMultisigProgram         cldf.ContractType = "ManyChainMultiSigProgram"
	RBACTimelockProgram              cldf.ContractType = "RBACTimelockProgram"
	AccessControllerProgram          cldf.ContractType = "AccessControllerProgram"
	ProposerAccessControllerAccount  cldf.ContractType = "ProposerAccessControllerAccount"
	ExecutorAccessControllerAccount  cldf.ContractType = "ExecutorAccessControllerAccount"
	CancellerAccessControllerAccount cldf.ContractType = "CancellerAccessControllerAccount"
	BypasserAccessControllerAccount  cldf.ContractType = "BypasserAccessControllerAccount"
)
