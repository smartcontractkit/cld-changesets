package stellardeploy

import (
	"fmt"

	mcmstypes "github.com/smartcontractkit/mcms/types"

	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
)

var seqDeployMCMSWithTimelock = operations.NewSequence(
	"seq-stellar-deploy-mcms-with-timelock",
	&semvers.V1_0_0,
	"Deploys and initializes three Stellar MCMS contracts and one timelock",
	runStellarDeployMCMSWithTimelock,
)

func runStellarDeployMCMSWithTimelock(
	b operations.Bundle,
	deps deploy.Deps,
	in deploy.ChainInput,
) (sequenceutils.OnChainOutput, error) {
	chain, ok := deps.BlockChains.StellarChains()[in.ChainSelector]
	if !ok {
		return sequenceutils.OnChainOutput{}, fmt.Errorf(
			"stellar chain %d not found in environment",
			in.ChainSelector,
		)
	}

	qualifier := ""
	if in.Config.Qualifier != nil {
		qualifier = *in.Config.Qualifier
	}

	label := ""
	if in.Config.Label != nil {
		label = *in.Config.Label
	}

	var minDelay uint64
	if in.Config.TimelockMinDelay != nil {
		minDelay = in.Config.TimelockMinDelay.Uint64()
	}

	var out sequenceutils.OnChainOutput

	proposer, discovered, err := ensureMCM(
		b,
		chain,
		deps.DataStore,
		qualifier,
		label,
		mcmscontracts.ProposerManyChainMultisig,
		in.Config.Proposer,
	)
	if err != nil {
		return out, fmt.Errorf("deploy/initialize Stellar proposer MCMS: %w", err)
	}
	if discovered {
		out.Metadata.Addresses = append(out.Metadata.Addresses, proposer)
	}

	canceller, discovered, err := ensureMCM(
		b,
		chain,
		deps.DataStore,
		qualifier,
		label,
		mcmscontracts.CancellerManyChainMultisig,
		in.Config.Canceller,
	)
	if err != nil {
		return out, fmt.Errorf("deploy/initialize Stellar canceller MCMS: %w", err)
	}
	if discovered {
		out.Metadata.Addresses = append(out.Metadata.Addresses, canceller)
	}

	bypasser, discovered, err := ensureMCM(
		b,
		chain,
		deps.DataStore,
		qualifier,
		label,
		mcmscontracts.BypasserManyChainMultisig,
		in.Config.Bypasser,
	)
	if err != nil {
		return out, fmt.Errorf("deploy/initialize Stellar bypasser MCMS: %w", err)
	}
	if discovered {
		out.Metadata.Addresses = append(out.Metadata.Addresses, bypasser)
	}

	if err := validateDistinctMCMs(proposer, canceller, bypasser); err != nil {
		return out, err
	}

	timelock, discovered, err := ensureTimelock(
		b,
		chain,
		deps.DataStore,
		qualifier,
		label,
		minDelay,
		proposer.Address,
		canceller.Address,
		bypasser.Address,
	)
	if err != nil {
		return out, fmt.Errorf("deploy/initialize Stellar timelock: %w", err)
	}
	if discovered {
		out.Metadata.Addresses = append(out.Metadata.Addresses, timelock)
	}

	return out, nil
}

func ensureMCM(
	b operations.Bundle,
	chain cldfstellar.Chain,
	ds datastore.DataStore,
	qualifier string,
	label string,
	contractType cldf.ContractType,
	config mcmstypes.Config,
) (datastore.AddressRef, bool, error) {
	ref, exists, err := loadAddressRef(ds, chain.Selector, contractType, qualifier)
	if err != nil {
		return datastore.AddressRef{}, false, err
	}
	if exists {
		b.Logger.Infow(
			"Using existing Stellar MCMS deployment",
			"chainSelector", chain.Selector,
			"contractType", contractType,
			"contractID", ref.Address,
			"qualifier", qualifier,
		)

		return ref, false, nil
	}

	report, err := operations.ExecuteOperation(
		b,
		opDeployMCM,
		chain,
		deployMCMInput{
			ContractType: contractType,
			Config:       config,
			Qualifier:    qualifier,
			Label:        label,
		},
		chainIdempotencyKey[deployMCMInput, cldfstellar.Chain](chain),
	)
	if err != nil {
		return datastore.AddressRef{}, false, err
	}

	return report.Output, true, nil
}

func ensureTimelock(
	b operations.Bundle,
	chain cldfstellar.Chain,
	ds datastore.DataStore,
	qualifier string,
	label string,
	minDelay uint64,
	proposer string,
	canceller string,
	bypasser string,
) (datastore.AddressRef, bool, error) {
	ref, exists, err := loadAddressRef(
		ds,
		chain.Selector,
		mcmscontracts.RBACTimelock,
		qualifier,
	)
	if err != nil {
		return datastore.AddressRef{}, false, err
	}
	if exists {
		b.Logger.Infow(
			"Using existing Stellar timelock deployment",
			"chainSelector", chain.Selector,
			"contractID", ref.Address,
			"qualifier", qualifier,
		)

		return ref, false, nil
	}

	report, err := operations.ExecuteOperation(
		b,
		opDeployTimelock,
		chain,
		deployTimelockInput{
			MinDelay:  minDelay,
			Proposer:  proposer,
			Canceller: canceller,
			Bypasser:  bypasser,
			Qualifier: qualifier,
			Label:     label,
		},
		chainIdempotencyKey[deployTimelockInput, cldfstellar.Chain](chain),
	)
	if err != nil {
		return datastore.AddressRef{}, false, err
	}

	return report.Output, true, nil
}

func mcmInstanceLabel(contractType cldf.ContractType) (string, error) {
	switch contractType {
	case mcmscontracts.ProposerManyChainMultisig:
		return "proposer", nil
	case mcmscontracts.CancellerManyChainMultisig:
		return "canceller", nil
	case mcmscontracts.BypasserManyChainMultisig:
		return "bypasser", nil
	default:
		return "", fmt.Errorf(
			"unsupported Stellar MCMS contract type %s",
			contractType,
		)
	}
}

func validateDistinctMCMs(
	proposer datastore.AddressRef,
	canceller datastore.AddressRef,
	bypasser datastore.AddressRef,
) error {
	refs := []struct {
		role string
		ref  datastore.AddressRef
	}{
		{role: "proposer", ref: proposer},
		{role: "canceller", ref: canceller},
		{role: "bypasser", ref: bypasser},
	}

	seen := make(map[string]string, len(refs))
	for _, item := range refs {
		if previous, ok := seen[item.ref.Address]; ok {
			return fmt.Errorf(
				"stellar %s and %s MCMS resolve to the same contract %s",
				previous,
				item.role,
				item.ref.Address,
			)
		}

		seen[item.ref.Address] = item.role
	}

	return nil
}
