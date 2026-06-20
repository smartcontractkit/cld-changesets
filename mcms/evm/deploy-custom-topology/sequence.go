package evmdeploytopology

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/burn_mint_erc677"
	mcmsevmsdk "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/mcmsrole"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	deploycustomtopology "github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy-custom-topology"
	evmgrantrole "github.com/smartcontractkit/cld-changesets/mcms/evm/grant-role"
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
		addr, err := deployMCMWithConfig(b, chain, in.ChainSelector, gasBoost, m)
		if err != nil {
			return out, fmt.Errorf("deploy MCM %q on chain %d: %w", m.Ref, in.ChainSelector, err)
		}
		refToAddr[m.Ref] = addr
		out.Metadata.Addresses = append(out.Metadata.Addresses,
			newAddressRef(in.ChainSelector, addr, m.ContractType, m.Qualifier, m.Label))
	}

	// 2. Deploy and wire each timelock.
	for _, tl := range in.Config.Timelocks {
		if err := deployTimelock(b, chain, in.ChainSelector, gasBoost, extra, tl, refToAddr, &out); err != nil {
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
	chainSelector uint64,
	gasBoost *cldfproposalutils.GasBoostConfig,
	spec deploycustomtopology.MCMSpec,
) (common.Address, error) {
	deployInput := evmops.EVMDeployInput[any]{
		ChainSelector: chainSelector,
		Qualifier:     &spec.Qualifier,
	}

	var (
		deployReport operations.Report[evmops.EVMDeployInput[any], evmops.EVMDeployOutput]
		err          error
	)
	switch spec.ContractType {
	case mcmscontracts.BypasserManyChainMultisig:
		deployReport, err = operations.ExecuteOperation(b, OpDeployBypasserMCM, chain, deployInput,
			evmops.RetryDeploymentWithGasBoost[any](gasBoost))
	case mcmscontracts.ProposerManyChainMultisig:
		deployReport, err = operations.ExecuteOperation(b, OpDeployProposerMCM, chain, deployInput,
			evmops.RetryDeploymentWithGasBoost[any](gasBoost))
	case mcmscontracts.CancellerManyChainMultisig:
		deployReport, err = operations.ExecuteOperation(b, OpDeployCancellerMCM, chain, deployInput,
			evmops.RetryDeploymentWithGasBoost[any](gasBoost))
	default:
		return common.Address{}, fmt.Errorf("unsupported contract type for deploy-custom-topology: %s", spec.ContractType)
	}
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to deploy %s: %w", spec.ContractType, err)
	}

	_, err = operations.ExecuteOperation(
		b,
		evmsetconfig.OpEVMSetConfigMCM,
		chain,
		evmsetconfig.OpEVMSetConfigInput{
			Target: evmsetconfig.MCMSetConfigTarget{
				Address:      deployReport.Output.Address,
				Config:       spec.Config,
				ContractType: spec.ContractType,
			},
			NoSend: false,
		},
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to set config on %s: %w", deployReport.Output.Address.Hex(), err)
	}

	return deployReport.Output.Address, nil
}

func deployTimelock(
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
		OpDeployTimelock,
		chain,
		evmops.EVMDeployInput[OpDeployTimelockInput]{
			ChainSelector: chainSelector,
			DeployInput: OpDeployTimelockInput{
				TimelockMinDelay: tl.MinDelay,
				Admin:            chain.DeployerKey.From,
				Proposers:        proposers,
				Executors:        []common.Address{},
				Cancellers:       cancellers,
				Bypassers:        bypassers,
			},
		},
		evmops.RetryDeploymentWithGasBoost[OpDeployTimelockInput](gasBoost),
	)
	if err != nil {
		return fmt.Errorf("deploy timelock %q on chain %d: %w", tl.Ref, chainSelector, err)
	}
	timelockAddr := tlReport.Output.Address
	refToAddr[tl.Ref] = timelockAddr
	out.Metadata.Addresses = append(out.Metadata.Addresses,
		newAddressRef(chainSelector, timelockAddr, mcmscontracts.RBACTimelock, tl.Qualifier, tl.Label))

	// Optionally deploy a call proxy and grant it the executor role.
	var callProxyAddr common.Address
	if extra.deployCallProxy(tl.Ref) {
		cpReport, err := operations.ExecuteOperation(
			b,
			OpDeployCallProxy,
			chain,
			evmops.EVMDeployInput[OpDeployCallProxyInput]{
				ChainSelector: chainSelector,
				DeployInput:   OpDeployCallProxyInput{Timelock: timelockAddr},
			},
			evmops.RetryDeploymentWithGasBoost[OpDeployCallProxyInput](gasBoost),
		)
		if err != nil {
			return fmt.Errorf("deploy call proxy for timelock %q on chain %d: %w", tl.Ref, chainSelector, err)
		}
		callProxyAddr = cpReport.Output.Address
		out.Metadata.Addresses = append(out.Metadata.Addresses,
			newAddressRef(chainSelector, callProxyAddr, mcmscontracts.CallProxy, tl.Qualifier, tl.Label))
	}

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
			existing = nil // TODO: change once inspector supports get Admin.
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
		if h.MCMRef != "" {
			a, ok := refToAddr[h.MCMRef]
			if !ok {
				return nil, fmt.Errorf("holder[%d]: mcmRef %q has not been deployed", i, h.MCMRef)
			}
			addrs = append(addrs, a)
			continue
		}
		if h.Address != nil {
			addrs = append(addrs, *h.Address)
			continue
		}
		return nil, fmt.Errorf("holder[%d]: exactly one of mcmRef or address is required", i)
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
