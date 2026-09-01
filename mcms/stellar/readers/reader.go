package stellarreaders

import (
	"errors"
	"fmt"

	mcmsstellar "github.com/smartcontractkit/mcms/sdk/stellar"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

var _ cldf.MCMSReader = Reader{}

// Reader resolves Stellar MCMS and timelock references from the CLDF datastore
// and reads the on-chain MCMS metadata required to construct proposals.
type Reader struct{}

func (Reader) GetTimelockRef(
	e cldf.Environment,
	chainSelector uint64,
	input cldf.MCMSTimelockProposalInput,
) (datastore.AddressRef, error) {
	ref, err := getAddressRef(
		e,
		chainSelector,
		datastore.ContractType(mcmscontracts.RBACTimelock),
		input.Qualifier,
	)
	if err != nil {
		return datastore.AddressRef{}, fmt.Errorf(
			"get stellar timelock ref for chain %d: %w",
			chainSelector,
			err,
		)
	}

	return ref, nil
}

func (Reader) GetMCMSRef(
	e cldf.Environment,
	chainSelector uint64,
	input cldf.MCMSTimelockProposalInput,
) (datastore.AddressRef, error) {
	var contractType datastore.ContractType

	switch input.TimelockAction {
	case "", mcmstypes.TimelockActionSchedule:
		contractType = datastore.ContractType(
			mcmscontracts.ProposerManyChainMultisig,
		)
	case mcmstypes.TimelockActionCancel:
		contractType = datastore.ContractType(
			mcmscontracts.CancellerManyChainMultisig,
		)
	case mcmstypes.TimelockActionBypass:
		contractType = datastore.ContractType(
			mcmscontracts.BypasserManyChainMultisig,
		)
	default:
		return datastore.AddressRef{}, fmt.Errorf(
			"invalid stellar timelock action %q",
			input.TimelockAction,
		)
	}

	ref, err := getAddressRef(
		e,
		chainSelector,
		contractType,
		input.Qualifier,
	)
	if err != nil {
		return datastore.AddressRef{}, fmt.Errorf(
			"get stellar MCMS ref for chain %d: %w",
			chainSelector,
			err,
		)
	}

	return ref, nil
}

func (r Reader) GetChainMetadata(
	e cldf.Environment,
	chainSelector uint64,
	input cldf.MCMSTimelockProposalInput,
) (mcmstypes.ChainMetadata, error) {
	mcmRef, err := r.GetMCMSRef(
		e,
		chainSelector,
		input,
	)
	if err != nil {
		return mcmstypes.ChainMetadata{}, err
	}

	chain, ok := e.BlockChains.StellarChains()[chainSelector]
	if !ok {
		return mcmstypes.ChainMetadata{}, fmt.Errorf(
			"stellar chain %d not found in environment",
			chainSelector,
		)
	}

	deployer, err := stellardeployment.NewDeployerFromChain(chain)
	if err != nil {
		return mcmstypes.ChainMetadata{}, fmt.Errorf(
			"create stellar deployer for chain %d: %w",
			chainSelector,
			err,
		)
	}

	inspector := mcmsstellar.NewInspectorFromInvoker(deployer)

	opCount, err := inspector.GetOpCount(
		e.GetContext(),
		mcmRef.Address,
	)
	if err != nil {
		return mcmstypes.ChainMetadata{}, fmt.Errorf(
			"get stellar MCMS op count for %s: %w",
			mcmRef.Address,
			err,
		)
	}

	return mcmstypes.ChainMetadata{
		MCMAddress:      mcmRef.Address,
		StartingOpCount: opCount,
	}, nil
}

func getAddressRef(
	e cldf.Environment,
	chainSelector uint64,
	contractType datastore.ContractType,
	qualifier string,
) (datastore.AddressRef, error) {
	if e.DataStore == nil {
		return datastore.AddressRef{}, errors.New(
			"datastore is not available",
		)
	}

	key := datastore.NewAddressRefKey(
		chainSelector,
		contractType,
		&semvers.V1_0_0,
		qualifier,
	)

	ref, err := e.DataStore.Addresses().Get(key)
	if err != nil {
		return datastore.AddressRef{}, fmt.Errorf(
			"resolve %s ref with qualifier %q: %w",
			contractType,
			qualifier,
			err,
		)
	}

	if ref.Address == "" {
		return datastore.AddressRef{}, fmt.Errorf(
			"%s ref has an empty address",
			contractType,
		)
	}

	return ref, nil
}
