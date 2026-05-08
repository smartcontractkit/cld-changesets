package legacy

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	cldf_solana "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	mcmsSolanaSdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmsTypes "github.com/smartcontractkit/mcms/types"

	mcmBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/mcm"
	solanaUtils "github.com/smartcontractkit/chainlink-ccip/chains/solana/utils/common"

	cldchangesetscommon "github.com/smartcontractkit/cld-changesets/pkg/common"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
	legacy2 "github.com/smartcontractkit/cld-changesets/pkg/family/solana/legacy"
	"github.com/smartcontractkit/cld-changesets/pkg/family/solana/legacy/solutils"
)

func deployMCMProgram(
	env cldf.Environment, chainState *legacy2.MCMSWithTimelockState,
	chain cldf_solana.Chain, addressBook cldf.AddressBook,
) error {
	typeAndVersion := cldf.NewTypeAndVersion(mcmscontracts.ManyChainMultisigProgram, cldchangesetscommon.Version1_0_0)

	programID, _, err := chainState.GetStateFromType(mcmscontracts.ManyChainMultisigProgram)
	if err != nil {
		return fmt.Errorf("failed to get mcm state: %w", err)
	}

	if programID.IsZero() {
		deployedProgramID, err := chain.DeployProgram(env.Logger, cldf_solana.ProgramInfo{
			Name:  solutils.ProgMCM,
			Bytes: solutils.GetProgramBufferBytes(solutils.ProgMCM),
		}, false, true)
		if err != nil {
			return fmt.Errorf("failed to deploy mcm program: %w", err)
		}

		programID, err = solana.PublicKeyFromBase58(deployedProgramID)
		if err != nil {
			return fmt.Errorf("failed to convert mcm program id to public key: %w", err)
		}

		err = addressBook.Save(chain.Selector, programID.String(), typeAndVersion)
		if err != nil {
			return fmt.Errorf("failed to save mcm address: %w", err)
		}

		err = chainState.SetState(mcmscontracts.ManyChainMultisigProgram, programID, legacy2.PDASeed{})
		if err != nil {
			return fmt.Errorf("failed to save onchain state: %w", err)
		}

		env.Logger.Infow("deployed mcm contract",
			"chain", chain.String(), "contract", typeAndVersion.String(), "programId", deployedProgramID)
	} else {
		env.Logger.Infow("using existing MCM program",
			"chain", chain.String(), "contract", typeAndVersion.String(), "programId", programID.String())
	}

	return nil
}

func initMCM(
	env cldf.Environment, chainState *legacy2.MCMSWithTimelockState, contractType cldf.ContractType,
	chain cldf_solana.Chain, addressBook cldf.AddressBook, mcmConfig *mcmsTypes.Config,
) error {
	if chainState.McmProgram.IsZero() {
		return errors.New("mcm program is not deployed")
	}
	programID := chainState.McmProgram

	typeAndVersion := cldf.NewTypeAndVersion(contractType, cldchangesetscommon.Version1_0_0)
	mcmProgram, mcmSeed, err := chainState.GetStateFromType(contractType)
	if err != nil {
		return fmt.Errorf("failed to get mcm state: %w", err)
	}

	if mcmSeed != (legacy2.PDASeed{}) {
		mcmConfigPDA := familysolana.GetMCMConfigPDA(mcmProgram, mcmSeed)
		var data mcmBindings.MultisigConfig
		err = solanaUtils.GetAccountDataBorshInto(env.GetContext(), chain.Client, mcmConfigPDA, rpc.CommitmentConfirmed, &data)
		if err == nil {
			env.Logger.Infow("mcm config already initialized, skipping initialization", "chain", chain.String())
			return nil
		}

		return fmt.Errorf("unable to read mcm ConfigPDA account config %s", mcmConfigPDA.String())
	}

	env.Logger.Infow("mcm config not initialized, initializing", "chain", chain.String())

	seed, err := randomSeed()
	if err != nil {
		return fmt.Errorf("failed to generate MCM seed: %w", err)
	}
	env.Logger.Infow("generated MCM seed",
		"chain", chain.String(), "contract", typeAndVersion.String(), "seed", string(seed[:]))

	err = initializeMCM(env, chain, programID, seed)
	if err != nil {
		return fmt.Errorf("failed to initialize mcm: %w", err)
	}

	mcmAddress := legacy2.EncodeAddressWithSeed(programID, seed)

	configurer := mcmsSolanaSdk.NewConfigurer(chain.Client, *chain.DeployerKey, mcmsTypes.ChainSelector(chain.Selector))
	tx, err := configurer.SetConfig(env.GetContext(), mcmAddress, mcmConfig, false)
	if err != nil {
		return fmt.Errorf("failed to set config on mcm: %w", err)
	}
	env.Logger.Infow("called SetConfig on MCM",
		"chain", chain.String(), "contract", typeAndVersion.String(), "transaction", tx.Hash)

	err = addressBook.Save(chain.Selector, mcmAddress, typeAndVersion)
	if err != nil {
		return fmt.Errorf("failed to save address: %w", err)
	}

	err = chainState.SetState(contractType, programID, seed)
	if err != nil {
		return fmt.Errorf("failed to save onchain state: %w", err)
	}

	return nil
}

func initializeMCM(e cldf.Environment, chain cldf_solana.Chain, mcmProgram solana.PublicKey, multisigID legacy2.PDASeed) error {
	var mcmConfig mcmBindings.MultisigConfig
	err := chain.GetAccountDataBorshInto(e.GetContext(), familysolana.GetMCMConfigPDA(mcmProgram, multisigID), &mcmConfig)
	if err == nil {
		e.Logger.Infow("MCM already initialized, skipping initialization", "chain", chain.String())
		return nil
	}

	var programData struct {
		DataType uint32
		Address  solana.PublicKey
	}
	opts := &rpc.GetAccountInfoOpts{Commitment: rpc.CommitmentConfirmed}

	data, err := chain.Client.GetAccountInfoWithOpts(e.GetContext(), mcmProgram, opts)
	if err != nil {
		return fmt.Errorf("failed to get mcm program account info: %w", err)
	}
	err = binary.UnmarshalBorsh(&programData, data.Bytes())
	if err != nil {
		return fmt.Errorf("failed to unmarshal program data: %w", err)
	}

	ix, err := mcmBindings.NewInitializeInstruction(
		chain.Selector,
		multisigID,
		familysolana.GetMCMConfigPDA(mcmProgram, multisigID),
		chain.DeployerKey.PublicKey(),
		solana.SystemProgramID,
		mcmProgram,
		programData.Address,
		familysolana.GetMCMRootMetadataPDA(mcmProgram, multisigID),
		familysolana.GetMCMExpiringRootAndOpCountPDA(mcmProgram, multisigID),
	).ValidateAndBuild()
	if err != nil {
		return fmt.Errorf("failed to build instruction: %w", err)
	}
	ixData, err := ix.Data()
	if err != nil {
		return fmt.Errorf("failed to extract data payload from mcm initialize instruction: %w", err)
	}
	initIx := solana.NewInstruction(mcmProgram, ix.Accounts(), ixData)
	err = chain.Confirm([]solana.Instruction{initIx})
	if err != nil {
		return fmt.Errorf("failed to confirm instructions: %w", err)
	}

	return nil
}

func randomSeed() (legacy2.PDASeed, error) {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	seed := legacy2.PDASeed{}
	for i := range seed {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return legacy2.PDASeed{}, fmt.Errorf("failed to generate random seed byte: %w", err)
		}
		seed[i] = alphabet[n.Int64()]
	}

	return seed, nil
}
