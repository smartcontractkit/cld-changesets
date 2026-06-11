package sequences

import (
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/burn_mint_erc677"
	mcmsTypes "github.com/smartcontractkit/mcms/types"

	opevm "github.com/smartcontractkit/cld-changesets/legacy/mcms/internal/family/evm/operations"
	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
)

type SeqTransferToMCMSWithTimelockDeps struct {
	Chain evm.Chain
}

type SeqTransferToMCMSWithTimelockInput struct {
	ChainSelector uint64           `json:"chainSelector"`
	Timelock      common.Address   `json:"timelock"`
	Contracts     []common.Address `json:"contracts"`

	OnlyAcceptOwnership bool `json:"onlyacceptownership"`
}

type SeqTransferToMCMSWithTimelockOutput struct {
	OpsMcms []mcmsTypes.Transaction `json:"opsMcms"`
}

var SeqTransferToMCMSWithTimelock = operations.NewSequence(
	"seq-transfer-to-mcms-with-timelock-v2",
	semver.MustParse("1.0.0"),
	"Transfers ownership to the Timelock contract",
	func(b operations.Bundle, deps SeqTransferToMCMSWithTimelockDeps, in SeqTransferToMCMSWithTimelockInput) (SeqTransferToMCMSWithTimelockOutput, error) {
		var (
			mcsmOps []mcmsTypes.Transaction
		)

		for _, contract := range in.Contracts {
			// Just using the ownership interface.
			// Already validated is ownable.
			owner, c, err := LoadOwnableContract(contract, deps.Chain.Client)
			if err != nil {
				b.Logger.Errorf("failed to load ownable contract %s: %v", contract.Hex(), err)
				return SeqTransferToMCMSWithTimelockOutput{}, fmt.Errorf("error loading ownable contract %s: %w", contract.Hex(), err)
			}

			if owner.String() == in.Timelock.Hex() {
				// Already owned by timelock.
				b.Logger.Infof("contract %s already owned by timelock", contract)
				continue
			}

			if !in.OnlyAcceptOwnership {
				// Transfer Ownership
				_, err = operations.ExecuteOperation(b, opevm.OpTransferOwnership,
					opevm.OpOwnershipDeps{
						Chain:    deps.Chain,
						OwnableC: c,
					},
					opevm.OpTransferOwnershipInput{
						ChainSelector:   in.ChainSelector,
						TimelockAddress: in.Timelock,
						Address:         contract,
					},
				)

				if err != nil {
					return SeqTransferToMCMSWithTimelockOutput{}, err
				}
			}

			// Accept Ownership
			opReport, err := operations.ExecuteOperation(b, opevm.OpAcceptOwnership,
				opevm.OpOwnershipDeps{
					Chain:    deps.Chain,
					OwnableC: c,
				},
				opevm.OpTransferOwnershipInput{
					ChainSelector:   in.ChainSelector,
					TimelockAddress: in.Timelock,
					Address:         contract,
				},
			)
			if err != nil {
				return SeqTransferToMCMSWithTimelockOutput{}, err
			}

			mcsmOps = append(mcsmOps, mcmsTypes.Transaction{
				To:               contract.Hex(),
				Data:             opReport.Output.Tx.Data(),
				AdditionalFields: json.RawMessage(`{"value": 0}`), // JSON-encoded `{"value": 0}`
			})
		}

		return SeqTransferToMCMSWithTimelockOutput{OpsMcms: mcsmOps}, nil
	},
)

// TODO: convert this to an OP
func LoadOwnableContract(addr common.Address, client bind.ContractBackend) (common.Address, evmstate.Ownable, error) {
	// Just using the ownership interface from here.
	c, err := burn_mint_erc677.NewBurnMintERC677(addr, client)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to create contract: %w", err)
	}
	owner, err := c.Owner(nil)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("failed to get owner of contract %s: %w", c.Address(), err)
	}

	return owner, c, nil
}
