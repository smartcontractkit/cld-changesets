package solreaders

import (
	"fmt"

	solanago "github.com/gagliardetto/solana-go"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

// Reader resolves MCMS timelock refs for Solana chains from the environment datastore.
type Reader struct{}

var _ cldf.MCMSReader = Reader{}

func (Reader) GetTimelockRef(e cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) (datastore.AddressRef, error) {
	if err := requireSolanaChain(e, chainSelector); err != nil {
		return datastore.AddressRef{}, err
	}

	return solanaAddressRef(e, chainSelector, mcmscontracts.RBACTimelock, input.Qualifier)
}

func (Reader) GetMCMSRef(e cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) (datastore.AddressRef, error) {
	if err := requireSolanaChain(e, chainSelector); err != nil {
		return datastore.AddressRef{}, err
	}

	return solanaMCMSRef(e, chainSelector, input)
}

func (Reader) GetChainMetadata(e cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) (mcmstypes.ChainMetadata, error) {
	if err := requireSolanaChain(e, chainSelector); err != nil {
		return mcmstypes.ChainMetadata{}, err
	}

	mcmRef, err := solanaMCMSRef(e, chainSelector, input)
	if err != nil {
		return mcmstypes.ChainMetadata{}, err
	}

	mcmProgram, mcmSeed, err := mcmssolanasdk.ParseContractAddress(mcmRef.Address)
	if err != nil {
		return mcmstypes.ChainMetadata{}, fmt.Errorf("parse MCMS address for chain %d: %w", chainSelector, err)
	}

	proposerAccessController, err := solanaAccessControllerPubkey(e, chainSelector, mcmscontracts.ProposerAccessControllerAccount, input.Qualifier)
	if err != nil {
		return mcmstypes.ChainMetadata{}, err
	}
	cancellerAccessController, err := solanaAccessControllerPubkey(e, chainSelector, mcmscontracts.CancellerAccessControllerAccount, input.Qualifier)
	if err != nil {
		return mcmstypes.ChainMetadata{}, err
	}
	bypasserAccessController, err := solanaAccessControllerPubkey(e, chainSelector, mcmscontracts.BypasserAccessControllerAccount, input.Qualifier)
	if err != nil {
		return mcmstypes.ChainMetadata{}, err
	}

	metadata, err := mcmssolanasdk.NewChainMetadata(
		0,
		mcmProgram,
		mcmSeed,
		proposerAccessController,
		cancellerAccessController,
		bypasserAccessController,
	)
	if err != nil {
		return mcmstypes.ChainMetadata{}, fmt.Errorf("create chain metadata for chain %d: %w", chainSelector, err)
	}

	inspector, err := cldfproposalutils.McmsInspectorForChain(
		e,
		chainSelector,
		cldfproposalutils.WithTimelockAction(input.TimelockAction),
	)
	if err != nil {
		return mcmstypes.ChainMetadata{}, fmt.Errorf("build inspector for chain %d: %w", chainSelector, err)
	}

	opCount, err := inspector.GetOpCount(e.GetContext(), metadata.MCMAddress)
	if err != nil {
		return mcmstypes.ChainMetadata{}, fmt.Errorf("get op count for chain %d: %w", chainSelector, err)
	}
	metadata.StartingOpCount = opCount

	return metadata, nil
}

func requireSolanaChain(e cldf.Environment, chainSelector uint64) error {
	if _, ok := e.BlockChains.SolanaChains()[chainSelector]; !ok {
		return fmt.Errorf("chain %d not found", chainSelector)
	}

	return nil
}

func solanaAddressRef(e cldf.Environment, chainSelector uint64, contractType cldf.ContractType, qualifier string) (datastore.AddressRef, error) {
	if e.DataStore == nil {
		return datastore.AddressRef{}, fmt.Errorf("datastore not available for chain %d", chainSelector)
	}

	return datastore.FindUniqueRef(e.DataStore.Addresses(), datastore.AddressRef{
		ChainSelector: chainSelector,
		Type:          datastore.ContractType(contractType),
		Qualifier:     qualifier,
	})
}

func solanaMCMSRef(e cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) (datastore.AddressRef, error) {
	contractType, err := solanaMCMContractType(input.TimelockAction)
	if err != nil {
		return datastore.AddressRef{}, err
	}

	return solanaAddressRef(e, chainSelector, contractType, input.Qualifier)
}

func solanaMCMContractType(action mcmstypes.TimelockAction) (cldf.ContractType, error) {
	switch action {
	case mcmstypes.TimelockActionSchedule, "":
		return mcmscontracts.ProposerManyChainMultisig, nil
	case mcmstypes.TimelockActionCancel:
		return mcmscontracts.CancellerManyChainMultisig, nil
	case mcmstypes.TimelockActionBypass:
		return mcmscontracts.BypasserManyChainMultisig, nil
	default:
		return "", fmt.Errorf("invalid MCMS action %q", action)
	}
}

func solanaAccessControllerPubkey(e cldf.Environment, chainSelector uint64, contractType cldf.ContractType, qualifier string) (solanago.PublicKey, error) {
	ref, err := solanaAddressRef(e, chainSelector, contractType, qualifier)
	if err != nil {
		return solanago.PublicKey{}, fmt.Errorf("resolve %s for chain %d: %w", contractType, chainSelector, err)
	}

	pubkey, err := solanago.PublicKeyFromBase58(ref.Address)
	if err != nil {
		return solanago.PublicKey{}, fmt.Errorf("parse %s address for chain %d: %w", contractType, chainSelector, err)
	}

	return pubkey, nil
}
