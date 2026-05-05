package operations

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/Masterminds/semver/v3"
	"github.com/gagliardetto/solana-go"
	chainsel "github.com/smartcontractkit/chain-selectors"
	mcmslib "github.com/smartcontractkit/mcms"
	mcmsevmsdk "github.com/smartcontractkit/mcms/sdk/evm"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"

	mcmscontract "github.com/smartcontractkit/cld-changesets/mcms/proposeutils"
	evmstate "github.com/smartcontractkit/cld-changesets/pkg/family/evm"
	solanastate "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

// FireDrillInput is JSON-serializable input for the MCMS signing fire-drill proposal operation.
type FireDrillInput struct {
	TimelockCfg cldfproposalutils.TimelockConfig `json:"timelockCfg"`
	Selectors   []uint64                         `json:"selectors,omitempty"`
}

// FireDrillDeps holds non-serializable dependencies for the fire-drill operation.
type FireDrillDeps struct {
	Environment cldf.Environment
}

// FireDrillOutput is the serializable result of building the fire-drill proposal.
type FireDrillOutput struct {
	Proposal mcmslib.TimelockProposal `json:"proposal"`
}

// BuildMCMSFiredrillProposalOp builds a noop MCMS timelock proposal covering the configured chains.
// Use [fwops.WithForceExecute] at the call site so repeated drills record fresh proposals under identical inputs.
var BuildMCMSFiredrillProposalOp = fwops.NewOperation(
	"mcms-firedrill-proposal",
	semver.MustParse("1.0.0"),
	"Build noop MCMS timelock proposal for signing fire drills",
	buildMCMSFiredrillProposal,
)

func buildMCMSFiredrillProposal(_ fwops.Bundle, deps FireDrillDeps, input FireDrillInput) (FireDrillOutput, error) {
	e := deps.Environment

	allSelectors := input.Selectors
	if len(allSelectors) == 0 {
		solSelectors := e.BlockChains.ListChainSelectors(cldf_chain.WithFamily(chainsel.FamilySolana))
		evmSelectors := e.BlockChains.ListChainSelectors(cldf_chain.WithFamily(chainsel.FamilyEVM))
		allSelectors = append(append(allSelectors, solSelectors...), evmSelectors...)
	}
	if len(allSelectors) == 0 {
		return FireDrillOutput{}, errors.New("no chain selectors resolved for MCMS fire drill")
	}

	operations := make([]mcmstypes.BatchOperation, 0, len(allSelectors))
	timelocks := make(map[uint64]string, len(allSelectors))
	mcmAddresses := make(map[uint64]string, len(allSelectors))

	inspectors, inspErr := cldfproposalutils.McmsInspectors(e)
	if inspErr != nil {
		return FireDrillOutput{}, inspErr
	}

	for _, selector := range allSelectors {
		family, famErr := chainsel.GetSelectorFamily(selector)
		if famErr != nil {
			return FireDrillOutput{}, famErr
		}

		switch family {
		case chainsel.FamilyEVM:
			evmChain, ok := e.BlockChains.EVMChains()[selector]
			if !ok {
				return FireDrillOutput{}, fmt.Errorf("evm chain %d not found in environment", selector)
			}

			addresses, err := e.ExistingAddresses.AddressesForChain(selector) //nolint:staticcheck // SA1019: AddressBook compatibility
			if err != nil {
				return FireDrillOutput{}, err
			}

			st, err := evmstate.MaybeLoadMCMSWithTimelockChainState(evmChain, addresses)
			if err != nil {
				return FireDrillOutput{}, err
			}

			timelocks[selector] = st.Timelock.Address().String()

			mcmAddress, err := input.TimelockCfg.MCMBasedOnAction(st)
			if err != nil {
				return FireDrillOutput{}, err
			}

			mcmAddresses[selector] = mcmAddress.Address().String()

			tx, err := buildNoOPEVM(st)
			if err != nil {
				return FireDrillOutput{}, err
			}

			operations = append(operations, mcmstypes.BatchOperation{
				ChainSelector: mcmstypes.ChainSelector(selector),
				Transactions:  []mcmstypes.Transaction{tx},
			})

		case chainsel.FamilySolana:
			solChain, ok := e.BlockChains.SolanaChains()[selector]
			if !ok {
				return FireDrillOutput{}, fmt.Errorf("solana chain %d not found in environment", selector)
			}

			addresses, err := e.ExistingAddresses.AddressesForChain(selector) //nolint:staticcheck // SA1019
			if err != nil {
				return FireDrillOutput{}, err
			}

			st, err := solanastate.MaybeLoadMCMSWithTimelockChainState(solChain, addresses)
			if err != nil {
				return FireDrillOutput{}, err
			}

			timelocks[selector] = mcmssolanasdk.ContractAddress(st.TimelockProgram, mcmssolanasdk.PDASeed(st.TimelockSeed))

			mcmAddr, err := input.TimelockCfg.MCMBasedOnActionSolana(st)
			if err != nil {
				return FireDrillOutput{}, err
			}

			mcmAddresses[selector] = mcmAddr

			tx, err := buildNoOPSolana()
			if err != nil {
				return FireDrillOutput{}, err
			}

			operations = append(operations, mcmstypes.BatchOperation{
				ChainSelector: mcmstypes.ChainSelector(selector),
				Transactions:  []mcmstypes.Transaction{tx},
			})

		default:
			return FireDrillOutput{}, fmt.Errorf("unsupported chain family for selector %d", selector)
		}
	}

	prop, err := mcmscontract.BuildProposalFromBatchesV2(
		e,
		timelocks,
		mcmAddresses,
		inspectors,
		operations,
		"firedrill proposal",
		input.TimelockCfg,
	)
	if err != nil {
		return FireDrillOutput{}, err
	}

	return FireDrillOutput{Proposal: *prop}, nil
}

// buildNoOPEVM builds a dummy tx that transfers 0 to the RBACTimelock (receive path).
func buildNoOPEVM(st *evmstate.MCMSWithTimelockState) (mcmstypes.Transaction, error) {
	if st == nil || st.Timelock == nil {
		return mcmstypes.Transaction{}, errors.New("timelock binding is required for noop EVM fire drill transaction")
	}

	return mcmsevmsdk.NewTransaction(
		st.Timelock.Address(),
		[]byte{},
		big.NewInt(0),
		"FireDrillNoop",
		nil,
	), nil
}

// buildNoOPSolana builds a dummy transaction that invokes the memo program.
func buildNoOPSolana() (mcmstypes.Transaction, error) {
	contractID := solana.MemoProgramID
	memo := []byte("noop")

	tx, err := mcmssolanasdk.NewTransaction(
		contractID.String(),
		memo,
		big.NewInt(0),
		[]*solana.AccountMeta{},
		"Memo",
		[]string{},
	)
	if err != nil {
		return mcmstypes.Transaction{}, err
	}

	return tx, nil
}
