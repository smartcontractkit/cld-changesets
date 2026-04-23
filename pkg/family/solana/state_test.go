package solana

import (
	"context"
	"fmt"
	"testing"

	"github.com/gagliardetto/solana-go"
	timelockBindings "github.com/smartcontractkit/chainlink-ccip/chains/solana/gobindings/v0_1_1/timelock"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestMaybeLoadMCMSWithTimelockChainState_NoMatchingRefs(t *testing.T) {
	t.Parallel()

	t.Run("nil refs", func(t *testing.T) {
		t.Parallel()
		got, err := MaybeLoadMCMSWithTimelockChainState(nil)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.NotNil(t, got.MCMSWithTimelockPrograms)
		require.Equal(t, solana.PublicKey{}, got.McmProgram)
	})

	t.Run("empty refs", func(t *testing.T) {
		t.Parallel()
		got, err := MaybeLoadMCMSWithTimelockChainState([]datastore.AddressRef{})
		require.NoError(t, err)
		require.NotNil(t, got)
		require.NotNil(t, got.MCMSWithTimelockPrograms)
		require.Equal(t, solana.PublicKey{}, got.McmProgram)
	})
}

func TestMaybeLoadMCMSWithTimelockState(t *testing.T) {
	t.Parallel()

	const chain1 uint64 = 100_001
	const chain2 uint64 = 100_002
	const mcmProgramAddr = "11111111111111111111111111111111"
	const otherProgramAddr = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

	t.Run("nil and empty chain selector list", func(t *testing.T) {
		t.Parallel()
		env := testSolanaEnv(t, datastore.NewMemoryDataStore().Seal())

		got, err := MaybeLoadMCMSWithTimelockState(env, nil)
		require.NoError(t, err)
		require.Empty(t, got)

		got, err = MaybeLoadMCMSWithTimelockState(env, []uint64{})
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("single chain no refs", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		env := testSolanaEnv(t, ds.Seal())
		got, err := MaybeLoadMCMSWithTimelockState(env, []uint64{chain1})
		require.NoError(t, err)
		require.Len(t, got, 1)
		st := got[chain1]
		require.NotNil(t, st)
		require.NotNil(t, st.MCMSWithTimelockPrograms)
		require.Equal(t, solana.PublicKey{}, st.McmProgram)
	})

	t.Run("two chains isolated by selector", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       mcmProgramAddr,
			ChainSelector: chain1,
			Type:          datastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
		}))
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       otherProgramAddr,
			ChainSelector: chain2,
			Type:          datastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
		}))
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       otherProgramAddr,
			ChainSelector: chain2,
			Type:          datastore.ContractType(mcmscontracts.RBACTimelockProgram),
		}))
		env := testSolanaEnv(t, ds.Seal())
		got, err := MaybeLoadMCMSWithTimelockState(env, []uint64{chain1, chain2})
		require.NoError(t, err)
		require.Len(t, got, 2)

		wantMcm1 := solana.MustPublicKeyFromBase58(mcmProgramAddr)
		wantMcm2 := solana.MustPublicKeyFromBase58(otherProgramAddr)
		require.Equal(t, wantMcm1, got[chain1].McmProgram)
		require.Equal(t, solana.PublicKey{}, got[chain1].TimelockProgram)

		require.Equal(t, wantMcm2, got[chain2].McmProgram)
		require.Equal(t, wantMcm2, got[chain2].TimelockProgram)
	})

	t.Run("invalid address returns wrapped error", func(t *testing.T) {
		t.Parallel()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       "not-valid-base58",
			ChainSelector: chain1,
			Type:          datastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
		}))
		env := testSolanaEnv(t, ds.Seal())
		_, err := MaybeLoadMCMSWithTimelockState(env, []uint64{chain1})
		require.ErrorContains(t, err, fmt.Sprintf(
			"unable to load mcms and timelock solana chain state for chain selector %d: unable to parse public key from mcm address",
			chain1))
	})
}

func TestMCMSWithTimelockPrograms_GetStateFromType(t *testing.T) {
	t.Parallel()

	mcm := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	tlProg := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	acProg := solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")

	var seedProposer, seedBypass, seedCanc, seedTL PDASeed
	seedProposer[0] = 1
	seedBypass[1] = 2
	seedCanc[2] = 3
	for i := range seedTL {
		seedTL[i] = byte(i + 1)
	}

	propAcc := solana.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111")
	execAcc := solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111")
	canAcc := solana.MustPublicKeyFromBase58("SysvarC1ock11111111111111111111111111111111")
	bypassAcc := solana.MustPublicKeyFromBase58("Stake11111111111111111111111111111111111111")

	s := &MCMSWithTimelockPrograms{
		McmProgram:                       mcm,
		ProposerMcmSeed:                  seedProposer,
		BypasserMcmSeed:                  seedBypass,
		CancellerMcmSeed:                 seedCanc,
		TimelockProgram:                  tlProg,
		TimelockSeed:                     seedTL,
		AccessControllerProgram:          acProg,
		ProposerAccessControllerAccount:  propAcc,
		ExecutorAccessControllerAccount:  execAcc,
		CancellerAccessControllerAccount: canAcc,
		BypasserAccessControllerAccount:  bypassAcc,
	}

	tests := []struct {
		name        string
		programType cldf.ContractType
		wantProgram solana.PublicKey
		wantSeed    PDASeed
	}{
		{name: "ManyChainMultisigProgram", programType: mcmscontracts.ManyChainMultisigProgram, wantProgram: mcm, wantSeed: PDASeed{}},
		{name: "ProposerManyChainMultisig", programType: mcmscontracts.ProposerManyChainMultisig, wantProgram: mcm, wantSeed: seedProposer},
		{name: "BypasserManyChainMultisig", programType: mcmscontracts.BypasserManyChainMultisig, wantProgram: mcm, wantSeed: seedBypass},
		{name: "CancellerManyChainMultisig", programType: mcmscontracts.CancellerManyChainMultisig, wantProgram: mcm, wantSeed: seedCanc},
		{name: "RBACTimelockProgram", programType: mcmscontracts.RBACTimelockProgram, wantProgram: tlProg, wantSeed: PDASeed{}},
		{name: "RBACTimelock", programType: mcmscontracts.RBACTimelock, wantProgram: tlProg, wantSeed: seedTL},
		{name: "AccessControllerProgram", programType: mcmscontracts.AccessControllerProgram, wantProgram: acProg, wantSeed: PDASeed{}},
		{name: "ProposerAccessControllerAccount", programType: mcmscontracts.ProposerAccessControllerAccount, wantProgram: acProg, wantSeed: PDASeed(propAcc)},
		{name: "ExecutorAccessControllerAccount", programType: mcmscontracts.ExecutorAccessControllerAccount, wantProgram: acProg, wantSeed: PDASeed(execAcc)},
		{name: "CancellerAccessControllerAccount", programType: mcmscontracts.CancellerAccessControllerAccount, wantProgram: acProg, wantSeed: PDASeed(canAcc)},
		{name: "BypasserAccessControllerAccount", programType: mcmscontracts.BypasserAccessControllerAccount, wantProgram: acProg, wantSeed: PDASeed(bypassAcc)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotProgram, gotSeed, err := s.GetStateFromType(tt.programType)
			require.NoError(t, err)
			require.Equal(t, tt.wantProgram, gotProgram)
			require.Equal(t, tt.wantSeed, gotSeed)
		})
	}

	t.Run("unknown program type", func(t *testing.T) {
		t.Parallel()
		_, _, err := s.GetStateFromType(cldf.ContractType("not-a-real-solana-program"))
		require.ErrorContains(t, err, "unknown program type")
	})
}

func TestMCMSWithTimelockPrograms_SetState(t *testing.T) {
	t.Parallel()

	mcm := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	tlProg := solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
	acProg := solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	roleAcc := solana.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111")

	var seed PDASeed
	seed[7] = 42

	t.Run("unknown contract type", func(t *testing.T) {
		t.Parallel()
		s := &MCMSWithTimelockPrograms{}
		err := s.SetState(cldf.ContractType("unknown-contract"), mcm, seed)
		require.ErrorContains(t, err, "unknown contract type")
	})

	t.Run("SetState then GetStateFromType", func(t *testing.T) {
		t.Parallel()

		roundTrips := []struct {
			name         string
			pre          func(*MCMSWithTimelockPrograms)
			contractType cldf.ContractType
			setProgram   solana.PublicKey
			setSeed      PDASeed
			wantProgram  solana.PublicKey
			wantSeed     PDASeed
			post         func(t *testing.T, s *MCMSWithTimelockPrograms)
		}{
			{
				name:         "ManyChainMultisigProgram",
				contractType: mcmscontracts.ManyChainMultisigProgram,
				setProgram:   mcm,
				wantProgram:  mcm,
			},
			{
				name:         "ProposerManyChainMultisig",
				contractType: mcmscontracts.ProposerManyChainMultisig,
				setProgram:   mcm,
				setSeed:      seed,
				wantProgram:  mcm,
				wantSeed:     seed,
			},
			{
				name:         "RBACTimelockProgram",
				contractType: mcmscontracts.RBACTimelockProgram,
				setProgram:   tlProg,
				wantProgram:  tlProg,
			},
			{
				name:         "RBACTimelock",
				contractType: mcmscontracts.RBACTimelock,
				setProgram:   tlProg,
				setSeed:      seed,
				wantProgram:  tlProg,
				wantSeed:     seed,
			},
			{
				name:         "AccessControllerProgram",
				contractType: mcmscontracts.AccessControllerProgram,
				setProgram:   acProg,
				wantProgram:  acProg,
			},
			{
				name: "ProposerAccessControllerAccount",
				pre: func(s *MCMSWithTimelockPrograms) {
					s.AccessControllerProgram = acProg
				},
				contractType: mcmscontracts.ProposerAccessControllerAccount,
				setProgram:   roleAcc,
				wantProgram:  acProg,
				wantSeed:     PDASeed(roleAcc),
				post: func(t *testing.T, s *MCMSWithTimelockPrograms) {
					t.Helper()
					require.Equal(t, roleAcc, s.ProposerAccessControllerAccount)
				},
			},
		}

		for _, tt := range roundTrips {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				s := &MCMSWithTimelockPrograms{}
				if tt.pre != nil {
					tt.pre(s)
				}
				require.NoError(t, s.SetState(tt.contractType, tt.setProgram, tt.setSeed))
				gotProgram, gotSeed, err := s.GetStateFromType(tt.contractType)
				require.NoError(t, err)
				require.Equal(t, tt.wantProgram, gotProgram)
				require.Equal(t, tt.wantSeed, gotSeed)
				if tt.post != nil {
					tt.post(t, s)
				}
			})
		}
	})
}

func TestMCMSWithTimelockPrograms_RoleAccount(t *testing.T) {
	t.Parallel()

	propAcc := solana.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111")
	execAcc := solana.MustPublicKeyFromBase58("SysvarRent111111111111111111111111111111111")
	canAcc := solana.MustPublicKeyFromBase58("SysvarC1ock11111111111111111111111111111111")
	bypassAcc := solana.MustPublicKeyFromBase58("Stake11111111111111111111111111111111111111")

	s := &MCMSWithTimelockPrograms{
		ProposerAccessControllerAccount:  propAcc,
		ExecutorAccessControllerAccount:  execAcc,
		CancellerAccessControllerAccount: canAcc,
		BypasserAccessControllerAccount:  bypassAcc,
	}

	require.Equal(t, propAcc, s.RoleAccount(timelockBindings.Proposer_Role))
	require.Equal(t, execAcc, s.RoleAccount(timelockBindings.Executor_Role))
	require.Equal(t, canAcc, s.RoleAccount(timelockBindings.Canceller_Role))
	require.Equal(t, bypassAcc, s.RoleAccount(timelockBindings.Bypasser_Role))
	require.Equal(t, solana.PublicKey{}, s.RoleAccount(timelockBindings.Admin_Role))
	require.Equal(t, solana.PublicKey{}, s.RoleAccount(timelockBindings.Role(99)))
}

func testSolanaEnv(t *testing.T, ds datastore.DataStore) cldf.Environment {
	t.Helper()
	return *cldf.NewEnvironment(
		"test",
		logger.Nop(),
		cldf.NewMemoryAddressBook(),
		ds,
		nil,
		nil,
		func() context.Context { return t.Context() },
		ocr.OCRSecrets{},
		chain.NewBlockChains(nil),
	)
}
