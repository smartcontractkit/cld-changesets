package evmgrantrole

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmsevm "github.com/smartcontractkit/mcms/sdk/evm"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
)

var seqGrantRole = operations.NewSequence(
	"seq-evm-grant-role",
	semver.MustParse("1.0.0"),
	"Grants RBACTimelock roles on EVM chains",
	runEVMGrantRole,
)

func runEVMGrantRole(
	b operations.Bundle,
	deps grantrole.Deps,
	in grantrole.SeqInput,
) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.EVMChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("EVM chain %d not found in environment", in.ChainSelector)
	}

	timelock, err := timelockAddress(grantrole.EnvFromDeps(deps), in)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}

	useMCMS := in.MCMS != nil
	var writes []opscontract.WriteOutput

	for _, grant := range in.Grants {
		addresses, err := addressesNeedingGrant(b, chain, timelock, grant)
		if err != nil {
			return sequenceutils.OnChainOutput{}, err
		}

		for _, address := range addresses {
			target := GrantRoleTarget{
				Timelock: timelock,
				Role:     grant.Role,
				Address:  address,
			}
			if err := validateGrantRoleTarget(target); err != nil {
				return sequenceutils.OnChainOutput{}, err
			}

			report, err := operations.ExecuteOperation(
				b,
				OpEVMGrantRole,
				chain,
				OpEVMGrantRoleInput{
					Target: target,
					NoSend: useMCMS,
				},
				retryGrantRoleWithGasBoost(in.GasBoostConfig),
				operations.WithIdempotencyKey[OpEVMGrantRoleInput, cldf_evm.Chain](
					strconv.FormatUint(chain.Selector, 10)+":"+timelock.Hex()+":"+grant.Role.String()+":"+address.Hex(),
				),
			)
			if err != nil {
				return sequenceutils.OnChainOutput{}, err
			}

			if useMCMS {
				writes = append(writes, report.Output)
				continue
			}

			if report.Output.Executed() {
				b.Logger.Infow("Role granted",
					"role", grant.Role.String(),
					"chainSelector", chain.Selector,
					"timelock", timelock.Hex(),
					"address", address.Hex(),
					"txHash", report.Output.ExecInfo.Hash,
				)
			}
		}
	}

	if !useMCMS {
		return sequenceutils.OnChainOutput{}, nil
	}

	batch, err := opscontract.NewBatchOperationFromWrites(writes)
	if err != nil {
		return sequenceutils.OnChainOutput{}, err
	}
	if len(batch.Transactions) == 0 {
		return sequenceutils.OnChainOutput{}, nil
	}

	return sequenceutils.OnChainOutput{BatchOps: []mcmstypes.BatchOperation{batch}}, nil
}

// addressesNeedingGrant returns the set of addresses that don't yet have the provided role.
func addressesNeedingGrant(
	b operations.Bundle,
	chain cldf_evm.Chain,
	timelock common.Address,
	grant grantrole.RoleGrant,
) ([]common.Address, error) {
	addressesWithRole, err := addressesForRole(b, chain, timelock, grant.Role)
	if err != nil {
		return nil, err
	}
	if len(addressesWithRole) == 0 {
		return grant.Addresses, nil
	}

	out := make([]common.Address, 0, len(grant.Addresses))
	for _, address := range grant.Addresses {
		if slices.Contains(addressesWithRole, address.Hex()) {
			continue
		}
		out = append(out, address)
	}

	return out, nil
}

func addressesForRole(
	b operations.Bundle,
	chain cldf_evm.Chain,
	timelock common.Address,
	role mcmssdk.TimelockRole,
) ([]string, error) {
	inspector := mcmsevm.NewTimelockInspector(chain.Client)
	switch role {
	case mcmssdk.TimelockRoleProposer:
		return inspector.GetProposers(b.GetContext(), timelock.Hex())
	case mcmssdk.TimelockRoleCanceller:
		return inspector.GetCancellers(b.GetContext(), timelock.Hex())
	case mcmssdk.TimelockRoleBypasser:
		return inspector.GetBypassers(b.GetContext(), timelock.Hex())
	case mcmssdk.TimelockRoleExecutor:
		return inspector.GetExecutors(b.GetContext(), timelock.Hex())
	case mcmssdk.TimelockRoleAdmin:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported timelock role %s", role.String())
	}
}

func timelockAddress(env cldf.Environment, in grantrole.SeqInput) (common.Address, error) {
	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyEVM)
	if !ok {
		return common.Address{}, fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilyEVM)
	}

	input := cldf.MCMSTimelockProposalInput{}
	if in.MCMS != nil {
		input = *in.MCMS
	}
	ref, err := reader.GetTimelockRef(env, in.ChainSelector, input)
	if err != nil {
		return common.Address{}, fmt.Errorf("resolve timelock for chain %d: %w", in.ChainSelector, err)
	}

	return parseEVMAddress(ref.Address, "timelock")
}
