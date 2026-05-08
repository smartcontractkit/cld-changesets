package seqs

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"

	"github.com/smartcontractkit/mcms/sdk"
	mcmsTypes "github.com/smartcontractkit/mcms/types"

	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	opsevm "github.com/smartcontractkit/cld-changesets/pkg/family/evm/operations"
)

type SeqDeployMCMWithConfigInput struct {
	ContractType   cldf.ContractType                 `json:"contractType"`
	MCMConfig      mcmsTypes.Config                  `json:"mcmConfig"`
	ChainSelector  uint64                            `json:"chainSelector"`
	GasBoostConfig *cldfproposalutils.GasBoostConfig `json:"gasBoostConfig"`
	Qualifier      *string                           `json:"qualifier"`
}

type SeqDeployMCMWithConfigOutput struct {
	Address common.Address `json:"address"`
}

var SeqEVMDeployMCMWithConfig = operations.NewSequence(
	"seq-deploy-mcm-with-config",
	semver.MustParse("1.0.0"),
	"Deploys MCM contract & sets config",
	func(b operations.Bundle, deps cldf_evm.Chain, in SeqDeployMCMWithConfigInput) (opsevm.EVMDeployOutput, error) {
		// Deploy MCM contract
		var deployReport operations.Report[opsevm.EVMDeployInput[any], opsevm.EVMDeployOutput]
		var deployErr error
		switch in.ContractType {
		case mcmscontracts.BypasserManyChainMultisig:
			deployReport, deployErr = operations.ExecuteOperation(b, opsevm.OpEVMDeployBypasserMCM, deps, opsevm.EVMDeployInput[any]{
				ChainSelector: in.ChainSelector,
				Qualifier:     in.Qualifier,
			}, opsevm.RetryDeploymentWithGasBoost[any](in.GasBoostConfig))
		case mcmscontracts.ProposerManyChainMultisig:
			deployReport, deployErr = operations.ExecuteOperation(b, opsevm.OpEVMDeployProposerMCM, deps, opsevm.EVMDeployInput[any]{
				ChainSelector: in.ChainSelector,
				Qualifier:     in.Qualifier,
			}, opsevm.RetryDeploymentWithGasBoost[any](in.GasBoostConfig))
		case mcmscontracts.CancellerManyChainMultisig:
			deployReport, deployErr = operations.ExecuteOperation(b, opsevm.OpEVMDeployCancellerMCM, deps, opsevm.EVMDeployInput[any]{
				ChainSelector: in.ChainSelector,
				Qualifier:     in.Qualifier,
			}, opsevm.RetryDeploymentWithGasBoost[any](in.GasBoostConfig))
		default:
			return opsevm.EVMDeployOutput{}, fmt.Errorf("unsupported contract type for seq-deploy-mcm-with-config: %s", in.ContractType)
		}
		if deployErr != nil {
			return opsevm.EVMDeployOutput{}, fmt.Errorf("failed to deploy %s: %w", in.ContractType, deployErr)
		}

		// Set config
		groupQuorums, groupParents, signerAddresses, signerGroups, err := sdk.ExtractSetConfigInputs(&in.MCMConfig)
		if err != nil {
			return opsevm.EVMDeployOutput{}, err
		}
		_, err = operations.ExecuteOperation(b, opsevm.OpEVMSetConfigMCM,
			deps,
			opsevm.EVMCallInput[opsevm.OpEVMSetConfigMCMInput]{
				ChainSelector: in.ChainSelector,
				Address:       deployReport.Output.Address,
				NoSend:        false,
				CallInput: opsevm.OpEVMSetConfigMCMInput{
					SignerAddresses: signerAddresses,
					SignerGroups:    signerGroups,
					GroupQuorums:    groupQuorums,
					GroupParents:    groupParents,
				},
			},
			opsevm.RetryCallWithGasBoost[opsevm.OpEVMSetConfigMCMInput](in.GasBoostConfig),
		)
		if err != nil {
			return opsevm.EVMDeployOutput{}, err
		}

		return deployReport.Output, nil
	},
)
