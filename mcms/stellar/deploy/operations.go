package stellardeploy

import (
	"fmt"

	chainsel "github.com/smartcontractkit/chain-selectors"
	mcmsstellar "github.com/smartcontractkit/mcms/sdk/stellar"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"
	stellarcre "github.com/smartcontractkit/chainlink-stellar/deployment/cre"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

type deployMCMInput struct {
	ContractType cldf.ContractType `json:"contractType"`
	Config       mcmstypes.Config  `json:"config"`
	Qualifier    string            `json:"qualifier"`
	Label        string            `json:"label"`
}

// opDeployMCM deploys (or adopts) one Stellar MCMS contract at its
// deterministic address and ensures it is initialized with the target config.
var opDeployMCM = operations.NewOperation(
	"op-stellar-deploy-mcm",
	&semvers.V1_0_0,
	"Deploys and initializes a Stellar MCMS contract",
	deployStellarMCM,
)

func deployStellarMCM(
	b operations.Bundle,
	chain cldfstellar.Chain,
	in deployMCMInput,
) (datastore.AddressRef, error) {
	deployer, err := stellardeployment.NewDeployerFromChain(chain)
	if err != nil {
		return datastore.AddressRef{}, fmt.Errorf(
			"create Stellar deployer for chain %d: %w",
			chain.Selector,
			err,
		)
	}

	chainID, err := chainsel.StellarChainIdFromSelector(chain.Selector)
	if err != nil {
		return datastore.AddressRef{}, fmt.Errorf(
			"resolve Stellar chain ID for selector %d: %w",
			chain.Selector,
			err,
		)
	}

	wasm, err := stellarcre.Artifact(stellarcre.MCMSWasm)
	if err != nil {
		return datastore.AddressRef{}, fmt.Errorf(
			"source Stellar MCMS WASM: %w",
			err,
		)
	}

	identity, err := resolveDeploymentIdentity(
		b.GetContext(),
		chain.Client,
		chain.NetworkPassphrase,
		deployer.SignerAddress(),
		in.ContractType,
		in.Qualifier,
		wasm,
	)
	if err != nil {
		return datastore.AddressRef{}, err
	}

	inspector := mcmsstellar.NewInspectorFromInvoker(deployer)
	configurer := mcmsstellar.NewConfigurer(deployer)

	contractID := identity.ContractID
	if !identity.Existing {
		contractID, err = deployer.DeployContractBytes(
			b.GetContext(),
			wasm,
			identity.Salt,
		)
		if err != nil {
			return datastore.AddressRef{}, fmt.Errorf(
				"deploy %s: %w",
				in.ContractType,
				err,
			)
		}

		if contractID != identity.ContractID {
			return datastore.AddressRef{}, fmt.Errorf(
				"deploy %s: expected deterministic contract ID %s, got %s",
				in.ContractType,
				identity.ContractID,
				contractID,
			)
		}
	} else {
		b.Logger.Infow(
			"Adopted existing deterministic Stellar MCMS deployment",
			"chainSelector", chain.Selector,
			"contractType", in.ContractType,
			"contractID", contractID,
			"qualifier", in.Qualifier,
			"saltAttempt", identity.Attempt,
		)
	}

	owner, err := inspector.GetOwner(b.GetContext(), contractID)
	if err != nil {
		return datastore.AddressRef{}, fmt.Errorf(
			"read owner for Stellar MCMS %s: %w",
			contractID,
			err,
		)
	}

	if owner == nil {
		instanceLabel, err := mcmInstanceLabel(in.ContractType)
		if err != nil {
			return datastore.AddressRef{}, err
		}

		err = initializeMCMS(
			b.GetContext(),
			deployer,
			initializeMCMSInput{
				ContractID:    contractID,
				Owner:         deployer.SignerAddress(),
				ChainID:       chainID,
				Config:        &in.Config,
				InstanceLabel: instanceLabel,
			},
		)
		if err != nil {
			return datastore.AddressRef{}, fmt.Errorf(
				"initialize %s at %s: %w",
				in.ContractType,
				contractID,
				err,
			)
		}
	} else {
		currentConfig, err := inspector.GetConfig(b.GetContext(), contractID)
		if err != nil {
			return datastore.AddressRef{}, fmt.Errorf(
				"read config for Stellar MCMS %s: %w",
				contractID,
				err,
			)
		}

		if !in.Config.Equals(currentConfig) {
			if *owner != deployer.SignerAddress() {
				return datastore.AddressRef{}, fmt.Errorf(
					"stellar MCMS %s config differs but owner %s does not match deployer %s",
					contractID,
					*owner,
					deployer.SignerAddress(),
				)
			}

			_, err = configurer.SetConfig(
				b.GetContext(),
				contractID,
				&in.Config,
				true,
			)
			if err != nil {
				return datastore.AddressRef{}, fmt.Errorf(
					"set config for %s at %s: %w",
					in.ContractType,
					contractID,
					err,
				)
			}
		}
	}

	return newAddressRef(
		chain.Selector,
		in.ContractType,
		contractID,
		in.Qualifier,
		in.Label,
	), nil
}

type deployTimelockInput struct {
	MinDelay  uint64 `json:"minDelay"`
	Proposer  string `json:"proposer"`
	Canceller string `json:"canceller"`
	Bypasser  string `json:"bypasser"`
	Qualifier string `json:"qualifier"`
	Label     string `json:"label"`
}

// opDeployTimelock deploys (or adopts) the Stellar RBAC timelock at its
// deterministic address and ensures it is initialized with the MCMS role
// topology.
var opDeployTimelock = operations.NewOperation(
	"op-stellar-deploy-timelock",
	&semvers.V1_0_0,
	"Deploys and initializes the Stellar RBAC timelock",
	deployStellarTimelock,
)

func deployStellarTimelock(
	b operations.Bundle,
	chain cldfstellar.Chain,
	in deployTimelockInput,
) (datastore.AddressRef, error) {
	deployer, err := stellardeployment.NewDeployerFromChain(chain)
	if err != nil {
		return datastore.AddressRef{}, fmt.Errorf(
			"create Stellar deployer for chain %d: %w",
			chain.Selector,
			err,
		)
	}

	wasm, err := stellarcre.Artifact(stellarcre.TimelockWasm)
	if err != nil {
		return datastore.AddressRef{}, fmt.Errorf(
			"source Stellar timelock WASM: %w",
			err,
		)
	}

	identity, err := resolveDeploymentIdentity(
		b.GetContext(),
		chain.Client,
		chain.NetworkPassphrase,
		deployer.SignerAddress(),
		mcmscontracts.RBACTimelock,
		in.Qualifier,
		wasm,
	)
	if err != nil {
		return datastore.AddressRef{}, err
	}

	timelockInspector := mcmsstellar.NewTimelockInspectorFromInvoker(deployer)

	contractID := identity.ContractID
	if !identity.Existing {
		contractID, err = deployer.DeployContractBytes(
			b.GetContext(),
			wasm,
			identity.Salt,
		)
		if err != nil {
			return datastore.AddressRef{}, fmt.Errorf(
				"deploy timelock: %w",
				err,
			)
		}

		if contractID != identity.ContractID {
			return datastore.AddressRef{}, fmt.Errorf(
				"deploy timelock: expected deterministic contract ID %s, got %s",
				identity.ContractID,
				contractID,
			)
		}
	} else {
		b.Logger.Infow(
			"Adopted existing deterministic Stellar timelock deployment",
			"chainSelector", chain.Selector,
			"contractID", contractID,
			"qualifier", in.Qualifier,
			"saltAttempt", identity.Attempt,
		)
	}

	initialized := false
	if identity.Existing {
		initialized, err = timelockInspector.IsInitialized(
			b.GetContext(),
			contractID,
		)
		if err != nil {
			return datastore.AddressRef{}, fmt.Errorf(
				"inspect Stellar timelock %s initialization: %w",
				contractID,
				err,
			)
		}
	}

	if !initialized {
		err = initializeTimelock(
			b.GetContext(),
			deployer,
			initializeTimelockInput{
				ContractID: contractID,
				MinDelay:   in.MinDelay,
				Proposers: []string{
					in.Proposer,
				},
				Cancellers: []string{
					in.Proposer,
					in.Canceller,
					in.Bypasser,
				},
				Bypassers: []string{
					in.Bypasser,
				},
			},
		)
		if err != nil {
			return datastore.AddressRef{}, fmt.Errorf(
				"initialize timelock at %s: %w",
				contractID,
				err,
			)
		}
	}

	return newAddressRef(
		chain.Selector,
		mcmscontracts.RBACTimelock,
		contractID,
		in.Qualifier,
		in.Label,
	), nil
}
