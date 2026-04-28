package mcms

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	mcmslib "github.com/smartcontractkit/mcms"
	mcmschainwrappers "github.com/smartcontractkit/mcms/chainwrappers"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	"github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/pkg/family/solana"

	cldf_adapters "github.com/smartcontractkit/chainlink-deployments-framework/chain/mcms/adapters"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
)

const (
	DefaultValidUntil = 72 * time.Hour
)

type ChainMetadata map[uint64]map[string]any

func (c *ChainMetadata) Set(chainSelector uint64, key string, value any) *ChainMetadata {
	_, exists := (*c)[chainSelector]
	if !exists {
		(*c)[chainSelector] = make(map[string]any)
	}

	(*c)[chainSelector][key] = value

	return c
}

type BuildProposalOption func(*buildProposalOptions)

type buildProposalOptions struct {
	chainMetadata ChainMetadata
}

func WithChainMetadata(chainMetadata ChainMetadata) BuildProposalOption {
	return func(opts *buildProposalOptions) {
		opts.chainMetadata = chainMetadata
	}
}

// BuildProposalFromBatchesV2 uses the new MCMS library which replaces the implementation in BuildProposalFromBatches.
func BuildProposalFromBatchesV2(
	e cldf.Environment,
	timelockAddressPerChain map[uint64]string,
	mcmsAddressPerChain map[uint64]string,
	inspectorPerChain map[uint64]mcmssdk.Inspector, // optional
	batches []types.BatchOperation,
	description string,
	mcmsCfg cldfproposalutils.TimelockConfig,
	opts ...BuildProposalOption,
) (*mcmslib.TimelockProposal, error) {
	buildOptions := buildProposalOptions{}
	for _, opt := range opts {
		opt(&buildOptions)
	}

	// default to schedule if not set, this is to be consistent with the old implementation
	// and to avoid breaking changes
	if mcmsCfg.MCMSAction == "" {
		mcmsCfg.MCMSAction = types.TimelockActionSchedule
	}
	if len(batches) == 0 {
		return nil, errors.New("no operations in batch")
	}

	chains := mapset.NewSet[uint64]()
	for _, op := range batches {
		chains.Add(uint64(op.ChainSelector))
	}
	tlsPerChainID := make(map[types.ChainSelector]string)
	for chainID, tl := range timelockAddressPerChain {
		tlsPerChainID[types.ChainSelector(chainID)] = tl
	}
	mcmsMd, err := buildProposalMetadataV2(e, chains.ToSlice(), inspectorPerChain, mcmsAddressPerChain,
		mcmsCfg.MCMSAction, buildOptions.chainMetadata)
	if err != nil {
		return nil, err
	}

	proposalDuration := DefaultValidUntil
	if mcmsCfg.ValidDuration != nil {
		proposalDuration = mcmsCfg.ValidDuration.Duration
	}
	validUntil := time.Now().Add(proposalDuration).Unix()

	builder := mcmslib.NewTimelockProposalBuilder()
	builder.
		SetVersion("v1").
		SetAction(mcmsCfg.MCMSAction).
		//nolint:gosec // G115
		SetValidUntil(uint32(validUntil)).
		SetDescription(description).
		SetDelay(types.NewDuration(mcmsCfg.MinDelay)).
		SetOverridePreviousRoot(mcmsCfg.OverrideRoot).
		SetChainMetadata(mcmsMd).
		SetTimelockAddresses(tlsPerChainID).
		SetOperations(batches)

	build, err := builder.Build()
	if err != nil {
		return nil, err
	}

	return build, nil
}

func buildProposalMetadataV2(
	env cldf.Environment,
	chainSelectors []uint64,
	inspectorPerChain map[uint64]mcmssdk.Inspector, // optional
	mcmAddresses map[uint64]string, // can be proposer, canceller or bypasser
	mcmsAction types.TimelockAction,
	additionalChainMetadata ChainMetadata,
) (map[types.ChainSelector]types.ChainMetadata, error) {
	proposalChainMetadata := make(map[types.ChainSelector]types.ChainMetadata)

	if len(additionalChainMetadata) == 0 {
		additionalChainMetadata = make(ChainMetadata)
	}

	for _, selector := range chainSelectors {
		mcmAddress, ok := mcmAddresses[selector]
		if !ok {
			return nil, fmt.Errorf("missing mcm address for chain %d", selector)
		}

		chainID := types.ChainSelector(selector)
		family, err := chain_selectors.GetSelectorFamily(selector)
		if err != nil {
			return nil, fmt.Errorf("failed to get family for chain %d: %w", selector, err)
		}

		switch family {
		case chain_selectors.FamilySolana:
			solanaState, err := solana.GetState(env, selector)
			if err != nil {
				return nil, err
			}

			var instanceSeed mcmssolanasdk.PDASeed
			switch mcmsAction {
			case types.TimelockActionSchedule:
				instanceSeed = mcmssolanasdk.PDASeed(solanaState.ProposerMcmSeed)
			case types.TimelockActionCancel:
				instanceSeed = mcmssolanasdk.PDASeed(solanaState.CancellerMcmSeed)
			case types.TimelockActionBypass:
				instanceSeed = mcmssolanasdk.PDASeed(solanaState.BypasserMcmSeed)
			default:
				return nil, fmt.Errorf("invalid MCMS action %s", mcmsAction)
			}

			proposalChainMetadata[chainID], err = mcmssolanasdk.NewChainMetadata(
				0, // opCount is set later
				solanaState.McmProgram,
				instanceSeed,
				solanaState.ProposerAccessControllerAccount,
				solanaState.CancellerAccessControllerAccount,
				solanaState.BypasserAccessControllerAccount)
			if err != nil {
				return nil, fmt.Errorf("failed to create chain metadata: %w", err)
			}

		case chain_selectors.FamilyAptos:
			role, err := cldfproposalutils.GetAptosRoleFromAction(mcmsAction)
			if err != nil {
				return nil, fmt.Errorf("failed to get role from action: %w", err)
			}
			additionalChainMetadata.Set(selector, "role", role)

			proposalChainMetadata[chainID] = types.ChainMetadata{MCMAddress: mcmAddress}

		default:
			proposalChainMetadata[chainID] = types.ChainMetadata{MCMAddress: mcmAddress}
		}
	}

	if len(inspectorPerChain) == 0 {
		mcmsChains := cldf_adapters.Wrap(env.BlockChains)
		inspectors, err := mcmschainwrappers.BuildInspectors(&mcmsChains, proposalChainMetadata, mcmsAction)
		if err != nil {
			return nil, fmt.Errorf("failed to build inspectors: %w", err)
		}

		inspectorPerChain = make(map[uint64]mcmssdk.Inspector)
		for selector, inspector := range inspectors {
			inspectorPerChain[uint64(selector)] = inspector
		}
	}

	for selector, metadata := range proposalChainMetadata {
		inspector, ok := inspectorPerChain[uint64(selector)]
		if !ok {
			return nil, fmt.Errorf("failed to get inspector for chain %d", selector)
		}

		opCount, err := inspector.GetOpCount(env.GetContext(), metadata.MCMAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to get op count for chain %d: %w", selector, err)
		}
		metadata.StartingOpCount = opCount

		additionalMetadata, exists := additionalChainMetadata[uint64(selector)]
		if exists {
			marshalledAdditionalMetadata, err := json.Marshal(additionalMetadata)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal extra chain metadata for chain %d: %w", selector, err)
			}
			metadata.AdditionalFields = marshalledAdditionalMetadata
		}

		proposalChainMetadata[selector] = metadata
	}

	return proposalChainMetadata, nil
}
