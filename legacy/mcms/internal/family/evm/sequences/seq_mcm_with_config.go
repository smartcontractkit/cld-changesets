package sequences

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"

	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/mcms/sdk"
	mcmsTypes "github.com/smartcontractkit/mcms/types"

	opevm "github.com/smartcontractkit/cld-changesets/legacy/mcms/internal/family/evm/operations"
	oputils "github.com/smartcontractkit/cld-changesets/pkg/family/evm/operations"
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

var SeqDeployMCMWithConfig = operations.NewSequence(
	"seq-deploy-mcm-with-config",
	semver.MustParse("1.0.0"),
	"Deploys MCM contract & sets config",
	func(b operations.Bundle, deps cldf_evm.Chain, in SeqDeployMCMWithConfigInput) (oputils.EVMDeployOutput, error) {
		// Deploy MCM contract
		var deployReport operations.Report[oputils.EVMDeployInput[any], oputils.EVMDeployOutput]
		var deployErr error
		switch in.ContractType {
		case mcmscontracts.BypasserManyChainMultisig:
			deployReport, deployErr = operations.ExecuteOperation(b, opevm.OpDeployBypasserMCM, deps, oputils.EVMDeployInput[any]{
				ChainSelector: in.ChainSelector,
				Qualifier:     in.Qualifier,
			}, oputils.RetryDeploymentWithGasBoost[any](in.GasBoostConfig))
		case mcmscontracts.ProposerManyChainMultisig:
			deployReport, deployErr = operations.ExecuteOperation(b, opevm.OpDeployProposerMCM, deps, oputils.EVMDeployInput[any]{
				ChainSelector: in.ChainSelector,
				Qualifier:     in.Qualifier,
			}, oputils.RetryDeploymentWithGasBoost[any](in.GasBoostConfig))
		case mcmscontracts.CancellerManyChainMultisig:
			deployReport, deployErr = operations.ExecuteOperation(b, opevm.OpDeployCancellerMCM, deps, oputils.EVMDeployInput[any]{
				ChainSelector: in.ChainSelector,
				Qualifier:     in.Qualifier,
			}, oputils.RetryDeploymentWithGasBoost[any](in.GasBoostConfig))
		default:
			return oputils.EVMDeployOutput{}, fmt.Errorf("unsupported contract type for seq-deploy-mcm-with-config: %s", in.ContractType)
		}
		if deployErr != nil {
			return oputils.EVMDeployOutput{}, fmt.Errorf("failed to deploy %s: %w", in.ContractType, deployErr)
		}

		// Set config
		groupQuorums, groupParents, signerAddresses, signerGroups, err := sdk.ExtractSetConfigInputs(&in.MCMConfig)
		if err != nil {
			return oputils.EVMDeployOutput{}, err
		}
		_, err = operations.ExecuteOperation(b, opevm.OpEVMSetConfigMCM,
			deps,
			oputils.EVMCallInput[opevm.OpEVMSetConfigMCMInput]{
				ChainSelector: in.ChainSelector,
				Address:       deployReport.Output.Address,
				NoSend:        false,
				CallInput: opevm.OpEVMSetConfigMCMInput{
					SignerAddresses: signerAddresses,
					SignerGroups:    signerGroups,
					GroupQuorums:    groupQuorums,
					GroupParents:    groupParents,
				},
			},
			oputils.RetryCallWithGasBoost[opevm.OpEVMSetConfigMCMInput](in.GasBoostConfig),
		)
		if err != nil {
			return oputils.EVMDeployOutput{}, err
		}

		return deployReport.Output, nil
	},
)
