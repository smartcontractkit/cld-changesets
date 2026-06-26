package soldeploy

import (
	"fmt"
	"math/big"

	"github.com/Masterminds/semver/v3"
	solanago "github.com/gagliardetto/solana-go"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	solutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
)

var seqDeployMCMSWithTimelock = operations.NewSequence(
	"seq-solana-mcms-deploy-with-timelock",
	semver.MustParse("1.0.0"),
	"Deploy MCMS and timelock programs on a Solana chain",
	deployMCMSWithTimelock,
)

// deployer accumulates per-chain deployment state within a single sequence run.
type deployer struct {
	b         operations.Bundle
	chain     cldfsol.Chain
	config    cldfproposalutils.MCMSWithTimelockConfig
	qualifier string
	label     string
	existing  deployedAddresses
	out       sequenceutils.OnChainOutput
}

func deployMCMSWithTimelock(
	b operations.Bundle,
	deps deploy.Deps,
	in deploy.ChainInput,
) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.SolanaChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("solana chain %d not found in environment", in.ChainSelector)
	}

	d := &deployer{
		b:         b,
		chain:     chain,
		config:    in.Config,
		qualifier: stringFromPtr(in.Config.Qualifier),
		label:     stringFromPtr(in.Config.Label),
	}

	var err error
	d.existing, err = loadDeployedAddresses(deps.DataStore, in.ChainSelector, d.qualifier)
	if err != nil {
		return sequenceutils.OnChainOutput{}, fmt.Errorf("load deployed addresses: %w", err)
	}

	// 1. Access controller program + accounts
	if err := d.deployAccessControllerProgramIfNeeded(); err != nil {
		return d.out, err
	}
	for _, role := range []cldf.ContractType{
		mcmscontracts.ProposerAccessControllerAccount,
		mcmscontracts.ExecutorAccessControllerAccount,
		mcmscontracts.CancellerAccessControllerAccount,
		mcmscontracts.BypasserAccessControllerAccount,
	} {
		if err := d.initAccessControllerAccountIfNeeded(role); err != nil {
			return d.out, err
		}
	}

	// 2. MCM program + instances
	if err := d.deployMCMProgramIfNeeded(); err != nil {
		return d.out, err
	}
	for _, r := range []struct {
		contractType cldf.ContractType
		mcmConfig    mcmstypes.Config
		hasFn        func() bool
		seedDst      *legacysolana.PDASeed
	}{
		{mcmscontracts.BypasserManyChainMultisig, in.Config.Bypasser, d.existing.hasBypasserMCM, &d.existing.BypasserMCMSeed},
		{mcmscontracts.CancellerManyChainMultisig, in.Config.Canceller, d.existing.hasCancellerMCM, &d.existing.CancellerMCMSeed},
		{mcmscontracts.ProposerManyChainMultisig, in.Config.Proposer, d.existing.hasProposerMCM, &d.existing.ProposerMCMSeed},
	} {
		if r.hasFn() {
			continue
		}
		seed, err := d.initMCMInstance(r.contractType, r.mcmConfig)
		if err != nil {
			return d.out, err
		}
		*r.seedDst = seed
	}

	// 3. Timelock program + instance
	if err := d.deployTimelockProgramIfNeeded(); err != nil {
		return d.out, err
	}
	if !d.existing.hasTimelock() {
		if err := d.initTimelock(); err != nil {
			return d.out, err
		}
	}

	// 4. Role grants
	if err := d.setupTimelockRoles(); err != nil {
		return d.out, err
	}

	return d.out, nil
}

// ─── Access controller ────────────────────────────────────────────────────────

func (d *deployer) deployAccessControllerProgramIfNeeded() error {
	if !d.existing.AccessControllerProgram.IsZero() {
		return nil
	}

	report, err := operations.ExecuteOperation(
		d.b,
		opDeployProgram,
		d.chain,
		deployProgramInput{
			ProgramName:  solutils.ProgAccessController,
			ContractType: mcmscontracts.AccessControllerProgram,
			Qualifier:    d.qualifier,
			Label:        d.label,
		},
		chainIdempotencyKey[deployProgramInput, cldfsol.Chain](d.chain),
	)
	if err != nil {
		return fmt.Errorf("deploy access controller program: %w", err)
	}

	ref := report.Output
	pk, err := solanaPubkeyFromRef(ref)
	if err != nil {
		return fmt.Errorf("parse access controller program address: %w", err)
	}
	d.existing.AccessControllerProgram = pk
	d.out.Metadata.Addresses = append(d.out.Metadata.Addresses, ref)

	return nil
}

func (d *deployer) initAccessControllerAccountIfNeeded(contractType cldf.ContractType) error {
	dest := d.accessControllerAccountPtr(contractType)
	if dest == nil || !dest.IsZero() {
		return nil
	}

	report, err := operations.ExecuteOperation(
		d.b,
		opInitAccessControllerAccount,
		d.chain,
		initACAccountInput{
			ProgramID:    d.existing.AccessControllerProgram,
			ContractType: contractType,
			Qualifier:    d.qualifier,
			Label:        d.label,
		},
		chainIdempotencyKey[initACAccountInput, cldfsol.Chain](d.chain),
	)
	if err != nil {
		return fmt.Errorf("init %s: %w", contractType, err)
	}

	ref := report.Output
	pk, err := solanaPubkeyFromRef(ref)
	if err != nil {
		return fmt.Errorf("parse %s address: %w", contractType, err)
	}
	*dest = pk
	d.out.Metadata.Addresses = append(d.out.Metadata.Addresses, ref)

	return nil
}

func (d *deployer) accessControllerAccountPtr(contractType cldf.ContractType) *solanago.PublicKey {
	switch contractType {
	case mcmscontracts.ProposerAccessControllerAccount:
		return &d.existing.ProposerAccessControllerAccount
	case mcmscontracts.ExecutorAccessControllerAccount:
		return &d.existing.ExecutorAccessControllerAccount
	case mcmscontracts.CancellerAccessControllerAccount:
		return &d.existing.CancellerAccessControllerAccount
	case mcmscontracts.BypasserAccessControllerAccount:
		return &d.existing.BypasserAccessControllerAccount
	default:
		return nil
	}
}

// ─── MCM program + instances ──────────────────────────────────────────────────

func (d *deployer) deployMCMProgramIfNeeded() error {
	if !d.existing.McmProgram.IsZero() {
		return nil
	}

	report, err := operations.ExecuteOperation(
		d.b,
		opDeployProgram,
		d.chain,
		deployProgramInput{
			ProgramName:  solutils.ProgMCM,
			ContractType: mcmscontracts.ManyChainMultisigProgram,
			Qualifier:    d.qualifier,
			Label:        d.label,
		},
		chainIdempotencyKey[deployProgramInput, cldfsol.Chain](d.chain),
	)
	if err != nil {
		return fmt.Errorf("deploy mcm program: %w", err)
	}

	ref := report.Output
	pk, err := solanaPubkeyFromRef(ref)
	if err != nil {
		return fmt.Errorf("parse mcm program address: %w", err)
	}
	d.existing.McmProgram = pk
	d.out.Metadata.Addresses = append(d.out.Metadata.Addresses, ref)

	return nil
}

func (d *deployer) initMCMInstance(contractType cldf.ContractType, mcmConfig mcmstypes.Config) (legacysolana.PDASeed, error) {
	report, err := operations.ExecuteOperation(
		d.b,
		opInitMCMInstance,
		d.chain,
		initMCMInstanceInput{
			McmProgram:   d.existing.McmProgram,
			ContractType: contractType,
			MCMConfig:    mcmConfig,
			Qualifier:    d.qualifier,
			Label:        d.label,
		},
		chainIdempotencyKey[initMCMInstanceInput, cldfsol.Chain](d.chain),
	)
	if err != nil {
		return legacysolana.PDASeed{}, fmt.Errorf("initialize %s: %w", contractType, err)
	}

	d.out.Metadata.Addresses = append(d.out.Metadata.Addresses, report.Output.Ref)

	return report.Output.Seed, nil
}

// ─── Timelock program + instance ─────────────────────────────────────────────

func (d *deployer) deployTimelockProgramIfNeeded() error {
	if !d.existing.TimelockProgram.IsZero() {
		return nil
	}

	report, err := operations.ExecuteOperation(
		d.b,
		opDeployProgram,
		d.chain,
		deployProgramInput{
			ProgramName:  solutils.ProgTimelock,
			ContractType: mcmscontracts.RBACTimelockProgram,
			Qualifier:    d.qualifier,
			Label:        d.label,
		},
		chainIdempotencyKey[deployProgramInput, cldfsol.Chain](d.chain),
	)
	if err != nil {
		return fmt.Errorf("deploy timelock program: %w", err)
	}

	ref := report.Output
	pk, err := solanaPubkeyFromRef(ref)
	if err != nil {
		return fmt.Errorf("parse timelock program address: %w", err)
	}
	d.existing.TimelockProgram = pk
	d.out.Metadata.Addresses = append(d.out.Metadata.Addresses, ref)

	return nil
}

func (d *deployer) initTimelock() error {
	minDelay := d.config.TimelockMinDelay
	if minDelay == nil {
		minDelay = big.NewInt(0)
	}

	report, err := operations.ExecuteOperation(
		d.b,
		opInitTimelockInstance,
		d.chain,
		initTimelockInstanceInput{
			TimelockProgram:         d.existing.TimelockProgram,
			AccessControllerProgram: d.existing.AccessControllerProgram,
			ProposerAC:              d.existing.ProposerAccessControllerAccount,
			ExecutorAC:              d.existing.ExecutorAccessControllerAccount,
			CancellerAC:             d.existing.CancellerAccessControllerAccount,
			BypasserAC:              d.existing.BypasserAccessControllerAccount,
			MinDelay:                minDelay,
			Qualifier:               d.qualifier,
			Label:                   d.label,
		},
		chainIdempotencyKey[initTimelockInstanceInput, cldfsol.Chain](d.chain),
	)
	if err != nil {
		return fmt.Errorf("initialize timelock: %w", err)
	}

	out := report.Output
	d.existing.TimelockSeed = out.Seed
	d.out.Metadata.Addresses = append(d.out.Metadata.Addresses, out.Ref)

	return nil
}

// ─── Role grants ──────────────────────────────────────────────────────────────

func (d *deployer) setupTimelockRoles() error {
	_, err := operations.ExecuteOperation(
		d.b,
		opSetupTimelockRoles,
		d.chain,
		setupTimelockRolesInput{
			McmProgram:              d.existing.McmProgram,
			ProposerMCMSeed:         d.existing.ProposerMCMSeed,
			CancellerMCMSeed:        d.existing.CancellerMCMSeed,
			BypasserMCMSeed:         d.existing.BypasserMCMSeed,
			TimelockProgram:         d.existing.TimelockProgram,
			TimelockSeed:            d.existing.TimelockSeed,
			AccessControllerProgram: d.existing.AccessControllerProgram,
			ProposerAC:              d.existing.ProposerAccessControllerAccount,
			ExecutorAC:              d.existing.ExecutorAccessControllerAccount,
			CancellerAC:             d.existing.CancellerAccessControllerAccount,
			BypasserAC:              d.existing.BypasserAccessControllerAccount,
		},
		chainIdempotencyKey[setupTimelockRolesInput, cldfsol.Chain](d.chain),
	)
	if err != nil {
		return fmt.Errorf("setup timelock roles: %w", err)
	}

	return nil
}

// solanaPubkeyFromRef parses the address field of an AddressRef as a Solana
// public key. Used after ExecuteOperation returns a ref built inside an
// Operation func.
func solanaPubkeyFromRef(ref cldfdatastore.AddressRef) (solanago.PublicKey, error) {
	return solanago.PublicKeyFromBase58(ref.Address)
}

func stringFromPtr(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
