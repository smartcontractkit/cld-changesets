package changesets

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"
	"github.com/smartcontractkit/mcms"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

const defaultProposalValidFor = 24 * time.Hour

var _ cldf.ChangeSetV2[TransferLinkTokenInput] = TransferLinkTokenChangeset{}

// TransferLinkTokenInput holds the parameters for an MCMS-gated LINK token transfer.
type TransferLinkTokenInput struct {
	ChainSelector uint64             `json:"chainSelector"        yaml:"chainSelector"`
	To            common.Address     `json:"to"                   yaml:"to"`
	Amount        *big.Int           `json:"amount"               yaml:"amount"`
	TimelockDelay mcmstypes.Duration `json:"timelockDelay"        yaml:"timelockDelay"`
	Qualifier     string             `json:"qualifier,omitempty"  yaml:"qualifier,omitempty"`
	ValidUntil    *time.Time         `json:"validUntil,omitempty" yaml:"validUntil,omitempty"`
}

// TransferLinkTokenChangeset creates an MCMS Timelock proposal to transfer LINK tokens
// from a Timelock-controlled address to a recipient.
type TransferLinkTokenChangeset struct{}

func (TransferLinkTokenChangeset) VerifyPreconditions(e cldf.Environment, input TransferLinkTokenInput) error {
	if input.ChainSelector == 0 {
		return errors.New("chain selector must be non-zero")
	}
	if input.To == (common.Address{}) {
		return errors.New("recipient address must be non-zero")
	}
	if input.Amount == nil || input.Amount.Sign() <= 0 {
		return errors.New("amount must be positive")
	}
	if e.DataStore == nil {
		return errors.New("datastore is required")
	}

	if _, err := e.DataStore.Addresses().Get(datastore.NewAddressRefKey(
		input.ChainSelector,
		datastore.ContractType(linkcontracts.LinkToken),
		&semvers.V1_0_0,
		input.Qualifier,
	)); err != nil {
		return fmt.Errorf("no LinkToken address found for chain selector %d: %w", input.ChainSelector, err)
	}
	if _, err := e.DataStore.Addresses().Get(datastore.NewAddressRefKey(
		input.ChainSelector,
		datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		&semvers.V1_0_0,
		input.Qualifier,
	)); err != nil {
		return fmt.Errorf("no ProposerManyChainMultisig address found for chain selector %d: %w", input.ChainSelector, err)
	}
	if _, err := e.DataStore.Addresses().Get(datastore.NewAddressRefKey(
		input.ChainSelector,
		datastore.ContractType(mcmscontracts.RBACTimelock),
		&semvers.V1_0_0,
		input.Qualifier,
	)); err != nil {
		return fmt.Errorf("no RBACTimelock address found for chain selector %d: %w", input.ChainSelector, err)
	}

	return nil
}

func (TransferLinkTokenChangeset) Apply(e cldf.Environment, input TransferLinkTokenInput) (cldf.ChangesetOutput, error) {
	chain, ok := e.BlockChains.EVMChains()[input.ChainSelector]
	if !ok {
		return cldf.ChangesetOutput{}, fmt.Errorf("chain not found in environment: %d", input.ChainSelector)
	}

	linkRef, err := e.DataStore.Addresses().Get(datastore.NewAddressRefKey(
		input.ChainSelector,
		datastore.ContractType(linkcontracts.LinkToken),
		&semvers.V1_0_0,
		input.Qualifier,
	))
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("no LinkToken address found for chain selector %d: %w", input.ChainSelector, err)
	}
	proposerRef, err := e.DataStore.Addresses().Get(datastore.NewAddressRefKey(
		input.ChainSelector,
		datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		&semvers.V1_0_0,
		input.Qualifier,
	))
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("no ProposerManyChainMultisig address found for chain selector %d: %w", input.ChainSelector, err)
	}
	timelockRef, err := e.DataStore.Addresses().Get(datastore.NewAddressRefKey(
		input.ChainSelector,
		datastore.ContractType(mcmscontracts.RBACTimelock),
		&semvers.V1_0_0,
		input.Qualifier,
	))
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("no RBACTimelock address found for chain selector %d: %w", input.ChainSelector, err)
	}

	token, err := link_token.NewLinkToken(common.HexToAddress(linkRef.Address), chain.Client)
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("failed to instantiate link token contract: %w", err)
	}

	tx, err := token.Transfer(cldf.SimTransactOpts(), input.To, input.Amount)
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("failed to simulate link token transfer: %w", err)
	}

	validUntil := time.Now().Add(defaultProposalValidFor)
	if input.ValidUntil != nil {
		validUntil = *input.ValidUntil
	}

	unixTs := validUntil.Unix()
	if unixTs < 0 || unixTs > math.MaxUint32 {
		return cldf.ChangesetOutput{}, fmt.Errorf("validUntil %s is out of uint32 unix range", validUntil.Format(time.RFC3339))
	}

	proposal, err := mcms.NewTimelockProposalBuilder().
		SetAction(mcmstypes.TimelockActionSchedule).
		SetTimelockAddresses(map[mcmstypes.ChainSelector]string{
			mcmstypes.ChainSelector(input.ChainSelector): timelockRef.Address,
		}).
		SetVersion("v1").
		SetValidUntil(uint32(unixTs)).
		SetChainMetadata(map[mcmstypes.ChainSelector]mcmstypes.ChainMetadata{
			mcmstypes.ChainSelector(input.ChainSelector): {
				StartingOpCount: 0,
				MCMAddress:      proposerRef.Address,
			},
		}).
		SetOperations([]mcmstypes.BatchOperation{
			{
				ChainSelector: mcmstypes.ChainSelector(input.ChainSelector),
				Transactions: []mcmstypes.Transaction{
					{
						OperationMetadata: mcmstypes.OperationMetadata{
							ContractType: "LinkToken",
							Tags:         []string{"transfer"},
						},
						To:               token.Address().Hex(),
						Data:             tx.Data(),
						AdditionalFields: json.RawMessage(`{"value": 0}`),
					},
				},
			},
		}).
		SetDelay(input.TimelockDelay).
		Build()
	if err != nil {
		return cldf.ChangesetOutput{}, fmt.Errorf("failed to build transfer proposal: %w", err)
	}

	return cldf.ChangesetOutput{
		MCMSTimelockProposals: []mcms.TimelockProposal{*proposal},
	}, nil
}
