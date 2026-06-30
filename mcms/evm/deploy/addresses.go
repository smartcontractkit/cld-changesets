// Package evmdeploy provides the EVM chain-family implementation for the
// MCMS deploy changeset (mcms/changesets/deploy).
package evmdeploy

import (
	"fmt"

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

func loadDeployedAddresses(ds cldfdatastore.DataStore, chainSelector uint64, qualifier string) (deployedAddresses, error) {
	if ds == nil {
		return deployedAddresses{}, nil
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
		addr, ok, err := findDeployedAddress(ds.Addresses(), chainSelector, l.contractType, qualifier)
		if err != nil {
			return deployedAddresses{}, err
		}
		if ok {
			*l.dest = addr
		}
	}

	return addrs, nil
}

// findDeployedAddress returns a previously deployed contract address for this
// deploy package's version. Qualifier is always matched exactly, including "".
// Returns ok=false when no ref matches; an error when multiple refs match.
func findDeployedAddress(
	store cldfdatastore.AddressRefStore,
	chainSelector uint64,
	contractType cldf.ContractType,
	qualifier string,
) (common.Address, bool, error) {
	version := semvers.V1_0_0
	refs := store.Filter(
		cldfdatastore.AddressRefByChainSelector(chainSelector),
		cldfdatastore.AddressRefByType(cldfdatastore.ContractType(contractType)),
		cldfdatastore.AddressRefByQualifier(qualifier),
		cldfdatastore.AddressRefByVersion(&version),
	)
	switch len(refs) {
	case 0:
		return common.Address{}, false, nil
	case 1:
		return common.HexToAddress(refs[0].Address), true, nil
	default:
		return common.Address{}, false, fmt.Errorf(
			"%w: chain selector %d contract type %s qualifier %q version %s: found %d refs",
			cldfdatastore.ErrAddressRefQueryAmbiguous,
			chainSelector, contractType, qualifier, version, len(refs),
		)
	}
}

// addressRefWithLabel attaches an optional label to a deployed contract address ref.
func addressRefWithLabel(ref cldfdatastore.AddressRef, label string) cldfdatastore.AddressRef {
	if label != "" {
		ref.Labels = cldfdatastore.NewLabelSet(label)
	}

	return ref
}
