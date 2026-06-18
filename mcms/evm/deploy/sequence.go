package evmdeploy

import (
	"fmt"
	"math"
	"math/big"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	bindings "github.com/smartcontractkit/ccip-owner-contracts/pkg/gethwrappers"
	cldfevm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	opscontract "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm/operations2/contract"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/mcmsrole"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	callproxyops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/call_proxy"
	timelockops "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy/v1_0_0/operations/rbac_timelock"
)

var seqDeployMCMSWithTimelock = operations.NewSequence(
	"seq-mcms-deploy-with-timelock",
	semver.MustParse("1.0.0"),
	"Deploy MCMS and timelock contracts on an EVM chain",
	deployMCMSWithTimelock,
)

// deployer accumulates per-chain deployment state within a single sequence run.
type deployer struct {
	b         operations.Bundle
	chain     cldfevm.Chain
	config    cldfproposalutils.MCMSWithTimelockConfig
	qualifier string
	out       sequenceutils.OnChainOutput
}

func deployMCMSWithTimelock(
	b operations.Bundle,
	deps deploy.Deps,
	in deploy.ChainInput,
) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.EVMChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("EVM chain %d not found in environment", in.ChainSelector)
	}

	qualifier := qualifierFromConfig(in.Config.Qualifier)

	existing := loadDeployedAddresses(deps.DataStore, in.ChainSelector, qualifier)

	d := &deployer{b: b, chain: chain, config: in.Config, qualifier: qualifier}

	var err error
	if existing.Bypasser, err = d.deployMCMIfNeeded(mcmscontracts.BypasserManyChainMultisig, in.Config.Bypasser, existing.Bypasser); err != nil {
		return d.out, err
	}
	if existing.Canceller, err = d.deployMCMIfNeeded(mcmscontracts.CancellerManyChainMultisig, in.Config.Canceller, existing.Canceller); err != nil {
		return d.out, err
	}
	if existing.Proposer, err = d.deployMCMIfNeeded(mcmscontracts.ProposerManyChainMultisig, in.Config.Proposer, existing.Proposer); err != nil {
		return d.out, err
	}
	if existing.Timelock == (common.Address{}) {
		if existing.Timelock, err = d.deployTimelock(existing); err != nil {
			return d.out, err
		}
	}
	if existing.CallProxy == (common.Address{}) {
		if existing.CallProxy, err = d.deployCallProxy(existing.Timelock); err != nil {
			return d.out, err
		}
	}

	if err = d.grantTimelockRoles(existing); err != nil {
		return d.out, err
	}

	return d.out, nil
}

func (d *deployer) deployMCMIfNeeded(
	contractType cldf.ContractType,
	mcmConfig mcmstypes.Config,
	existing common.Address,
) (common.Address, error) {
	if existing != (common.Address{}) {
		return existing, nil
	}

	report, err := operations.ExecuteSequence(
		d.b,
		seqDeployMCMWithConfig,
		d.chain,
		deployMCMWithConfigInput{
			ContractType:   contractType,
			MCMConfig:      mcmConfig,
			Qualifier:      stringPtrIfNonEmpty(d.qualifier),
			GasBoostConfig: d.config.GasBoostConfig,
		},
		chainSequenceIdempotencyKey[deployMCMWithConfigInput, cldfevm.Chain](d.chain),
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("deploy %s: %w", contractType, err)
	}

	ref := addressRefWithLabel(report.Output, labelFromConfig(d.config.Label))
	d.out.Metadata.Addresses = append(d.out.Metadata.Addresses, ref)

	return common.HexToAddress(ref.Address), nil
}

func (d *deployer) deployTimelock(addrs deployedAddresses) (common.Address, error) {
	report, err := operations.ExecuteOperation(
		d.b,
		timelockops.Deploy,
		d.chain,
		opscontract.DeployInput[timelockops.ConstructorArgs]{
			TypeAndVersion: timelockops.TypeAndVersion,
			Qualifier:      stringPtrIfNonEmpty(d.qualifier),
			Args: timelockops.ConstructorArgs{
				MinDelay:   d.config.TimelockMinDelay,
				Admin:      d.chain.DeployerKey.From,
				Proposers:  []common.Address{addrs.Proposer},
				Executors:  []common.Address{},
				Cancellers: []common.Address{addrs.Canceller, addrs.Proposer, addrs.Bypasser},
				Bypassers:  []common.Address{addrs.Bypasser},
			},
		},
		retryDeployWithGasBoost[timelockops.ConstructorArgs](d.config.GasBoostConfig),
		chainIdempotencyKey[opscontract.DeployInput[timelockops.ConstructorArgs], cldfevm.Chain](d.chain),
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("deploy timelock: %w", err)
	}

	ref := addressRefWithLabel(report.Output, labelFromConfig(d.config.Label))
	d.out.Metadata.Addresses = append(d.out.Metadata.Addresses, ref)

	return common.HexToAddress(ref.Address), nil
}

func (d *deployer) deployCallProxy(timelockAddr common.Address) (common.Address, error) {
	report, err := operations.ExecuteOperation(
		d.b,
		callproxyops.Deploy,
		d.chain,
		opscontract.DeployInput[callproxyops.ConstructorArgs]{
			TypeAndVersion: callproxyops.TypeAndVersion,
			Qualifier:      stringPtrIfNonEmpty(d.qualifier),
			Args:           callproxyops.ConstructorArgs{Target: timelockAddr},
		},
		retryDeployWithGasBoost[callproxyops.ConstructorArgs](d.config.GasBoostConfig),
		chainIdempotencyKey[opscontract.DeployInput[callproxyops.ConstructorArgs], cldfevm.Chain](d.chain),
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("deploy call proxy: %w", err)
	}

	ref := addressRefWithLabel(report.Output, labelFromConfig(d.config.Label))
	d.out.Metadata.Addresses = append(d.out.Metadata.Addresses, ref)

	return common.HexToAddress(ref.Address), nil
}

func (d *deployer) grantTimelockRoles(addrs deployedAddresses) error {
	isDeployerAdmin, isTimelockAdmin, err := d.timelockAdminStatus(addrs.Timelock)
	if err != nil {
		return fmt.Errorf("check timelock admin: %w", err)
	}
	if !isDeployerAdmin {
		d.b.Logger.Infow("Deployer key is not timelock admin, skipping role grants",
			"chain", d.chain.String(),
			"timelock", addrs.Timelock.Hex(),
		)

		return nil
	}

	roleGrants := []timelockRoleGrant{
		{
			Role:      mcmsrole.ProposerRole.ID,
			Name:      mcmsrole.ProposerRole.Name,
			Addresses: []common.Address{addrs.Proposer},
		},
		{
			Role:      mcmsrole.CancellerRole.ID,
			Name:      mcmsrole.CancellerRole.Name,
			Addresses: []common.Address{addrs.Proposer, addrs.Canceller, addrs.Bypasser},
		},
		{
			Role:      mcmsrole.BypasserRole.ID,
			Name:      mcmsrole.BypasserRole.Name,
			Addresses: []common.Address{addrs.Bypasser},
		},
		{
			Role:      mcmsrole.ExecutorRole.ID,
			Name:      mcmsrole.ExecutorRole.Name,
			Addresses: []common.Address{addrs.CallProxy},
		},
	}
	if !isTimelockAdmin {
		roleGrants = append(roleGrants, timelockRoleGrant{
			Role:      mcmsrole.AdminRole.ID,
			Name:      mcmsrole.AdminRole.Name,
			Addresses: []common.Address{addrs.Timelock},
		})
	}

	_, err = operations.ExecuteSequence(
		d.b,
		seqGrantRolesTimelock,
		d.chain,
		grantRolesTimelockInput{
			Timelock:       addrs.Timelock,
			RoleGrants:     roleGrants,
			GasBoostConfig: d.config.GasBoostConfig,
		},
		chainSequenceIdempotencyKey[grantRolesTimelockInput, cldfevm.Chain](d.chain),
	)

	return err
}

func (d *deployer) timelockAdminStatus(timelockAddr common.Address) (isDeployerAdmin, isTimelockAdmin bool, err error) {
	admins, err := d.getTimelockAdminAddresses(timelockAddr)
	if err != nil {
		return false, false, err
	}

	for _, admin := range admins {
		if admin == d.chain.DeployerKey.From {
			isDeployerAdmin = true
		}
		if admin == timelockAddr {
			isTimelockAdmin = true
		}
	}

	return isDeployerAdmin, isTimelockAdmin, nil
}

func (d *deployer) getTimelockAdminAddresses(timelockAddr common.Address) ([]common.Address, error) {
	timelock, err := bindings.NewRBACTimelock(timelockAddr, d.chain.Client)
	if err != nil {
		return nil, fmt.Errorf("bind timelock: %w", err)
	}

	callOpts := &bind.CallOpts{Context: d.b.GetContext()}
	count, err := timelock.GetRoleMemberCount(callOpts, mcmsrole.AdminRole.ID)
	if err != nil {
		return nil, fmt.Errorf("get admin count: %w", err)
	}

	n := count.Uint64()
	if n > uint64(math.MaxInt) {
		return nil, fmt.Errorf("admin count %d exceeds int range", n)
	}
	adminCount := int(n)
	admins := make([]common.Address, 0, adminCount)
	for i := range adminCount {
		member, err := timelock.GetRoleMember(callOpts, mcmsrole.AdminRole.ID, big.NewInt(int64(i)))
		if err != nil {
			return nil, fmt.Errorf("get admin member %d: %w", i, err)
		}
		admins = append(admins, member)
	}

	return admins, nil
}

func qualifierFromConfig(q *string) string {
	if q == nil {
		return ""
	}

	return *q
}

func labelFromConfig(l *string) string {
	if l == nil {
		return ""
	}

	return *l
}

func stringPtrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
