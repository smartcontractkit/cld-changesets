package evmdeploy

import (
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"

	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	cldfevm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/mcms/sdk"
	evmMcms "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/mcmsrole"
	mcmops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/many_chain_multi_sig"
	timelockops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/rbac_timelock"
)

type deployMCMWithConfigInput struct {
	ContractType   cldf.ContractType                 `json:"contractType"`
	MCMConfig      mcmstypes.Config                  `json:"mcmConfig"`
	Qualifier      *string                           `json:"qualifier"`
	GasBoostConfig *cldfproposalutils.GasBoostConfig `json:"gasBoostConfig"`
}

var seqDeployMCMWithConfig = operations.NewSequence(
	"seq-deploy-mcm-with-config",
	semver.MustParse("1.0.0"),
	"Deploys an MCM contract and sets its signer config",
	func(b operations.Bundle, chain cldfevm.Chain, in deployMCMWithConfigInput) (cldfdatastore.AddressRef, error) {
		typeAndVersion, err := mcmTypeVersion(in.ContractType)
		if err != nil {
			return cldfdatastore.AddressRef{}, err
		}

		deployReport, err := operations.ExecuteOperation(
			b,
			mcmops.Deploy,
			chain,
			opscontract.DeployInput[mcmops.ConstructorArgs]{
				TypeAndVersion: typeAndVersion,
				Qualifier:      in.Qualifier,
				Args:           mcmops.ConstructorArgs{},
			},
			retryDeployWithGasBoost[mcmops.ConstructorArgs](in.GasBoostConfig),
			chainIdempotencyKey[opscontract.DeployInput[mcmops.ConstructorArgs], cldfevm.Chain](chain),
		)
		if err != nil {
			return cldfdatastore.AddressRef{}, fmt.Errorf("deploy %s: %w", in.ContractType, err)
		}

		groupQuorums, groupParents, signerAddresses, signerGroups, err := sdk.ExtractSetConfigInputs(&in.MCMConfig)
		if err != nil {
			return cldfdatastore.AddressRef{}, err
		}

		mcm, err := bindings.NewManyChainMultiSig(common.HexToAddress(deployReport.Output.Address), chain.Client)
		if err != nil {
			return cldfdatastore.AddressRef{}, fmt.Errorf("bind %s: %w", in.ContractType, err)
		}

		_, err = operations.ExecuteOperation(
			b,
			mcmops.NewWriteSetConfig(mcm),
			chain,
			opscontract.FunctionInput[mcmops.SetConfigArgs]{
				Args: mcmops.SetConfigArgs{
					SignerAddresses: signerAddresses,
					SignerGroups:    signerGroups,
					GroupQuorums:    groupQuorums,
					GroupParents:    groupParents,
					ClearRoot:       false,
				},
			},
			retryWriteWithGasBoost[mcmops.SetConfigArgs](in.GasBoostConfig),
			outputAddressIdempotencyKey[opscontract.FunctionInput[mcmops.SetConfigArgs], cldfevm.Chain](chain, deployReport.Output.Address),
		)
		if err != nil {
			return cldfdatastore.AddressRef{}, fmt.Errorf("set config on %s: %w", in.ContractType, err)
		}

		return deployReport.Output, nil
	},
)

func mcmTypeVersion(contractType cldf.ContractType) (cldf.TypeAndVersion, error) {
	switch contractType {
	case mcmscontracts.ProposerManyChainMultisig:
		return mcmops.ProposerManyChainMultiSigTypeAndVersion, nil
	case mcmscontracts.BypasserManyChainMultisig:
		return mcmops.BypasserManyChainMultiSigTypeAndVersion, nil
	case mcmscontracts.CancellerManyChainMultisig:
		return mcmops.CancellerManyChainMultiSigTypeAndVersion, nil
	default:
		return cldf.TypeAndVersion{}, fmt.Errorf("unsupported contract type for seq-deploy-mcm-with-config: %s", contractType)
	}
}

type timelockRoleGrant struct {
	Role      common.Hash
	Name      string
	Addresses []common.Address
}

type grantRolesTimelockInput struct {
	Timelock       common.Address                    `json:"timelock"`
	RoleGrants     []timelockRoleGrant               `json:"roleGrants"`
	GasBoostConfig *cldfproposalutils.GasBoostConfig `json:"gasBoostConfig"`
}

var seqGrantRolesTimelock = operations.NewSequence(
	"seq-grant-roles-timelock",
	semver.MustParse("1.0.0"),
	"Grants appropriate roles to MCMS contracts on the RBAC timelock",
	func(b operations.Bundle, chain cldfevm.Chain, in grantRolesTimelockInput) (struct{}, error) {
		timelockInspector := evmMcms.NewTimelockInspector(chain.Client)

		timelock, err := bindings.NewRBACTimelock(in.Timelock, chain.Client)
		if err != nil {
			return struct{}{}, err
		}

		grantRole := timelockops.NewWriteGrantRole(timelock)

		for _, roleAndAddress := range in.RoleGrants {
			var addressesInInspector []string
			switch roleAndAddress.Role {
			case mcmsrole.ProposerRole.ID:
				addressesInInspector, err = timelockInspector.GetProposers(b.GetContext(), in.Timelock.Hex())
			case mcmsrole.CancellerRole.ID:
				addressesInInspector, err = timelockInspector.GetCancellers(b.GetContext(), in.Timelock.Hex())
			case mcmsrole.BypasserRole.ID:
				addressesInInspector, err = timelockInspector.GetBypassers(b.GetContext(), in.Timelock.Hex())
			case mcmsrole.ExecutorRole.ID:
				addressesInInspector, err = timelockInspector.GetExecutors(b.GetContext(), in.Timelock.Hex())
			case mcmsrole.AdminRole.ID:
				addressesInInspector = []string{}
			default:
				return struct{}{}, fmt.Errorf("unsupported timelock role %q", roleAndAddress.Name)
			}
			if err != nil {
				b.Logger.Errorw("Failed to get addresses from timelock inspector",
					"chain", chain.String(),
					"timelock", in.Timelock.Hex(),
					"role", roleAndAddress.Name,
					"err", err,
				)

				return struct{}{}, err
			}

			for _, addressToGrantRole := range roleAndAddress.Addresses {
				if slices.Contains(addressesInInspector, addressToGrantRole.Hex()) {
					continue
				}

				report, err := operations.ExecuteOperation(
					b,
					grantRole,
					chain,
					opscontract.FunctionInput[timelockops.GrantRoleArgs]{
						Args: timelockops.GrantRoleArgs{
							Role:    roleAndAddress.Role,
							Account: addressToGrantRole,
						},
					},
					retryWriteWithGasBoost[timelockops.GrantRoleArgs](in.GasBoostConfig),
					outputAddressIdempotencyKey[opscontract.FunctionInput[timelockops.GrantRoleArgs], cldfevm.Chain](chain, in.Timelock.Hex()),
				)
				if err != nil {
					b.Logger.Errorw("Failed to grant role",
						"chain", chain.String(),
						"timelock", in.Timelock.Hex(),
						"role", roleAndAddress.Name,
						"address", addressToGrantRole.Hex(),
					)

					return struct{}{}, err
				}

				if report.Output.Executed() {
					b.Logger.Infow("Role granted",
						"role", roleAndAddress.Name,
						"chain", chain.String(),
						"timelock", in.Timelock.Hex(),
						"address", addressToGrantRole.Hex(),
					)
				}
			}
		}

		return struct{}{}, nil
	},
)
