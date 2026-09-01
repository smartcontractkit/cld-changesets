package stellarsetconfig

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	mcmsstellar "github.com/smartcontractkit/mcms/sdk/stellar"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stellar/go-stellar-sdk/xdr"

	cldfstellar "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-stellar/bindings/scval"
	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

const setConfigFunction = "set_config"

var OpStellarSetConfigMCM = operations.NewOperation(
	"op-stellar-set-config-mcms",
	&semvers.V1_0_0,
	"Sets MCMS config on a Stellar contract",
	setStellarMCMConfig,
)

type MCMSetConfigTarget struct {
	Address      string           `json:"address"`
	Config       mcmstypes.Config `json:"config"`
	ContractType string           `json:"contractType"`
}

type OpStellarSetConfigInput struct {
	Target MCMSetConfigTarget `json:"target"`
	NoSend bool               `json:"noSend"`
}

type OpStellarSetConfigOutput struct {
	BatchOperation *mcmstypes.BatchOperation `json:"batchOperation,omitempty"`
}

func setStellarMCMConfig(
	b operations.Bundle,
	chain cldfstellar.Chain,
	in OpStellarSetConfigInput,
) (OpStellarSetConfigOutput, error) {
	if in.Target.Address == "" {
		return OpStellarSetConfigOutput{}, errors.New(
			"stellar set-config: target address is empty",
		)
	}

	if in.Target.ContractType == "" {
		return OpStellarSetConfigOutput{}, errors.New(
			"stellar set-config: contract type is empty",
		)
	}

	if err := in.Target.Config.Validate(); err != nil {
		return OpStellarSetConfigOutput{}, fmt.Errorf(
			"stellar set-config: invalid config: %w",
			err,
		)
	}

	deployer, err := stellardeployment.NewDeployerFromChain(chain)
	if err != nil {
		return OpStellarSetConfigOutput{}, fmt.Errorf(
			"create stellar deployer for chain %d: %w",
			chain.Selector,
			err,
		)
	}

	inspector := mcmsstellar.NewInspectorFromInvoker(deployer)

	currentConfig, err := inspector.GetConfig(
		b.GetContext(),
		in.Target.Address,
	)
	if err != nil {
		return OpStellarSetConfigOutput{}, fmt.Errorf(
			"read current Stellar MCMS config for %s on chain %d: %w",
			in.Target.Address,
			chain.Selector,
			err,
		)
	}

	const clearRoot = true

	if in.Target.Config.Equals(currentConfig) {
		root, _, rootErr := inspector.GetRoot(
			b.GetContext(),
			in.Target.Address,
		)
		if rootErr != nil {
			return OpStellarSetConfigOutput{}, fmt.Errorf(
				"read Stellar MCMS root for %s on chain %d: %w",
				in.Target.Address,
				chain.Selector,
				rootErr,
			)
		}

		if root == (common.Hash{}) {
			b.Logger.Infow(
				"Stellar MCMS config already matches and root is clear; skipping update",
				"chainSelector",
				chain.Selector,
				"contractID",
				in.Target.Address,
			)

			return OpStellarSetConfigOutput{}, nil
		}
	}

	if !in.NoSend {
		_, err = mcmsstellar.NewConfigurer(deployer).SetConfig(
			b.GetContext(),
			in.Target.Address,
			&in.Target.Config,
			clearRoot,
		)
		if err != nil {
			return OpStellarSetConfigOutput{}, fmt.Errorf(
				"set Stellar MCMS config for %s: %w",
				in.Target.Address,
				err,
			)
		}

		b.Logger.Infow(
			"Updated Stellar MCMS config",
			"chainSelector",
			chain.Selector,
			"contractID",
			in.Target.Address,
		)

		return OpStellarSetConfigOutput{}, nil
	}

	signerAddresses, signerGroups, groupQuorums, groupParents, err :=
		mcmsstellar.ConfigToSetConfigInputs(&in.Target.Config)
	if err != nil {
		return OpStellarSetConfigOutput{}, fmt.Errorf(
			"convert Stellar MCMS config for %s: %w",
			in.Target.Address,
			err,
		)
	}

	args := []xdr.ScVal{
		scval.MustToScVal(signerAddresses.ToScVal()),
		scval.MustToScVal(signerGroups.ToScVal()),
		scval.Bytes32ToScVal(groupQuorums),
		scval.Bytes32ToScVal(groupParents),
		scval.BoolToScVal(clearRoot),
	}

	batchOp, err := mcmsstellar.NewBatchOperation(
		mcmstypes.ChainSelector(chain.Selector),
		in.Target.Address,
		setConfigFunction,
		args,
		in.Target.ContractType,
		nil,
	)
	if err != nil {
		return OpStellarSetConfigOutput{}, fmt.Errorf(
			"build Stellar set_config batch operation for %s: %w",
			in.Target.Address,
			err,
		)
	}

	return OpStellarSetConfigOutput{
		BatchOperation: &batchOp,
	}, nil
}
