package changesets

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/spf13/cast"

	"github.com/smartcontractkit/cld-changesets/pkg/contract/mcms/view/v1_0"
	evmstate "github.com/smartcontractkit/cld-changesets/pkg/family/evm"
	opsevm "github.com/smartcontractkit/cld-changesets/pkg/family/evm/operations"
	seqs "github.com/smartcontractkit/cld-changesets/pkg/family/evm/sequences"
)

// DeployMCMSOption is a function that modifies a TypeAndVersion before or after deployment.
type DeployMCMSOption func(*cldf.TypeAndVersion)

// WithLabel is a functional option that sets a label on the TypeAndVersion.
func WithLabel(label string) DeployMCMSOption {
	return func(tv *cldf.TypeAndVersion) {
		tv.AddLabel(label)
	}
}

// MCMSWithTimelockEVMDeploy holds a bundle of MCMS contract deploys.
type MCMSWithTimelockEVMDeploy struct {
	Canceller *cldf.ContractDeploy[*bindings.ManyChainMultiSig]
	Bypasser  *cldf.ContractDeploy[*bindings.ManyChainMultiSig]
	Proposer  *cldf.ContractDeploy[*bindings.ManyChainMultiSig]
	Timelock  *cldf.ContractDeploy[*bindings.RBACTimelock]
	CallProxy *cldf.ContractDeploy[*bindings.CallProxy]
}

// DeployMCMSWithTimelockContractsEVM deploys an MCMS for
// each of the timelock roles Bypasser, ProposerMcm, Canceller on an EVM chain.
// MCMS contracts for the given configuration
// as well as the timelock. It's not necessarily the only way to use
// the timelock and MCMS, but its reasonable pattern.
func DeployMCMSWithTimelockContractsEVM(
	env cldf.Environment,
	chain cldf_evm.Chain,
	ab cldf.AddressBook,
	config cldfproposalutils.MCMSWithTimelockConfig,
	state *evmstate.MCMSWithTimelockState,
) ([]operations.Report[any, any], error) {
	execReports := make([]operations.Report[any, any], 0)
	lggr := env.Logger
	opts := []func(*cldf.TypeAndVersion){}
	if config.Label != nil {
		opts = append(opts, WithLabel(*config.Label))
	}
	var bypasser, proposer, canceller *bindings.ManyChainMultiSig
	var timelock *bindings.RBACTimelock
	var callProxy *bindings.CallProxy
	if state != nil {
		bypasser = state.BypasserMcm
		proposer = state.ProposerMcm
		canceller = state.CancellerMcm
		timelock = state.Timelock
		callProxy = state.CallProxy
	}
	if bypasser == nil {
		seqInput := seqs.SeqDeployMCMWithConfigInput{
			ContractType:   mcmscontracts.BypasserManyChainMultisig,
			MCMConfig:      config.Bypasser,
			ChainSelector:  chain.Selector,
			GasBoostConfig: config.GasBoostConfig,
		}

		report, err := operations.ExecuteSequence(
			env.OperationsBundle,
			seqs.SeqEVMDeployMCMWithConfig,
			chain,
			seqInput,
		)
		execReports = append(execReports, report.ExecutionReports...)
		if err != nil {
			lggr.Errorw("Failed to deploy bypasser MCMS", "chain", chain.String(), "err", err)
			return execReports, err
		}
		typeAndVersion := cldf.MustTypeAndVersionFromString(report.Output.TypeAndVersion)
		for _, option := range opts {
			option(&typeAndVersion)
		}
		err = ab.Save(chain.Selector, report.Output.Address.Hex(), typeAndVersion)
		if err != nil {
			lggr.Errorw("Failed to save bypasser MCMS address in address book", "chain", chain.String(), "err", err)
			return execReports, err
		}

		bypasser, err = bindings.NewManyChainMultiSig(report.Output.Address, chain.Client)
		if err != nil {
			lggr.Errorw("Failed to create bypasser MCMS binding", "chain", chain.String(), "err", err)
			return execReports, err
		}
		lggr.Infow("Bypasser MCMS deployed", "chain", chain.String(), "address", bypasser.Address().String())
	} else {
		lggr.Infow("Bypasser MCMS already deployed", "chain", chain.String(), "address", bypasser.Address().String())
	}

	if canceller == nil {
		seqInput := seqs.SeqDeployMCMWithConfigInput{
			ContractType:   mcmscontracts.CancellerManyChainMultisig,
			MCMConfig:      config.Canceller,
			ChainSelector:  chain.Selector,
			GasBoostConfig: config.GasBoostConfig,
		}

		report, err := operations.ExecuteSequence(
			env.OperationsBundle,
			seqs.SeqEVMDeployMCMWithConfig,
			chain,
			seqInput,
		)
		execReports = append(execReports, report.ExecutionReports...)
		if err != nil {
			lggr.Errorw("Failed to deploy Canceller MCMS", "chain", chain.String(), "err", err)
			return execReports, err
		}
		typeAndVersion := cldf.MustTypeAndVersionFromString(report.Output.TypeAndVersion)
		for _, option := range opts {
			option(&typeAndVersion)
		}
		err = ab.Save(chain.Selector, report.Output.Address.Hex(), typeAndVersion)
		if err != nil {
			lggr.Errorw("Failed to save canceller MCMS address in address book", "chain", chain.String(), "err", err)
			return execReports, err
		}

		canceller, err = bindings.NewManyChainMultiSig(report.Output.Address, chain.Client)
		if err != nil {
			lggr.Errorw("Failed to create Canceller MCMS binding", "chain", chain.String(), "err", err)
			return execReports, err
		}
		lggr.Infow("Canceller MCMS deployed", "chain", chain.String(), "address", canceller.Address().String())
	} else {
		lggr.Infow("Canceller MCMS already deployed", "chain", chain.String(), "address", canceller.Address().String())
	}

	if proposer == nil {
		seqInput := seqs.SeqDeployMCMWithConfigInput{
			ContractType:   mcmscontracts.ProposerManyChainMultisig,
			MCMConfig:      config.Proposer,
			ChainSelector:  chain.Selector,
			GasBoostConfig: config.GasBoostConfig,
		}

		report, err := operations.ExecuteSequence(
			env.OperationsBundle,
			seqs.SeqEVMDeployMCMWithConfig,
			chain,
			seqInput,
		)
		execReports = append(execReports, report.ExecutionReports...)
		if err != nil {
			lggr.Errorw("Failed to deploy Proposer MCMS", "chain", chain.String(), "err", err)
			return execReports, err
		}
		typeAndVersion := cldf.MustTypeAndVersionFromString(report.Output.TypeAndVersion)
		for _, option := range opts {
			option(&typeAndVersion)
		}
		err = ab.Save(chain.Selector, report.Output.Address.Hex(), typeAndVersion)
		if err != nil {
			lggr.Errorw("Failed to save proposer MCMS address in address book", "chain", chain.String(), "err", err)
			return execReports, err
		}

		proposer, err = bindings.NewManyChainMultiSig(report.Output.Address, chain.Client)
		if err != nil {
			lggr.Errorw("Failed to create Proposer MCMS binding", "chain", chain.String(), "err", err)
			return execReports, err
		}
		lggr.Infow("Proposer MCMS deployed", "chain", chain.String(), "address", proposer.Address().String())
	} else {
		lggr.Infow("Proposer MCMS already deployed", "chain", chain.String(), "address", proposer.Address().String())
	}

	if timelock == nil {
		opInput := opsevm.OpEVMDeployTimelockInput{
			// Deployer is the initial admin.
			// TODO: Could expose this as config?
			// Or keep this enforced to follow the same pattern?
			Admin:     chain.DeployerKey.From,
			Proposers: []common.Address{proposer.Address()},
			// Executors field is empty here because we grant the executor role to the call proxy later
			// and the call proxy cannot be deployed before the timelock.
			Executors:        []common.Address{},
			Cancellers:       []common.Address{canceller.Address(), proposer.Address(), bypasser.Address()}, // cancellers
			Bypassers:        []common.Address{bypasser.Address()},                                          // bypassers
			TimelockMinDelay: config.TimelockMinDelay,
		}

		report, err := operations.ExecuteOperation(
			env.OperationsBundle,
			opsevm.OpEVMDeployTimelock,
			chain,
			opsevm.EVMDeployInput[opsevm.OpEVMDeployTimelockInput]{
				ChainSelector: chain.Selector,
				DeployInput:   opInput,
			},
			opsevm.RetryDeploymentWithGasBoost[opsevm.OpEVMDeployTimelockInput](config.GasBoostConfig),
		)
		execReports = append(execReports, report.ToGenericReport())
		if err != nil {
			lggr.Errorw("Failed to deploy timelock", "chain", chain.String(), "err", err)
			return execReports, err
		}
		typeAndVersion := cldf.MustTypeAndVersionFromString(report.Output.TypeAndVersion)
		for _, option := range opts {
			option(&typeAndVersion)
		}
		err = ab.Save(chain.Selector, report.Output.Address.Hex(), typeAndVersion)
		if err != nil {
			lggr.Errorw("Failed to save timelock address in address book", "chain", chain.String(), "err", err)
			return execReports, err
		}

		timelock, err = bindings.NewRBACTimelock(report.Output.Address, chain.Client)
		if err != nil {
			lggr.Errorw("Failed to create Timelock binding", "chain", chain.String(), "err", err)
			return execReports, err
		}

		lggr.Infow("Timelock deployed", "chain", chain.String(), "address", timelock.Address().String())
	} else {
		lggr.Infow("Timelock already deployed", "chain", chain.String(), "address", timelock.Address().String())
	}

	if callProxy == nil {
		opInput := opsevm.OpEVMDeployCallProxyInput{
			Timelock: timelock.Address(),
		}

		report, err := operations.ExecuteOperation(
			env.OperationsBundle,
			opsevm.OpEVMDeployCallProxy,
			chain,
			opsevm.EVMDeployInput[opsevm.OpEVMDeployCallProxyInput]{
				ChainSelector: chain.Selector,
				DeployInput:   opInput,
			},
			opsevm.RetryDeploymentWithGasBoost[opsevm.OpEVMDeployCallProxyInput](config.GasBoostConfig),
		)
		execReports = append(execReports, report.ToGenericReport())
		if err != nil {
			lggr.Errorw("Failed to deploy CallProxy", "chain", chain.String(), "err", err)
			return execReports, err
		}
		typeAndVersion := cldf.MustTypeAndVersionFromString(report.Output.TypeAndVersion)
		for _, option := range opts {
			option(&typeAndVersion)
		}
		err = ab.Save(chain.Selector, report.Output.Address.Hex(), typeAndVersion)
		if err != nil {
			lggr.Errorw("Failed to save CallProxy address in address book", "chain", chain.String(), "err", err)
		}

		callProxy, err = bindings.NewCallProxy(report.Output.Address, chain.Client)
		if err != nil {
			lggr.Errorw("Failed to create CallProxy binding", "chain", chain.String(), "err", err)
			return execReports, err
		}
		lggr.Infow("CallProxy deployed", "chain", chain.String(), "address", callProxy.Address().String())
	} else {
		lggr.Infow("CallProxy already deployed", "chain", chain.String(), "address", callProxy.Address().String())
	}
	timelockContracts := &cldfproposalutils.MCMSWithTimelockContracts{
		BypasserMcm:  bypasser,
		ProposerMcm:  proposer,
		CancellerMcm: canceller,
		Timelock:     timelock,
		CallProxy:    callProxy,
	}
	// grant roles for timelock
	// this is called only if deployer key is an admin in timelock
	seqReport, err := GrantRolesForTimelock(env, chain, timelockContracts, true, config.GasBoostConfig)
	execReports = append(execReports, seqReport.ExecutionReports...)
	if err != nil {
		return execReports, err
	}
	// After the proposer cycle is validated,
	// we can remove the deployer as an admin.
	return execReports, nil
}

// TODO: delete this function after it is available in timelock Inspector
func getAdminAddresses(ctx context.Context, timelock *bindings.RBACTimelock) ([]string, error) {
	numAddresses, err := timelock.GetRoleMemberCount(&bind.CallOpts{
		Context: ctx,
	}, v1_0.ADMIN_ROLE.ID)
	if err != nil {
		return nil, err
	}
	adminAddresses := make([]string, 0, numAddresses.Uint64())
	for i := range numAddresses.Uint64() {
		if i > math.MaxUint32 {
			return nil, fmt.Errorf("value %d exceeds uint32 range", i)
		}
		idx, err := cast.ToInt64E(i)
		if err != nil {
			return nil, err
		}
		address, err := timelock.GetRoleMember(&bind.CallOpts{
			Context: ctx,
		}, v1_0.ADMIN_ROLE.ID, big.NewInt(idx))
		if err != nil {
			return nil, err
		}
		adminAddresses = append(adminAddresses, address.String())
	}
	return adminAddresses, nil
}

func GrantRolesForTimelock(
	env cldf.Environment,
	chain cldf_evm.Chain,
	timelockContracts *cldfproposalutils.MCMSWithTimelockContracts,
	skipIfDeployerKeyNotAdmin bool, // If true, skip role grants if the deployer key is not an admin.
	gasBoostConfig *cldfproposalutils.GasBoostConfig,
) (operations.SequenceReport[seqs.SeqGrantRolesTimelockInput, map[uint64][]opsevm.EVMCallOutput], error) {
	lggr := env.Logger
	ctx := env.GetContext()

	if timelockContracts == nil {
		lggr.Errorw("Timelock contracts not found", "chain", chain.String())
		return operations.SequenceReport[seqs.SeqGrantRolesTimelockInput, map[uint64][]opsevm.EVMCallOutput]{}, fmt.Errorf("timelock contracts not found for chain %s", chain.String())
	}

	timelock := timelockContracts.Timelock
	proposer := timelockContracts.ProposerMcm
	canceller := timelockContracts.CancellerMcm
	bypasser := timelockContracts.BypasserMcm
	callProxy := timelockContracts.CallProxy

	// get admin addresses
	adminAddresses, err := getAdminAddresses(ctx, timelock)
	if err != nil {
		return operations.SequenceReport[seqs.SeqGrantRolesTimelockInput, map[uint64][]opsevm.EVMCallOutput]{}, fmt.Errorf("failed to get admin addresses: %w", err)
	}
	isDeployerKeyAdmin := slices.Contains(adminAddresses, chain.DeployerKey.From.String())
	isTimelockAdmin := slices.Contains(adminAddresses, timelock.Address().String())
	if !isDeployerKeyAdmin && skipIfDeployerKeyNotAdmin {
		lggr.Infow("Deployer key is not admin, skipping role grants", "chain", chain.String())
		return operations.SequenceReport[seqs.SeqGrantRolesTimelockInput, map[uint64][]opsevm.EVMCallOutput]{}, nil
	}
	if !isDeployerKeyAdmin && !isTimelockAdmin {
		return operations.SequenceReport[seqs.SeqGrantRolesTimelockInput, map[uint64][]opsevm.EVMCallOutput]{}, errors.New("neither deployer key nor timelock is admin, cannot grant roles")
	}

	seqDeps := seqs.SeqGrantRolesTimelockDeps{
		Chain: chain,
	}

	seqInput := seqs.SeqGrantRolesTimelockInput{
		ContractType:       mcmscontracts.RBACTimelock,
		ChainSelector:      chain.Selector,
		Timelock:           timelock.Address(),
		IsDeployerKeyAdmin: isDeployerKeyAdmin,
		RolesAndAddresses: []seqs.RolesAndAddresses{
			{
				Role:      v1_0.PROPOSER_ROLE.ID,
				Name:      v1_0.PROPOSER_ROLE.Name,
				Addresses: []common.Address{proposer.Address()},
			},
			{
				Role:      v1_0.CANCELLER_ROLE.ID,
				Name:      v1_0.CANCELLER_ROLE.Name,
				Addresses: []common.Address{proposer.Address(), canceller.Address(), bypasser.Address()},
			},
			{
				Role:      v1_0.BYPASSER_ROLE.ID,
				Name:      v1_0.BYPASSER_ROLE.Name,
				Addresses: []common.Address{bypasser.Address()},
			},
			{
				Role:      v1_0.EXECUTOR_ROLE.ID,
				Name:      v1_0.EXECUTOR_ROLE.Name,
				Addresses: []common.Address{callProxy.Address()},
			},
		},
		GasBoostConfig: gasBoostConfig,
	}

	if !isTimelockAdmin {
		// We grant the timelock the admin role on the MCMS contracts.
		seqInput.RolesAndAddresses = append(seqInput.RolesAndAddresses, seqs.RolesAndAddresses{
			Role:      v1_0.ADMIN_ROLE.ID,
			Name:      v1_0.ADMIN_ROLE.Name,
			Addresses: []common.Address{timelock.Address()},
		})
	}

	report, err := operations.ExecuteSequence(
		env.OperationsBundle,
		seqs.SeqGrantRolesTimelock,
		seqDeps,
		seqInput,
	)
	if err != nil {
		lggr.Errorw("Failed to grant roles for timelock", "chain", chain.String(), "err", err)
		return operations.SequenceReport[seqs.SeqGrantRolesTimelockInput, map[uint64][]opsevm.EVMCallOutput]{}, err
	}

	return report, nil
}
