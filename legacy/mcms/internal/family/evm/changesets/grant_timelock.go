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
	"github.com/spf13/cast"

	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/internal/mcmsrole"
	seqevm "github.com/smartcontractkit/cld-changesets/legacy/mcms/internal/family/evm/sequences"
	evmops "github.com/smartcontractkit/cld-changesets/legacy/mcms/operations"
)

// GrantRolesForTimelock grants RBACTimelock roles to the MCMS contracts in timelockContracts.
//
// It runs SeqGrantRolesTimelock to assign PROPOSER, CANCELLER, BYPASSER, and EXECUTOR roles,
// and grants ADMIN to the timelock when it is not already an admin. When the deployer key is
// not a timelock admin, transactions are queued for MCMS proposal execution; if
// skipIfDeployerKeyNotAdmin is true, role grants are skipped instead.
//
// This is a deployment helper invoked by DeployMCMSWithTimelock, not a standalone Changeset.
func GrantRolesForTimelock(
	env cldf.Environment,
	chain cldf_evm.Chain,
	timelockContracts *cldfproposalutils.MCMSWithTimelockContracts,
	skipIfDeployerKeyNotAdmin bool, // If true, skip role grants if the deployer key is not an admin.
	gasBoostConfig *cldfproposalutils.GasBoostConfig,
) (operations.SequenceReport[seqevm.SeqGrantRolesTimelockInput, map[uint64][]evmops.EVMCallOutput], error) {
	lggr := env.Logger
	ctx := env.GetContext()

	if timelockContracts == nil {
		lggr.Errorw("Timelock contracts not found", "chain", chain.String())
		return operations.SequenceReport[seqevm.SeqGrantRolesTimelockInput, map[uint64][]evmops.EVMCallOutput]{}, fmt.Errorf("timelock contracts not found for chain %s", chain.String())
	}

	timelock := timelockContracts.Timelock
	proposer := timelockContracts.ProposerMcm
	canceller := timelockContracts.CancellerMcm
	bypasser := timelockContracts.BypasserMcm
	callProxy := timelockContracts.CallProxy

	// get admin addresses
	adminAddresses, err := getAdminAddresses(ctx, timelock)
	if err != nil {
		return operations.SequenceReport[seqevm.SeqGrantRolesTimelockInput, map[uint64][]evmops.EVMCallOutput]{}, fmt.Errorf("failed to get admin addresses: %w", err)
	}
	isDeployerKeyAdmin := slices.Contains(adminAddresses, chain.DeployerKey.From.String())
	isTimelockAdmin := slices.Contains(adminAddresses, timelock.Address().String())
	if !isDeployerKeyAdmin && skipIfDeployerKeyNotAdmin {
		lggr.Infow("Deployer key is not admin, skipping role grants", "chain", chain.String())
		return operations.SequenceReport[seqevm.SeqGrantRolesTimelockInput, map[uint64][]evmops.EVMCallOutput]{}, nil
	}
	if !isDeployerKeyAdmin && !isTimelockAdmin {
		return operations.SequenceReport[seqevm.SeqGrantRolesTimelockInput, map[uint64][]evmops.EVMCallOutput]{}, errors.New("neither deployer key nor timelock is admin, cannot grant roles")
	}

	seqDeps := seqevm.SeqGrantRolesTimelockDeps{
		Chain: chain,
	}

	seqInput := seqevm.SeqGrantRolesTimelockInput{
		ContractType:       mcmscontracts.RBACTimelock,
		ChainSelector:      chain.Selector,
		Timelock:           timelock.Address(),
		IsDeployerKeyAdmin: isDeployerKeyAdmin,
		RolesAndAddresses: []seqevm.RolesAndAddresses{
			{
				Role:      mcmsrole.ProposerRole.ID,
				Name:      mcmsrole.ProposerRole.Name,
				Addresses: []common.Address{proposer.Address()},
			},
			{
				Role:      mcmsrole.CancellerRole.ID,
				Name:      mcmsrole.CancellerRole.Name,
				Addresses: []common.Address{proposer.Address(), canceller.Address(), bypasser.Address()},
			},
			{
				Role:      mcmsrole.BypasserRole.ID,
				Name:      mcmsrole.BypasserRole.Name,
				Addresses: []common.Address{bypasser.Address()},
			},
			{
				Role:      mcmsrole.ExecutorRole.ID,
				Name:      mcmsrole.ExecutorRole.Name,
				Addresses: []common.Address{callProxy.Address()},
			},
		},
		GasBoostConfig: gasBoostConfig,
	}

	if !isTimelockAdmin {
		// We grant the timelock the admin role on the MCMS contracts.
		seqInput.RolesAndAddresses = append(seqInput.RolesAndAddresses, seqevm.RolesAndAddresses{
			Role:      mcmsrole.AdminRole.ID,
			Name:      mcmsrole.AdminRole.Name,
			Addresses: []common.Address{timelock.Address()},
		})
	}

	report, err := operations.ExecuteSequence(
		env.OperationsBundle,
		seqevm.SeqGrantRolesTimelock,
		seqDeps,
		seqInput,
	)
	if err != nil {
		lggr.Errorw("Failed to grant roles for timelock", "chain", chain.String(), "err", err)
		return operations.SequenceReport[seqevm.SeqGrantRolesTimelockInput, map[uint64][]evmops.EVMCallOutput]{}, err
	}

	return report, nil
}

// TODO: delete this function after it is available in timelock Inspector
func getAdminAddresses(ctx context.Context, timelock *bindings.RBACTimelock) ([]string, error) {
	numAddresses, err := timelock.GetRoleMemberCount(&bind.CallOpts{
		Context: ctx,
	}, mcmsrole.AdminRole.ID)
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
		}, mcmsrole.AdminRole.ID, big.NewInt(idx))
		if err != nil {
			return nil, err
		}
		adminAddresses = append(adminAddresses, address.String())
	}

	return adminAddresses, nil
}
