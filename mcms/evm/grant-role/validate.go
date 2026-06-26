package evmgrantrole

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
	evmreaders "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
)

func validateEVMChains(env cldf.Environment, chains []grantrole.SeqInput) error {
	for _, in := range chains {
		if _, ok := env.BlockChains.EVMChains()[in.ChainSelector]; !ok {
			return fmt.Errorf("EVM chain %d not found in environment", in.ChainSelector)
		}
		if err := validateMCMSRefs(env, in); err != nil {
			return err
		}
		if err := validateRoles(in); err != nil {
			return fmt.Errorf("chain %d: %w", in.ChainSelector, err)
		}
	}

	return nil
}

func validateMCMSRefs(env cldf.Environment, in grantrole.SeqInput) error {
	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyEVM)
	if !ok {
		return fmt.Errorf("no MCMS reader registered for family %q", chainselectors.FamilyEVM)
	}

	input := cldf.MCMSTimelockProposalInput{}
	if in.MCMS != nil {
		input = *in.MCMS
	}
	timelockRef, err := reader.GetTimelockRef(env, in.ChainSelector, input)
	if err != nil {
		return fmt.Errorf("timelock not present on chain %d: %w", in.ChainSelector, err)
	}
	if _, err = parseEVMAddress(timelockRef.Address, "timelock"); err != nil {
		return fmt.Errorf("invalid timelock ref on chain %d: %w", in.ChainSelector, err)
	}

	if in.MCMS == nil {
		return nil
	}
	if _, err = reader.GetMCMSRef(env, in.ChainSelector, *in.MCMS); err != nil {
		return fmt.Errorf("mcms not present on chain %d: %w", in.ChainSelector, err)
	}

	evmReader, ok := reader.(evmreaders.CallProxyReader)
	if !ok {
		return fmt.Errorf("validate call proxy ref for chain %d: reader for family %q does not support call proxy lookup", in.ChainSelector, chainselectors.FamilyEVM)
	}
	if _, err = evmReader.GetCallProxyRef(env, in.ChainSelector, in.MCMS.Qualifier); err != nil {
		return fmt.Errorf("validate call proxy ref for chain %d: %w", in.ChainSelector, err)
	}

	return nil
}

func validateRoles(in grantrole.SeqInput) error {
	for i, grant := range in.Grants {
		if !grant.Role.Valid() {
			return fmt.Errorf("grants[%d]: unsupported timelock role %s", i, grant.Role.String())
		}
	}

	return nil
}

func parseEVMAddress(raw string, name string) (common.Address, error) {
	if !common.IsHexAddress(raw) {
		return common.Address{}, fmt.Errorf("%s address %q is not a valid EVM address", name, raw)
	}
	addr := common.HexToAddress(raw)
	if addr == (common.Address{}) {
		return common.Address{}, errors.New(name + " address must not be zero")
	}

	return addr, nil
}
