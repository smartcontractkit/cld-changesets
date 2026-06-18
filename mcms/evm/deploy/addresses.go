// Package evmdeploy provides the EVM chain-family implementation for the
// MCMS deploy changeset (mcms/changesets/deploy).
package evmdeploy

import (
	"github.com/ethereum/go-ethereum/common"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

// deployedAddresses holds the on-chain addresses of an MCMS+timelock set on one
// EVM chain. A zero address means the contract has not yet been deployed.
type deployedAddresses struct {
	Bypasser  common.Address
	Canceller common.Address
	Proposer  common.Address
	Timelock  common.Address
	CallProxy common.Address
}

func loadDeployedAddresses(ds cldfdatastore.DataStore, chainSelector uint64, qualifier string) deployedAddresses {
	if ds == nil {
		return deployedAddresses{}
	}

	type lookup struct {
		contractType cldf.ContractType
		dest         *common.Address
	}

	var addrs deployedAddresses
	lookups := []lookup{
		{mcmscontracts.BypasserManyChainMultisig, &addrs.Bypasser},
		{mcmscontracts.CancellerManyChainMultisig, &addrs.Canceller},
		{mcmscontracts.ProposerManyChainMultisig, &addrs.Proposer},
		{mcmscontracts.RBACTimelock, &addrs.Timelock},
		{mcmscontracts.CallProxy, &addrs.CallProxy},
	}

	for _, l := range lookups {
		if addr, ok := findDeployedAddress(ds.Addresses(), chainSelector, l.contractType, qualifier); ok {
			*l.dest = addr
		}
	}

	return addrs
}

// findDeployedAddress returns a previously deployed contract address for this
// deploy package's version. Legacy datastore entries without a version are
// accepted as a fallback; refs with a different version are ignored so a
// version bump can redeploy.
func findDeployedAddress(
	store cldfdatastore.AddressRefStore,
	chainSelector uint64,
	contractType cldf.ContractType,
	qualifier string,
) (common.Address, bool) {
	baseFilters := make([]cldfdatastore.FilterFunc[cldfdatastore.AddressRefKey, cldfdatastore.AddressRef], 0, 3)
	baseFilters = append(baseFilters,
		cldfdatastore.AddressRefByChainSelector(chainSelector),
		cldfdatastore.AddressRefByType(cldfdatastore.ContractType(contractType)),
		cldfdatastore.AddressRefByQualifier(qualifier),
	)

	version := semvers.V1_0_0
	refs := store.Filter(append(baseFilters, cldfdatastore.AddressRefByVersion(&version))...)
	if len(refs) > 0 {
		return common.HexToAddress(refs[0].Address), true
	}

	refs = store.Filter(baseFilters...)
	for _, ref := range refs {
		if ref.Version == nil {
			return common.HexToAddress(ref.Address), true
		}
	}

	return common.Address{}, false
}

// addressRefWithLabel attaches an optional label to a deployed contract address ref.
func addressRefWithLabel(ref cldfdatastore.AddressRef, label string) cldfdatastore.AddressRef {
	if label != "" {
		ref.Labels = cldfdatastore.NewLabelSet(label)
	}

	return ref
}
