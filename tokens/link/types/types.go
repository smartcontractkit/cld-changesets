package types

import (
	"github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

const (
	// LinkToken is the burn/mint link token. It should be used everywhere for
	// new deployments. Corresponds to
	// https://github.com/smartcontractkit/chainlink/blob/develop/core/gethwrappers/shared/generated/link_token/link_token.go#L34
	ContractTypeLinkToken deployment.ContractType = "LinkToken"

	// StaticLinkToken represents the (very old) non-burn/mint link token.
	// It is not used in new deployments, but still exists on some chains
	// and has a distinct ABI from the new LinkToken.
	// Corresponds to the ABI
	// https://github.com/smartcontractkit/chainlink/blob/develop/core/gethwrappers/generated/link_token_interface/link_token_interface.go#L34
	ContractTypeStaticLinkToken deployment.ContractType = "StaticLinkToken"
)

// TypeAndVersion constants for the LINK token contracts.
var (
	BurnMintLinkTokenTypeAndVersion = deployment.NewTypeAndVersion(ContractTypeLinkToken, semvers.V1_0_0)
	StaticLinkTokenTypeAndVersion   = deployment.NewTypeAndVersion(ContractTypeStaticLinkToken, semvers.V1_0_0)
)
