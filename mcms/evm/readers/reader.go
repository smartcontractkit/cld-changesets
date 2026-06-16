package evmreaders

import (
	"fmt"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

// CallProxyReader resolves the MCMS call proxy ref for EVM chains.
type CallProxyReader interface {
	GetCallProxyRef(e cldf.Environment, chainSelector uint64, qualifier string) (datastore.AddressRef, error)
}

// Reader resolves MCMS timelock refs for EVM chains from the environment datastore.
type Reader struct{}

var (
	_ cldf.MCMSReader = Reader{}
	_ CallProxyReader = Reader{}
)

func (Reader) GetTimelockRef(e cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) (datastore.AddressRef, error) {
	return evmAddressRef(e, chainSelector, mcmscontracts.RBACTimelock, input.Qualifier)
}

func (Reader) GetMCMSRef(e cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) (datastore.AddressRef, error) {
	return evmMCMSRef(e, chainSelector, input)
}

func (Reader) GetCallProxyRef(e cldf.Environment, chainSelector uint64, qualifier string) (datastore.AddressRef, error) {
	return evmAddressRef(e, chainSelector, mcmscontracts.CallProxy, qualifier)
}

func (Reader) GetChainMetadata(e cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) (mcmstypes.ChainMetadata, error) {
	ref, err := evmMCMSRef(e, chainSelector, input)
	if err != nil {
		return mcmstypes.ChainMetadata{}, err
	}

	inspector, err := cldfproposalutils.McmsInspectorForChain(
		e,
		chainSelector,
		cldfproposalutils.WithTimelockAction(input.TimelockAction),
	)
	if err != nil {
		return mcmstypes.ChainMetadata{}, fmt.Errorf("build inspector for chain %d: %w", chainSelector, err)
	}

	opCount, err := inspector.GetOpCount(e.GetContext(), ref.Address)
	if err != nil {
		return mcmstypes.ChainMetadata{}, fmt.Errorf("get op count for chain %d: %w", chainSelector, err)
	}

	return mcmstypes.ChainMetadata{
		MCMAddress:      ref.Address,
		StartingOpCount: opCount,
	}, nil
}

func evmAddressRef(e cldf.Environment, chainSelector uint64, contractType cldf.ContractType, qualifier string) (datastore.AddressRef, error) {
	if e.DataStore == nil {
		return datastore.AddressRef{}, fmt.Errorf("datastore not available for chain %d", chainSelector)
	}

	return datastore.FindUniqueRef(e.DataStore.Addresses(), datastore.AddressRef{
		ChainSelector: chainSelector,
		Type:          datastore.ContractType(contractType),
		Qualifier:     qualifier,
	})
}

func evmMCMSRef(e cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) (datastore.AddressRef, error) {
	contractType, err := evmMCMContractType(input.TimelockAction)
	if err != nil {
		return datastore.AddressRef{}, err
	}

	return evmAddressRef(e, chainSelector, contractType, input.Qualifier)
}

func evmMCMContractType(action mcmstypes.TimelockAction) (cldf.ContractType, error) {
	switch action {
	case mcmstypes.TimelockActionSchedule, "":
		return mcmscontracts.ProposerManyChainMultisig, nil
	case mcmstypes.TimelockActionCancel:
		return mcmscontracts.CancellerManyChainMultisig, nil
	case mcmstypes.TimelockActionBypass:
		return mcmscontracts.BypasserManyChainMultisig, nil
	default:
		return "", fmt.Errorf("invalid timelock action %q", action)
	}
}
