package evmdeploytopology

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/burn_mint_erc677"
	mcmsevmsdk "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/mcmsrole"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	deploycustomtopology "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy-custom-topology"
	callproxyops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/call_proxy"
	mcmops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/many_chain_multi_sig"
	timelockops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/rbac_timelock"
	evmgrantrole "github.com/smartcontractkit/cld-changesets/mcms/evm/grant-role"
	"github.com/smartcontractkit/cld-changesets/mcms/evm/internal/gasboost"
	evmops "github.com/smartcontractkit/cld-changesets/mcms/evm/operations"
	evmsetconfig "github.com/smartcontractkit/cld-changesets/mcms/evm/set-config"
	evmtransferownership "github.com/smartcontractkit/cld-changesets/mcms/evm/transfer-ownership"
)

// seqDeployTopology is the single sequence for this changeset; it calls the
// underlying EVM operations directly instead of composing inner sequences.
var seqDeployTopology = operations.NewSequence(
	"seq-evm-deploy-custom-topology",
	semver.MustParse("0.0.0"),
	"Deploys a custom MCMS topology (MCMs + timelocks, custom roles, optional ownership transfers) on an EVM chain",
	runEVMDeployTopology,
)

func runEVMDeployTopology(
	b operations.Bundle,
	deps deploycustomtopology.Deps,
	in deploycustomtopology.ChainInput,
) (deploycustomtopology.ChainOutput, error) {
	var out deploycustomtopology.ChainOutput

	chain, ok := deps.BlockChains.EVMChains()[in.ChainSelector]
	if !ok {
		return out, fmt.Errorf("EVM chain %d not found in environment", in.ChainSelector)
	}

	extra, err := parseEVMExtraArgs(in.ExtraArgs)
	if err != nil {
		return out, fmt.Errorf("chain %d: %w", in.ChainSelector, err)
	}

	gasBoost := in.Config.GasBoostConfig

	// refToAddr maps each spec Ref (MCM or timelock) to its deployed address so
	// later specs can reference it (e.g. role holders by MCMRef).
	refToAddr := make(map[string]common.Address, len(in.Config.MCMs)+len(in.Config.Timelocks))

	// 1. Deploy MCMs (each with its signer config).
	for _, m := range in.Config.MCMs {
		addr, err := deployMCMWithConfig(b, chain, gasBoost, m)
		if err != nil {
			return out, fmt.Errorf("deploy MCM %q on chain %d: %w", m.Ref, in.ChainSelector, err)
		}
		refToAddr[m.Ref] = addr
		out.Metadata.Addresses = append(out.Metadata.Addresses,
			newAddressRef(in.ChainSelector, addr, m.ContractType, m.Qualifier, m.Label))
	}

	// 2. Deploy and wire each timelock.
	for _, tl := range in.Config.Timelocks {
		err := deployTimelockAndSetRoles(b, chain, in.ChainSelector, gasBoost, extra, tl, refToAddr, &out)
		if err != nil {
			return out, err
		}
	}

	return out, nil
}

// deployMCMWithConfig deploys a single MCM contract and applies its signer
// config via the set-config changeset's operation (no duplication of that
// logic in this package).
func deployMCMWithConfig(
	b operations.Bundle,
	chain cldf_evm.Chain,
	gasBoost *cldfproposalutils.GasBoostConfig,
	spec deploycustomtopology.MCMSpec,
) (common.Address, error) {
	typeAndVersion, err := mcmTypeVersion(spec.ContractType)
	if err != nil {
		return common.Address{}, err
	}

	deployReport, err := operations.ExecuteOperation(
		b,
		mcmops.Deploy,
		chain,
		opscontract.DeployInput[mcmops.ConstructorArgs]{
			TypeAndVersion: typeAndVersion,
			Qualifier:      stringPtrIfNonEmpty(spec.Qualifier),
			Args:           mcmops.ConstructorArgs{},
		},
		gasboost.RetryDeploy[mcmops.ConstructorArgs](gasBoost),
		chainIdempotencyKey[opscontract.DeployInput[mcmops.ConstructorArgs], cldf_evm.Chain](chain),
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to deploy %s: %w", spec.ContractType, err)
	}

	addr := common.HexToAddress(deployReport.Output.Address)

	_, err = operations.ExecuteOperation(
		b,
		evmsetconfig.OpEVMSetConfigMCM,
		chain,
		evmsetconfig.OpEVMSetConfigInput{
			Target: evmsetconfig.MCMSetConfigTarget{
				Address:      addr,
				Config:       spec.Config,
				ContractType: spec.ContractType,
			},
			NoSend: false,
		},
		gasboost.RetryWithGasBoost[evmsetconfig.OpEVMSetConfigInput](gasBoost),
		outputAddressIdempotencyKey[evmsetconfig.OpEVMSetConfigInput, cldf_evm.Chain](chain, deployReport.Output.Address),
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to set config on %s: %w", addr.Hex(), err)
	}

	return addr, nil
}

// deployCallProxy deploys a call proxy address and serts the given address as target.
func deployCallProxy(
	b operations.Bundle,
	chain cldf_evm.Chain,
	chainSelector uint64,
	targetAddr common.Address,
	gasBoost *cldfproposalutils.GasBoostConfig,
	timelockSpec deploycustomtopology.TimelockSpec) (common.Address, error) {
	cpReport, err := operations.ExecuteOperation(
		b,
		callproxyops.Deploy,
		chain,
		opscontract.DeployInput[callproxyops.ConstructorArgs]{
			TypeAndVersion: callproxyops.TypeAndVersion,
			Qualifier:      stringPtrIfNonEmpty(timelockSpec.Qualifier),
			Args:           callproxyops.ConstructorArgs{Target: targetAddr},
		},
		gasboost.RetryDeploy[callproxyops.ConstructorArgs](gasBoost),
		chainIdempotencyKey[opscontract.DeployInput[callproxyops.ConstructorArgs], cldf_evm.Chain](chain),
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("deploy call proxy for timelock %q on chain %d: %w", timelockSpec.Ref, chainSelector, err)
	}
	callProxyAddr := common.HexToAddress(cpReport.Output.Address)

	return callProxyAddr, nil
}

// deployTimelockAndSetRoles deploys a timelock contract and configures the roles according to the
// provided ref spec. Can also deploy a call proxy if specified in the configuration.
func deployTimelockAndSetRoles(
	b operations.Bundle,
	chain cldf_evm.Chain,
	chainSelector uint64,
	gasBoost *cldfproposalutils.GasBoostConfig,
	extra EVMExtraArgs,
	tl deploycustomtopology.TimelockSpec,
	refToAddr map[string]common.Address,
	out *deploycustomtopology.ChainOutput,
) error {
	proposers, err := resolveHolders(tl.Roles.Proposers, refToAddr)
	if err != nil {
		return fmt.Errorf("timelock %q proposers: %w", tl.Ref, err)
	}
	cancellers, err := resolveHolders(tl.Roles.Cancellers, refToAddr)
	if err != nil {
		return fmt.Errorf("timelock %q cancellers: %w", tl.Ref, err)
	}
	bypassers, err := resolveHolders(tl.Roles.Bypassers, refToAddr)
	if err != nil {
		return fmt.Errorf("timelock %q bypassers: %w", tl.Ref, err)
	}
	executors, err := resolveHolders(tl.Roles.Executors, refToAddr)
	if err != nil {
		return fmt.Errorf("timelock %q executors: %w", tl.Ref, err)
	}
	admins, err := resolveHolders(tl.Roles.Admins, refToAddr)
	if err != nil {
		return fmt.Errorf("timelock %q admins: %w", tl.Ref, err)
	}

	// Deploy the timelock with the deployer as initial admin. Executors are left
	// empty here and granted below (the call proxy must be deployed after the
	// timelock, and explicit executors are granted alongside it).
	tlReport, err := operations.ExecuteOperation(
		b,
		timelockops.Deploy,
		chain,
		opscontract.DeployInput[timelockops.ConstructorArgs]{
			TypeAndVersion: timelockops.TypeAndVersion,
			Qualifier:      stringPtrIfNonEmpty(tl.Qualifier),
			Args: timelockops.ConstructorArgs{
				MinDelay:   tl.MinDelay,
				Admin:      chain.DeployerKey.From,
				Proposers:  proposers,
				Executors:  []common.Address{},
				Cancellers: cancellers,
				Bypassers:  bypassers,
			},
		},
		gasboost.RetryDeploy[timelockops.ConstructorArgs](gasBoost),
		chainIdempotencyKey[opscontract.DeployInput[timelockops.ConstructorArgs], cldf_evm.Chain](chain),
	)
	if err != nil {
		return fmt.Errorf("deploy timelock %q on chain %d: %w", tl.Ref, chainSelector, err)
	}
	timelockAddr := common.HexToAddress(tlReport.Output.Address)
	refToAddr[tl.Ref] = timelockAddr
	out.Metadata.Addresses = append(out.Metadata.Addresses,
		newAddressRef(chainSelector,
			timelockAddr,
			mcmscontracts.RBACTimelock,
			tl.Qualifier,
			tl.Label,
		),
	)
	// Deploy call proxy
	var callProxyAddr common.Address
	if extra.shouldDeployCallProxy(tl.Ref) {
		callProxyAddr, err = deployCallProxy(b, chain, chainSelector, timelockAddr, gasBoost, tl)
		if err != nil {
			return fmt.Errorf("failed to deploy call proxy for timelock %q on chain %d: %w", tl.Ref, chainSelector, err)
		}
		out.Metadata.Addresses = append(out.Metadata.Addresses,
			newAddressRef(chainSelector, callProxyAddr, mcmscontracts.CallProxy, tl.Qualifier, tl.Label))
	}

	// Grant roles for all deployed contracts
	if err := grantRoles(b, chain, chainSelector, gasBoost, timelockAddr, callProxyAddr,
		proposers, cancellers, bypassers, executors, admins); err != nil {
		return fmt.Errorf("grant roles for timelock %q on chain %d: %w", tl.Ref, chainSelector, err)
	}

	// Transfer ownership of the requested contracts to this timelock. The
	// acceptOwnership() calls are routed through the timelock as MCMS batch ops.
	if len(tl.TransferOwnership) > 0 {
		contracts, err := resolveHolders(tl.TransferOwnership, refToAddr)
		if err != nil {
			return fmt.Errorf("timelock %q transferOwnership: %w", tl.Ref, err)
		}
		ops, err := transferOwnershipToTimelock(b, chain, chainSelector, timelockAddr, contracts)
		if err != nil {
			return fmt.Errorf("transfer ownership to timelock %q on chain %d: %w", tl.Ref, chainSelector, err)
		}
		if len(ops) > 0 {
			out.ProposalGroups = append(out.ProposalGroups, deploycustomtopology.ProposalGroup{
				Qualifier: tl.Qualifier,
				BatchOps: []mcmstypes.BatchOperation{{
					ChainSelector: mcmstypes.ChainSelector(chainSelector),
					Transactions:  ops,
				}},
			})
		}
	}

	return nil
}

// roleGrant pairs a timelock role with the addresses to grant it to.
type roleGrant struct {
	Role      common.Hash
	Name      string
	Addresses []common.Address
}

// grantRoles grants the configured roles on the timelock. Proposer/canceller/
// bypasser members already set in the constructor are skipped; this adds the
// call proxy (and any explicit executors) to the executor role and grants admin
// to the timelock itself (plus any extra admins).
func grantRoles(
	b operations.Bundle,
	chain cldf_evm.Chain,
	chainSelector uint64,
	gasBoost *cldfproposalutils.GasBoostConfig,
	timelockAddr, callProxyAddr common.Address,
	proposers, cancellers, bypassers, executors, admins []common.Address,
) error {
	grants := []roleGrant{
		{Role: mcmsrole.ProposerRole.ID, Name: mcmsrole.ProposerRole.Name, Addresses: proposers},
		{Role: mcmsrole.CancellerRole.ID, Name: mcmsrole.CancellerRole.Name, Addresses: cancellers},
		{Role: mcmsrole.BypasserRole.ID, Name: mcmsrole.BypasserRole.Name, Addresses: bypassers},
	}

	execAddrs := append([]common.Address{}, executors...)
	if callProxyAddr != (common.Address{}) {
		execAddrs = append(execAddrs, callProxyAddr)
	}
	if len(execAddrs) > 0 {
		grants = append(grants, roleGrant{
			Role: mcmsrole.ExecutorRole.ID, Name: mcmsrole.ExecutorRole.Name, Addresses: execAddrs,
		})
	}

	// Grant admin to the timelock itself so it can self-govern, plus any extra admins.
	adminAddrs := append([]common.Address{}, admins...)
	adminAddrs = append(adminAddrs, timelockAddr)
	grants = append(grants, roleGrant{
		Role: mcmsrole.AdminRole.ID, Name: mcmsrole.AdminRole.Name, Addresses: adminAddrs,
	})

	timelockInspector := mcmsevmsdk.NewTimelockInspector(chain.Client)

	for _, g := range grants {
		var (
			existing []string
			err      error
		)
		switch g.Role {
		case mcmsrole.ProposerRole.ID:
			existing, err = timelockInspector.GetProposers(b.GetContext(), timelockAddr.Hex())
		case mcmsrole.CancellerRole.ID:
			existing, err = timelockInspector.GetCancellers(b.GetContext(), timelockAddr.Hex())
		case mcmsrole.BypasserRole.ID:
			existing, err = timelockInspector.GetBypassers(b.GetContext(), timelockAddr.Hex())
		case mcmsrole.ExecutorRole.ID:
			existing, err = timelockInspector.GetExecutors(b.GetContext(), timelockAddr.Hex())
		case mcmsrole.AdminRole.ID:
			existing = nil
		}
		if err != nil {
			b.Logger.Errorw("Failed to get addresses from Timelock Inspector",
				"chainSelector", chain.ChainSelector(),
				"chainName", chain.Name(),
				"Timelock Address", timelockAddr.Hex(),
				"Role", g.Name,
				"Error", err,
			)

			return err
		}

		for _, addr := range g.Addresses {
			if slices.Contains(existing, addr.Hex()) {
				continue
			}
			_, err := operations.ExecuteOperation(b, evmgrantrole.OpGrantRole,
				chain,
				evmops.EVMCallInput[evmgrantrole.OpGrantRoleInput]{
					ChainSelector: chainSelector,
					CallInput: evmgrantrole.OpGrantRoleInput{
						Account: addr,
						RoleID:  g.Role,
					},
					Address: timelockAddr,
				},
				evmops.RetryCallWithGasBoost[evmgrantrole.OpGrantRoleInput](gasBoost),
			)
			if err != nil {
				b.Logger.Errorw("Failed to grant role",
					"chainSelector", chain.ChainSelector(),
					"chainName", chain.Name(),
					"Timelock Address", timelockAddr.Hex(),
					"Role Name", g.Name,
					"Address", addr.Hex(),
				)

				return err
			}
			b.Logger.Infow("Role granted",
				"Role Name", g.Name,
				"chainSelector", chain.ChainSelector(),
				"chainName", chain.Name(),
				"Timelock Address", timelockAddr.Hex(),
				"Address", addr.Hex(),
			)
		}
	}

	return nil
}

// transferOwnershipToTimelock runs transferOwnership() (deployer-key send) and
// then simulates acceptOwnership() so the resulting calldata can be routed
// through the timelock as MCMS batch operations.
func transferOwnershipToTimelock(
	b operations.Bundle,
	chain cldf_evm.Chain,
	chainSelector uint64,
	timelock common.Address,
	contracts []common.Address,
) ([]mcmstypes.Transaction, error) {
	var mcsmOps []mcmstypes.Transaction

	for _, contract := range contracts {
		owner, c, err := LoadOwnableContract(contract, chain.Client)
		if err != nil {
			b.Logger.Errorf("failed to load ownable contract %s: %v", contract.Hex(), err)
			return nil, fmt.Errorf("error loading ownable contract %s: %w", contract.Hex(), err)
		}

		if owner.String() == timelock.Hex() {
			b.Logger.Infof("contract %s already owned by timelock", contract)
			continue
		}

		deps := evmtransferownership.OpOwnershipDeps{Chain: chain, OwnableC: c}
		_, err = operations.ExecuteOperation(b, evmtransferownership.OpTransferOwnership, deps,
			evmtransferownership.OpTransferOwnershipInput{
				ChainSelector:   chainSelector,
				TimelockAddress: timelock,
				Address:         contract,
			},
		)
		if err != nil {
			return nil, err
		}

		acceptReport, err := operations.ExecuteOperation(b, evmtransferownership.OpAcceptOwnership, deps,
			evmtransferownership.OpTransferOwnershipInput{
				ChainSelector:   chainSelector,
				TimelockAddress: timelock,
				Address:         contract,
			},
		)
		if err != nil {
			return nil, err
		}

		mcsmOps = append(mcsmOps, mcmstypes.Transaction{
			To:               contract.Hex(),
			Data:             acceptReport.Output.Tx.Data(),
			AdditionalFields: json.RawMessage(`{"value": 0}`),
		})
	}

	return mcsmOps, nil
}

// LoadOwnableContract loads an ownable contract using the shared ownership ABI
// and returns its current owner and an Ownable handle.
func LoadOwnableContract(addr common.Address, client bind.ContractBackend) (common.Address, evmtransferownership.Ownable, error) {
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

// resolveHolders maps role holders to concrete addresses: an MCMRef resolves to a
// contract deployed earlier in this run; an Address is used directly.
func resolveHolders(holders []deploycustomtopology.RoleHolder, refToAddr map[string]common.Address) ([]common.Address, error) {
	addrs := make([]common.Address, 0, len(holders))
	for i, h := range holders {
		switch {
		case h.MCMRef != "":
			a, ok := refToAddr[h.MCMRef]
			if !ok {
				return nil, fmt.Errorf("holder[%d]: mcmRef %q has not been deployed", i, h.MCMRef)
			}
			addrs = append(addrs, a)
		case h.Address != nil:
			addrs = append(addrs, *h.Address)
		default:
			return nil, fmt.Errorf("holder[%d]: exactly one of mcmRef or address is required", i)
		}
	}

	return addrs, nil
}

// newAddressRef builds a datastore AddressRef for a newly deployed contract.
// label is optional; pass nil or empty to omit it.
func newAddressRef(chainSelector uint64, addr common.Address, contractType cldf.ContractType, qualifier string, label *string) cldfdatastore.AddressRef {
	v := semvers.V1_0_0
	ref := cldfdatastore.AddressRef{
		ChainSelector: chainSelector,
		Address:       addr.Hex(),
		Type:          cldfdatastore.ContractType(contractType),
		Version:       &v,
		Qualifier:     qualifier,
	}
	if label != nil && *label != "" {
		ref.Labels = cldfdatastore.NewLabelSet(*label)
	}

	return ref
}

func mcmTypeVersion(contractType cldf.ContractType) (cldf.TypeAndVersion, error) {
	switch contractType {
	case mcmscontracts.ProposerManyChainMultisig:
		return mcmops.ProposerManyChainMultiSigTypeAndVersion, nil
	case mcmscontracts.BypasserManyChainMultisig:
		return mcmops.BypasserManyChainMultiSigTypeAndVersion, nil
	case mcmscontracts.CancellerManyChainMultisig:
		return mcmops.CancellerManyChainMultiSigTypeAndVersion, nil
	default:
		return cldf.TypeAndVersion{}, fmt.Errorf("unsupported contract type for deploy-custom-topology: %s", contractType)
	}
}

func stringPtrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func chainIdempotencyKey[IN, DEP any](chain cldf_evm.Chain) operations.ExecuteOption[IN, DEP] {
	return operations.WithIdempotencyKey[IN, DEP](strconv.FormatUint(chain.Selector, 10))
}

func outputAddressIdempotencyKey[IN, DEP any](chain cldf_evm.Chain, address string) operations.ExecuteOption[IN, DEP] {
	return operations.WithIdempotencyKey[IN, DEP](strconv.FormatUint(chain.Selector, 10) + ":" + address)
}
