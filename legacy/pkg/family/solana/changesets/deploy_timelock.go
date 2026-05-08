package changesets

import (
	"errors"
	"fmt"
	"math/big"

	binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"

	timelockBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/timelock"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	legacy2 "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	"github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

func deployTimelockProgram(
	e cldf.Environment, chainState *legacy2.MCMSWithTimelockState, chain cldf_solana.Chain,
	addressBook cldf.AddressBook,
) error {
	typeAndVersion := cldf.NewTypeAndVersion(mcmscontracts.RBACTimelockProgram, cldchangesetscommon.Version1_0_0)

	programID, _, err := chainState.GetStateFromType(mcmscontracts.RBACTimelock)
	if err != nil {
		return fmt.Errorf("failed to get timelock state: %w", err)
	}

	if programID.IsZero() {
		deployedProgramID, err := chain.DeployProgram(e.Logger, cldf_solana.ProgramInfo{
			Name:  solutils.ProgTimelock,
			Bytes: solutils.GetProgramBufferBytes(solutils.ProgTimelock),
		}, false, true)
		if err != nil {
			return fmt.Errorf("failed to deploy timelock program: %w", err)
		}

		programID, err = solana.PublicKeyFromBase58(deployedProgramID)
		if err != nil {
			return fmt.Errorf("failed to convert timelock program id to public key: %w", err)
		}

		err = addressBook.Save(chain.Selector, programID.String(), typeAndVersion)
		if err != nil {
			return fmt.Errorf("failed to save mcm address: %w", err)
		}

		err = chainState.SetState(mcmscontracts.RBACTimelockProgram, programID, legacy2.PDASeed{})
		if err != nil {
			return fmt.Errorf("failed to save onchain state: %w", err)
		}

		e.Logger.Infow("deployed timelock contract",
			"chain", chain.String(), "contract", typeAndVersion.String(), "programId", programID)
	} else {
		e.Logger.Infow("using existing Timelock program",
			"chain", chain.String(), "contract", typeAndVersion.String(), "programId", programID.String())
	}

	return nil
}

func initTimelock(
	e cldf.Environment, chainState *legacy2.MCMSWithTimelockState, chain cldf_solana.Chain,
	addressBook cldf.AddressBook, minDelay *big.Int,
) error {
	if chainState.TimelockProgram.IsZero() {
		return errors.New("mcm program is not deployed")
	}
	programID := chainState.TimelockProgram
	timelockBindings.SetProgramID(programID)

	typeAndVersion := cldf.NewTypeAndVersion(mcmscontracts.RBACTimelock, cldchangesetscommon.Version1_0_0)
	timelockProgram, timelockSeed, err := chainState.GetStateFromType(mcmscontracts.RBACTimelock)
	if err != nil {
		return fmt.Errorf("failed to get timelock state: %w", err)
	}

	if (timelockSeed != legacy2.PDASeed{}) {
		timelockConfigPDA := familysolana.GetTimelockConfigPDA(timelockProgram, timelockSeed)
		var timelockConfig timelockBindings.Config
		err = chain.GetAccountDataBorshInto(e.GetContext(), timelockConfigPDA, &timelockConfig)
		if err == nil {
			e.Logger.Infow("timelock config already initialized, skipping initialization", "chain", chain.String())
			return nil
		}

		return fmt.Errorf("unable to read timelock ConfigPDA account config %s", timelockConfigPDA.String())
	}

	e.Logger.Infow("timelock config not initialized, initializing", "chain", chain.String())

	seed, err := randomSeed()
	if err != nil {
		return fmt.Errorf("failed to generate timelock seed: %w", err)
	}
	e.Logger.Infow("generated Timelock seed",
		"chain", chain.String(), "contract", typeAndVersion.String(), "seed", string(seed[:]))

	err = initializeTimelock(e, chain, programID, seed, chainState, minDelay)
	if err != nil {
		return fmt.Errorf("failed to initialize timelock: %w", err)
	}

	timelockAddress := legacy2.EncodeAddressWithSeed(programID, seed)

	err = addressBook.Save(chain.Selector, timelockAddress, typeAndVersion)
	if err != nil {
		return fmt.Errorf("failed to save address: %w", err)
	}

	err = chainState.SetState(mcmscontracts.RBACTimelock, programID, seed)
	if err != nil {
		return fmt.Errorf("failed to save onchain state: %w", err)
	}

	return nil
}

func initializeTimelock(
	e cldf.Environment, chain cldf_solana.Chain, timelockProgram solana.PublicKey, timelockID legacy2.PDASeed,
	chainState *legacy2.MCMSWithTimelockState, minDelay *big.Int,
) error {
	if minDelay == nil {
		minDelay = big.NewInt(0)
	}

	var timelockConfig timelockBindings.Config
	err := chain.GetAccountDataBorshInto(e.GetContext(), familysolana.GetTimelockConfigPDA(timelockProgram, timelockID),
		&timelockConfig)
	if err == nil {
		e.Logger.Infow("Timelock already initialized, skipping initialization", "chain", chain.String())
		return nil
	}

	var programData struct {
		DataType uint32
		Address  solana.PublicKey
	}
	opts := &rpc.GetAccountInfoOpts{Commitment: rpc.CommitmentConfirmed}

	data, err := chain.Client.GetAccountInfoWithOpts(e.GetContext(), timelockProgram, opts)
	if err != nil {
		return fmt.Errorf("failed to get timelock program account info: %w", err)
	}
	err = binary.UnmarshalBorsh(&programData, data.Bytes())
	if err != nil {
		return fmt.Errorf("failed to unmarshal program data: %w", err)
	}

	instruction, err := timelockBindings.NewInitializeInstruction(
		timelockID,
		minDelay.Uint64(),
		familysolana.GetTimelockConfigPDA(timelockProgram, timelockID),
		chain.DeployerKey.PublicKey(),
		solana.SystemProgramID,
		timelockProgram,
		programData.Address,
		chainState.AccessControllerProgram,
		chainState.ProposerAccessControllerAccount,
		chainState.ExecutorAccessControllerAccount,
		chainState.CancellerAccessControllerAccount,
		chainState.BypasserAccessControllerAccount,
	).ValidateAndBuild()
	if err != nil {
		return fmt.Errorf("failed to build instruction: %w", err)
	}

	err = chain.Confirm([]solana.Instruction{instruction})
	if err != nil {
		return fmt.Errorf("failed to confirm instructions: %w", err)
	}

	return nil
}
