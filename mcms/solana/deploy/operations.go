package soldeploy

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	binary "github.com/gagliardetto/binary"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/Masterminds/semver/v3"
	accessControllerBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/access_controller"
	mcmBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/mcm"
	timelockBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/timelock"
	solanaUtils "github.com/smartcontractkit/chainlink-ccip/chains/solana/utils/common"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	mcmssolanasdk "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	solutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
	familysolana "github.com/smartcontractkit/cld-changesets/pkg/family/solana"
)

const programReadinessTimeout = 30 * time.Second

// ─── Input / output types ─────────────────────────────────────────────────────

type deployProgramInput struct {
	ProgramName  string            `json:"programName"`
	ContractType cldf.ContractType `json:"contractType"`
	Qualifier    string            `json:"qualifier"`
	Label        string            `json:"label"`
}

type initACAccountInput struct {
	ProgramID    solanago.PublicKey `json:"programID"`
	ContractType cldf.ContractType  `json:"contractType"`
	Qualifier    string             `json:"qualifier"`
	Label        string             `json:"label"`
}

type mcmInstanceOutput struct {
	Ref  cldfdatastore.AddressRef `json:"ref"`
	Seed legacysolana.PDASeed     `json:"seed"`
}

type initMCMInstanceInput struct {
	McmProgram   solanago.PublicKey `json:"mcmProgram"`
	ContractType cldf.ContractType  `json:"contractType"`
	MCMConfig    mcmstypes.Config   `json:"mcmConfig"`
	Qualifier    string             `json:"qualifier"`
	Label        string             `json:"label"`
}

type timelockInstanceOutput struct {
	Ref  cldfdatastore.AddressRef `json:"ref"`
	Seed legacysolana.PDASeed     `json:"seed"`
}

type initTimelockInstanceInput struct {
	TimelockProgram         solanago.PublicKey `json:"timelockProgram"`
	AccessControllerProgram solanago.PublicKey `json:"accessControllerProgram"`
	ProposerAC              solanago.PublicKey `json:"proposerAC"`
	ExecutorAC              solanago.PublicKey `json:"executorAC"`
	CancellerAC             solanago.PublicKey `json:"cancellerAC"`
	BypasserAC              solanago.PublicKey `json:"bypasserAC"`
	MinDelay                *big.Int           `json:"minDelay"`
	Qualifier               string             `json:"qualifier"`
	Label                   string             `json:"label"`
}

type setupTimelockRolesInput struct {
	McmProgram              solanago.PublicKey   `json:"mcmProgram"`
	ProposerMCMSeed         legacysolana.PDASeed `json:"proposerMCMSeed"`
	CancellerMCMSeed        legacysolana.PDASeed `json:"cancellerMCMSeed"`
	BypasserMCMSeed         legacysolana.PDASeed `json:"bypasserMCMSeed"`
	TimelockProgram         solanago.PublicKey   `json:"timelockProgram"`
	TimelockSeed            legacysolana.PDASeed `json:"timelockSeed"`
	AccessControllerProgram solanago.PublicKey   `json:"accessControllerProgram"`
	ProposerAC              solanago.PublicKey   `json:"proposerAC"`
	ExecutorAC              solanago.PublicKey   `json:"executorAC"`
	CancellerAC             solanago.PublicKey   `json:"cancellerAC"`
	BypasserAC              solanago.PublicKey   `json:"bypasserAC"`
}

// ─── Operations ───────────────────────────────────────────────────────────────

// opDeployProgram deploys a Solana program binary and returns its address ref.
var opDeployProgram = operations.NewOperation(
	"sol-deploy-program",
	semver.MustParse("1.0.0"),
	"Deploy a Solana program binary and return its program ID as an address ref",
	func(b operations.Bundle, chain cldfsol.Chain, in deployProgramInput) (cldfdatastore.AddressRef, error) {
		size := solutils.GetProgramBufferBytes(in.ProgramName)

		programIDStr, err := chain.DeployProgram(b.Logger, cldfsol.ProgramInfo{
			Name:  in.ProgramName,
			Bytes: size,
		}, false, true)
		if err != nil {
			return cldfdatastore.AddressRef{}, fmt.Errorf("deploy program %q: %w", in.ProgramName, err)
		}

		programID, err := solanago.PublicKeyFromBase58(programIDStr)
		if err != nil {
			return cldfdatastore.AddressRef{}, fmt.Errorf("parse program ID %q: %w", programIDStr, err)
		}

		if err = waitForProgramReady(b.GetContext(), chain.Client, programID); err != nil {
			return cldfdatastore.AddressRef{}, fmt.Errorf("program %q not ready: %w", in.ProgramName, err)
		}

		return addressRef(chain.Selector, in.ContractType, programIDStr, in.Qualifier, in.Label), nil
	},
)

// opInitAccessControllerAccount creates and initializes one access controller account.
var opInitAccessControllerAccount = operations.NewOperation(
	"sol-init-ac-account",
	semver.MustParse("1.0.0"),
	"Create and initialize a Solana access controller account",
	func(b operations.Bundle, chain cldfsol.Chain, in initACAccountInput) (cldfdatastore.AddressRef, error) {
		accessControllerBindings.SetProgramID(in.ProgramID)

		rentExemption, err := chain.Client.GetMinimumBalanceForRentExemption(
			b.GetContext(), accessControllerAccountSize, rpc.CommitmentConfirmed,
		)
		if err != nil {
			return cldfdatastore.AddressRef{}, fmt.Errorf("get rent exemption: %w", err)
		}

		account, err := solanago.NewRandomPrivateKey()
		if err != nil {
			return cldfdatastore.AddressRef{}, fmt.Errorf("generate keypair: %w", err)
		}

		instructions, err := buildAccessControllerInitInstructions(
			in.ProgramID,
			chain.DeployerKey.PublicKey(),
			account,
			rentExemption,
		)
		if err != nil {
			return cldfdatastore.AddressRef{}, err
		}

		if err = chain.Confirm(instructions, solanaUtils.AddSigners(account)); err != nil {
			return cldfdatastore.AddressRef{}, fmt.Errorf("confirm access controller init: %w", err)
		}

		return addressRef(chain.Selector, in.ContractType, account.PublicKey().String(), in.Qualifier, in.Label), nil
	},
)

// opInitMCMInstance initializes one MCM instance on an already-deployed MCM program,
// sets its signer config, and returns the encoded address ref and seed.
var opInitMCMInstance = operations.NewOperation(
	"sol-init-mcm-instance",
	semver.MustParse("1.0.0"),
	"Initialize a Solana MCM instance, set its config, and return the encoded address ref",
	func(b operations.Bundle, chain cldfsol.Chain, in initMCMInstanceInput) (mcmInstanceOutput, error) {
		mcmBindings.SetProgramID(in.McmProgram)

		seed, err := randomSeed()
		if err != nil {
			return mcmInstanceOutput{}, fmt.Errorf("generate seed: %w", err)
		}

		var programData struct {
			DataType uint32
			Address  solanago.PublicKey
		}

		data, err := chain.Client.GetAccountInfoWithOpts(b.GetContext(), in.McmProgram, &rpc.GetAccountInfoOpts{Commitment: rpc.CommitmentConfirmed})
		if err != nil {
			return mcmInstanceOutput{}, fmt.Errorf("get mcm program account info: %w", err)
		}
		if err = binary.UnmarshalBorsh(&programData, data.Bytes()); err != nil {
			return mcmInstanceOutput{}, fmt.Errorf("unmarshal mcm program data: %w", err)
		}

		initIx, err := mcmBindings.NewInitializeInstruction(
			chain.Selector,
			seed,
			familysolana.GetMCMConfigPDA(in.McmProgram, seed),
			chain.DeployerKey.PublicKey(),
			solanago.SystemProgramID,
			in.McmProgram,
			programData.Address,
			familysolana.GetMCMRootMetadataPDA(in.McmProgram, seed),
			familysolana.GetMCMExpiringRootAndOpCountPDA(in.McmProgram, seed),
		).ValidateAndBuild()
		if err != nil {
			return mcmInstanceOutput{}, fmt.Errorf("build mcm Initialize: %w", err)
		}
		if err = chain.Confirm([]solanago.Instruction{initIx}); err != nil {
			return mcmInstanceOutput{}, fmt.Errorf("confirm mcm Initialize: %w", err)
		}

		encodedAddr := legacysolana.EncodeAddressWithSeed(in.McmProgram, seed)

		configurer := mcmssolanasdk.NewConfigurer(chain.Client, *chain.DeployerKey, mcmstypes.ChainSelector(chain.Selector))
		if _, err = configurer.SetConfig(b.GetContext(), encodedAddr, &in.MCMConfig, false); err != nil {
			return mcmInstanceOutput{}, fmt.Errorf("set config: %w", err)
		}

		return mcmInstanceOutput{
			Ref:  addressRef(chain.Selector, in.ContractType, encodedAddr, in.Qualifier, in.Label),
			Seed: seed,
		}, nil
	},
)

// opInitTimelockInstance initializes one timelock instance and returns its encoded
// address ref and seed.
var opInitTimelockInstance = operations.NewOperation(
	"sol-init-timelock-instance",
	semver.MustParse("1.0.0"),
	"Initialize a Solana timelock instance and return the encoded address ref",
	func(b operations.Bundle, chain cldfsol.Chain, in initTimelockInstanceInput) (timelockInstanceOutput, error) {
		timelockBindings.SetProgramID(in.TimelockProgram)

		seed, err := randomSeed()
		if err != nil {
			return timelockInstanceOutput{}, fmt.Errorf("generate seed: %w", err)
		}

		var programData struct {
			DataType uint32
			Address  solanago.PublicKey
		}

		data, err := chain.Client.GetAccountInfoWithOpts(b.GetContext(), in.TimelockProgram, &rpc.GetAccountInfoOpts{Commitment: rpc.CommitmentConfirmed})
		if err != nil {
			return timelockInstanceOutput{}, fmt.Errorf("get timelock program account info: %w", err)
		}
		if err = binary.UnmarshalBorsh(&programData, data.Bytes()); err != nil {
			return timelockInstanceOutput{}, fmt.Errorf("unmarshal timelock program data: %w", err)
		}

		minDelay, err := timelockMinDelayUint64(in.MinDelay)
		if err != nil {
			return timelockInstanceOutput{}, err
		}

		initIx, err := timelockBindings.NewInitializeInstruction(
			seed,
			minDelay,
			familysolana.GetTimelockConfigPDA(in.TimelockProgram, seed),
			chain.DeployerKey.PublicKey(),
			solanago.SystemProgramID,
			in.TimelockProgram,
			programData.Address,
			in.AccessControllerProgram,
			in.ProposerAC,
			in.ExecutorAC,
			in.CancellerAC,
			in.BypasserAC,
		).ValidateAndBuild()
		if err != nil {
			return timelockInstanceOutput{}, fmt.Errorf("build timelock Initialize: %w", err)
		}
		if err = chain.Confirm([]solanago.Instruction{initIx}); err != nil {
			return timelockInstanceOutput{}, fmt.Errorf("confirm timelock Initialize: %w", err)
		}

		encodedAddr := legacysolana.EncodeAddressWithSeed(in.TimelockProgram, seed)

		return timelockInstanceOutput{
			Ref:  addressRef(chain.Selector, mcmscontracts.RBACTimelock, encodedAddr, in.Qualifier, in.Label),
			Seed: seed,
		}, nil
	},
)

// opSetupTimelockRoles grants proposer/executor/canceller/bypasser roles on the timelock.
var opSetupTimelockRoles = operations.NewOperation(
	"sol-setup-timelock-roles",
	semver.MustParse("1.0.0"),
	"Grant MCMS signer PDAs their roles on the Solana timelock",
	func(b operations.Bundle, chain cldfsol.Chain, in setupTimelockRolesInput) (struct{}, error) {
		for _, g := range timelockRoleGrants(in, chain.DeployerKey.PublicKey()) {
			ix, err := buildTimelockBatchAddAccessInstruction(in, g, chain.DeployerKey.PublicKey())
			if err != nil {
				return struct{}{}, fmt.Errorf("build BatchAddAccess for role %v: %w", g.role, err)
			}
			if err = chain.Confirm([]solanago.Instruction{ix}); err != nil {
				return struct{}{}, fmt.Errorf("confirm BatchAddAccess for role %v: %w", g.role, err)
			}
		}

		return struct{}{}, nil
	},
)

type timelockRoleGrant struct {
	role      timelockBindings.Role
	accounts  []solanago.PublicKey
	acAccount solanago.PublicKey
}

func timelockRoleGrants(in setupTimelockRolesInput, deployer solanago.PublicKey) []timelockRoleGrant {
	proposerPDA := familysolana.GetMCMSignerPDA(in.McmProgram, in.ProposerMCMSeed)
	cancellerPDA := familysolana.GetMCMSignerPDA(in.McmProgram, in.CancellerMCMSeed)
	bypasserPDA := familysolana.GetMCMSignerPDA(in.McmProgram, in.BypasserMCMSeed)

	return []timelockRoleGrant{
		{timelockBindings.Proposer_Role, []solanago.PublicKey{proposerPDA}, in.ProposerAC},
		{timelockBindings.Executor_Role, []solanago.PublicKey{deployer}, in.ExecutorAC},
		{timelockBindings.Canceller_Role, []solanago.PublicKey{cancellerPDA, proposerPDA, bypasserPDA}, in.CancellerAC},
		{timelockBindings.Bypasser_Role, []solanago.PublicKey{bypasserPDA}, in.BypasserAC},
	}
}

func buildTimelockBatchAddAccessInstruction(
	in setupTimelockRolesInput,
	g timelockRoleGrant,
	admin solanago.PublicKey,
) (solanago.Instruction, error) {
	timelockBindings.SetProgramID(in.TimelockProgram)

	timelockConfigPDA := familysolana.GetTimelockConfigPDA(in.TimelockProgram, in.TimelockSeed)

	ib := timelockBindings.NewBatchAddAccessInstruction(
		[32]byte(in.TimelockSeed),
		g.role,
		timelockConfigPDA,
		in.AccessControllerProgram,
		g.acAccount,
		admin,
	)
	for _, acc := range g.accounts {
		ib.Append(solanago.Meta(acc))
	}

	return ib.ValidateAndBuild()
}

func buildAccessControllerInitInstructions(
	programID solanago.PublicKey,
	payer solanago.PublicKey,
	account solanago.PrivateKey,
	rentExemption uint64,
) ([]solanago.Instruction, error) {
	accessControllerBindings.SetProgramID(programID)

	createIx, err := system.NewCreateAccountInstruction(
		rentExemption, accessControllerAccountSize,
		programID, payer, account.PublicKey(),
	).ValidateAndBuild()
	if err != nil {
		return nil, fmt.Errorf("build CreateAccount: %w", err)
	}

	initIx, err := accessControllerBindings.NewInitializeInstruction(
		account.PublicKey(), payer,
	).ValidateAndBuild()
	if err != nil {
		return nil, fmt.Errorf("build Initialize: %w", err)
	}

	return []solanago.Instruction{createIx, initIx}, nil
}

// ─── Helpers shared across operations ────────────────────────────────────────

// timelockMinDelayUint64 converts a config min delay to uint64 for on-chain init.
func timelockMinDelayUint64(minDelay *big.Int) (uint64, error) {
	if minDelay == nil {
		return 0, nil
	}
	if minDelay.Sign() < 0 {
		return 0, fmt.Errorf("timelock min delay must be non-negative, got %s", minDelay)
	}
	if minDelay.BitLen() > 64 {
		return 0, fmt.Errorf("timelock min delay overflows uint64, got %s", minDelay)
	}

	return minDelay.Uint64(), nil
}

// accessControllerAccountSize is the on-chain byte size for an AccessController account.
// discriminator(8) + owner(32) + proposed_owner(32) + access_list(64 entries × 32 + length(8))
const accessControllerAccountSize = uint64(8 + 32 + 32 + ((32 * 64) + 8))

func randomSeed() (legacysolana.PDASeed, error) {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	var seed legacysolana.PDASeed
	for i := range seed {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return legacysolana.PDASeed{}, fmt.Errorf("random byte: %w", err)
		}
		seed[i] = alphabet[n.Int64()]
	}

	return seed, nil
}

func semverPtr(v semver.Version) *semver.Version { return &v }

func addressRef(chainSelector uint64, contractType cldf.ContractType, address, qualifier, label string) cldfdatastore.AddressRef {
	ref := cldfdatastore.AddressRef{
		Address:       address,
		ChainSelector: chainSelector,
		Type:          cldfdatastore.ContractType(contractType),
		Version:       semverPtr(semvers.V1_0_0),
		Qualifier:     qualifier,
	}
	if label != "" {
		ref.Labels = cldfdatastore.NewLabelSet(label)
	}

	return ref
}

type getAccountInfoFunc func(ctx context.Context, programID solanago.PublicKey) (*rpc.GetAccountInfoResult, error)

// waitForProgramReady polls until the program account is executable or the
// timeout is reached. This is required because Solana validators can process
// the program deploy transaction before the program account becomes queryable
// by downstream RPC calls, causing init instructions to fail.
func waitForProgramReady(ctx context.Context, client *rpc.Client, programID solanago.PublicKey) error {
	return waitForProgramReadyWith(
		ctx,
		programID,
		client.GetAccountInfo,
		500*time.Millisecond,
		programReadinessTimeout,
	)
}

func waitForProgramReadyWith(
	ctx context.Context,
	programID solanago.PublicKey,
	getAccountInfo getAccountInfoFunc,
	pollInterval time.Duration,
	timeout time.Duration,
) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				if lastErr != nil {
					return fmt.Errorf("timed out waiting for program %s to be executable: %w", programID, lastErr)
				}

				return fmt.Errorf("timed out waiting for program %s to be executable", programID)
			}
			resp, err := getAccountInfo(ctx, programID)
			if err != nil {
				lastErr = err
				continue
			}
			lastErr = nil
			if resp != nil && resp.Value != nil && resp.Value.Executable {
				return nil
			}
		}
	}
}
