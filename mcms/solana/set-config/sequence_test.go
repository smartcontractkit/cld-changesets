package solsetconfig

import (
	"context"
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	solanago "github.com/gagliardetto/solana-go"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations/optest"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"

	// TODO: remove legacymcms import once remaining MCMS changesets are migrated out of legacy/mcms/changesets.
	legacymcms "github.com/smartcontractkit/cld-changesets/legacy/mcms/changesets"
	solchangesets "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/changesets"
	"github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
	solreaders "github.com/smartcontractkit/cld-changesets/mcms/solana/readers"
)

//nolint:paralleltest // global mcm.SetProgramID state; serialized via soltestutils.PreloadMCMS lock
func testSeqSolanaSetConfig(t *testing.T) {
	tests := []struct {
		name   string
		noSend bool
	}{
		{name: "direct send", noSend: false},
		{name: "MCMS proposal", noSend: true},
	}

	for _, tt := range tests { //nolint:paralleltest // global mcm.SetProgramID state
		t.Run(tt.name, func(t *testing.T) {
			selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
			rt := newSolanaSetConfigRuntime(t, selector)
			chain := rt.Environment().BlockChains.SolanaChains()[selector]
			refs := solanaSetConfigRefs(t, rt.Environment(), selector)
			fundSolanaSignerPDAs(t, chain, refs)

			authorityAccount := solanago.PublicKey{}
			if tt.noSend {
				transferSolanaMCMSToTimelock(t, rt, selector)
				fundSolanaSignerPDAs(t, chain, refs)
				authorityAccount = refs.TimelockSigner
			}

			proposerCfg := cldftesthelpers.SingleGroupMCMS(t)
			proposerCfg.Signers = append(proposerCfg.Signers, common.HexToAddress("0x0000000000000000000000000000000000000101"))
			proposerCfg.Quorum = 2

			cancellerCfg := cldftesthelpers.SingleGroupMCMS(t)
			cancellerCfg.Signers = append(cancellerCfg.Signers, common.HexToAddress("0x0000000000000000000000000000000000000202"))
			cancellerCfg.Quorum = 2

			bypasserCfg := cldftesthelpers.SingleGroupMCMS(t)
			bypasserCfg.Signers = append(bypasserCfg.Signers, common.HexToAddress("0x0000000000000000000000000000000000000303"))
			bypasserCfg.Quorum = 2

			targets := []MCMSetConfigTarget{
				{
					Address:      refs.Proposer,
					Config:       proposerCfg,
					ContractType: string(mcmscontracts.ProposerManyChainMultisig),
				},
				{
					Address:      refs.Canceller,
					Config:       cancellerCfg,
					ContractType: string(mcmscontracts.CancellerManyChainMultisig),
				},
			}
			if tt.noSend {
				targets = []MCMSetConfigTarget{
					{
						Address:      refs.Canceller,
						Config:       cancellerCfg,
						ContractType: string(mcmscontracts.CancellerManyChainMultisig),
					},
				}
			}

			report, err := operations.ExecuteSequence(
				rt.Environment().OperationsBundle,
				SeqSolanaSetConfig,
				chain,
				SeqSolanaSetConfigInput{
					ChainSelector:    selector,
					NoSend:           tt.noSend,
					AuthorityAccount: authorityAccount,
					Targets:          targets,
				},
			)
			require.NoError(t, err)

			if tt.noSend {
				require.Len(t, report.Output.BatchOps, 1)
				require.NotEmpty(t, report.Output.BatchOps[0].Transactions)
				require.NoError(t, rt.Exec(
					newTimelockProposalTask(report.Output.BatchOps, "solana set config sequence test"),
					runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
				))
			} else {
				require.Empty(t, report.Output.BatchOps)
			}

			inspector := mcmssolana.NewInspector(chain.Client)
			if tt.noSend {
				assertSolanaConfigEquals(t, inspector, refs.Canceller, cancellerCfg)
			} else {
				assertSolanaConfigEquals(t, inspector, refs.Proposer, proposerCfg)
				assertSolanaConfigEquals(t, inspector, refs.Canceller, cancellerCfg)
			}
		})
	}
}

func newSolanaSetConfigRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	programsPath, programIDs, ab := soltestutils.PreloadMCMS(t, selector)
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{selector}, programsPath, programIDs),
		environment.WithAddressBook(ab),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)
	require.Contains(t, rt.Environment().BlockChains.SolanaChains(), selector)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(legacymcms.DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	return rt
}

type solanaMCMSRefs struct {
	Timelock        string
	Proposer        string
	Canceller       string
	Bypasser        string
	TimelockSigner  solanago.PublicKey
	ProposerSigner  solanago.PublicKey
	CancellerSigner solanago.PublicKey
	BypasserSigner  solanago.PublicKey
}

func solanaSetConfigRefs(t *testing.T, env cldf.Environment, selector uint64) solanaMCMSRefs {
	t.Helper()

	reader := solreaders.Reader{}
	timelock, err := reader.GetTimelockRef(env, selector, cldf.MCMSTimelockProposalInput{})
	require.NoError(t, err)
	proposer, err := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
	})
	require.NoError(t, err)
	canceller, err := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionCancel,
	})
	require.NoError(t, err)
	bypasser, err := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionBypass,
	})
	require.NoError(t, err)

	timelockProgram, timelockSeed, err := mcmssolana.ParseContractAddress(timelock.Address)
	require.NoError(t, err)
	timelockSigner, err := mcmssolana.FindTimelockSignerPDA(timelockProgram, timelockSeed)
	require.NoError(t, err)

	return solanaMCMSRefs{
		Timelock:        timelock.Address,
		Proposer:        proposer.Address,
		Canceller:       canceller.Address,
		Bypasser:        bypasser.Address,
		TimelockSigner:  timelockSigner,
		ProposerSigner:  solanaMCMSSignerPDA(t, proposer.Address),
		CancellerSigner: solanaMCMSSignerPDA(t, canceller.Address),
		BypasserSigner:  solanaMCMSSignerPDA(t, bypasser.Address),
	}
}

func solanaMCMSSignerPDA(t *testing.T, address string) solanago.PublicKey {
	t.Helper()

	program, seed, err := mcmssolana.ParseContractAddress(address)
	require.NoError(t, err)
	signer, err := mcmssolana.FindSignerPDA(program, seed)
	require.NoError(t, err)

	return signer
}

func fundSolanaSignerPDAs(t *testing.T, chain cldfsol.Chain, refs solanaMCMSRefs) {
	t.Helper()

	err := solutils.FundAccounts(t.Context(), chain.Client, []solanago.PublicKey{
		refs.TimelockSigner,
		refs.ProposerSigner,
		refs.CancellerSigner,
		refs.BypasserSigner,
	}, 1)
	require.NoError(t, err)
}

func transferSolanaMCMSToTimelock(t *testing.T, rt *runtime.Runtime, selector uint64) {
	t.Helper()

	err := rt.Exec(
		runtime.ChangesetTask(solchangesets.TransferMCMSToTimelockSolana{}, solchangesets.TransferMCMSToTimelockSolanaConfig{
			Chains:  []uint64{selector},
			MCMSCfg: cldfproposalutils.TimelockConfig{MinDelay: time.Second},
		}),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)
}

type timelockProposalTask struct {
	id          string
	batchOps    []mcmstypes.BatchOperation
	description string
}

func newTimelockProposalTask(batchOps []mcmstypes.BatchOperation, description string) timelockProposalTask {
	return timelockProposalTask{
		id:          ksuid.New().String(),
		batchOps:    batchOps,
		description: description,
	}
}

func (t timelockProposalTask) ID() string {
	return t.id
}

func (t timelockProposalTask) Run(e cldf.Environment, state *runtime.State) error {
	out, err := cldf.NewOutputBuilder(e, datastore.NewMemoryDataStore()).
		WithTimelockProposal(cldf.MCMSTimelockProposalInput{
			TimelockAction: mcmstypes.TimelockActionSchedule,
			ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
			TimelockDelay:  mcmstypes.NewDuration(time.Second),
			Description:    t.description,
		}, t.batchOps).
		Build()
	if err != nil {
		return err
	}

	return state.MergeChangesetOutput(t.id, out)
}

func assertSolanaConfigEquals(t *testing.T, inspector *mcmssolana.Inspector, address string, want mcmstypes.Config) {
	t.Helper()

	got, err := inspector.GetConfig(t.Context(), address)
	require.NoError(t, err)
	require.ElementsMatch(t, want.Signers, got.Signers)
	require.Equal(t, want.Quorum, got.Quorum)
}

func TestSetConfigTargets(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)
	validRef := refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, "")

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "proposer-account",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       semver.MustParse("1.0.0"),
	}))
	env := cldf.Environment{
		DataStore:  ds.Seal(),
		GetContext: context.Background,
	}

	got, err := setConfigTargets(env, []setconfig.ContractSetConfig{
		{Ref: validRef, Config: cfg},
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "proposer-account", got[0].Address)
	require.Equal(t, cfg, got[0].Config)

	_, err = setConfigTargets(env, []setconfig.ContractSetConfig{
		{Ref: refkey.New(selector, datastore.ContractType(mcmscontracts.CancellerManyChainMultisig), &semvers.V1_0_0, ""), Config: cfg},
	})
	require.ErrorContains(t, err, "targets[0]:")
}

func TestRunSolanaSetConfig_errors(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)

	_, err := runSolanaSetConfig(
		optest.NewBundle(t),
		setconfig.Deps{BlockChains: chain.NewBlockChains(nil)},
		setconfig.ChainInput{ChainSelector: selector},
	)
	require.ErrorContains(t, err, "solana chain")

	deps := setconfig.Deps{
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
		DataStore: datastore.NewMemoryDataStore().Seal(),
	}
	_, err = runSolanaSetConfig(
		optest.NewBundle(t),
		deps,
		setconfig.ChainInput{
			ChainSelector: selector,
			Targets: []setconfig.ContractSetConfig{
				{
					Ref:    refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, ""),
					Config: cfg,
				},
			},
		},
	)
	require.ErrorContains(t, err, "targets[0]:")

	mcmsInput := &cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
	}
	_, err = runSolanaSetConfig(
		optest.NewBundle(t),
		deps,
		setconfig.ChainInput{
			ChainSelector: selector,
			Targets: []setconfig.ContractSetConfig{
				{
					Ref:    refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, ""),
					Config: cfg,
				},
			},
			MCMS: mcmsInput,
		},
	)
	require.ErrorContains(t, err, "resolve timelock ref")
}

func TestRunSolanaSetConfig_invalidTimelockAddress(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)
	version := semver.MustParse("1.0.0")

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "timelock-account",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.RBACTimelock),
		Version:       version,
	}))
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "proposer-account",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       version,
	}))

	deps := setconfig.Deps{
		BlockChains: chain.NewBlockChains(map[uint64]chain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
		DataStore: ds.Seal(),
	}
	_, err := runSolanaSetConfig(
		optest.NewBundle(t),
		deps,
		setconfig.ChainInput{
			ChainSelector: selector,
			Targets: []setconfig.ContractSetConfig{
				{
					Ref:    refkey.New(selector, datastore.ContractType(mcmscontracts.ProposerManyChainMultisig), &semvers.V1_0_0, ""),
					Config: cfg,
				},
			},
			MCMS: &cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionSchedule,
				ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
				TimelockDelay:  mcmstypes.NewDuration(time.Second),
			},
		},
	)
	require.ErrorContains(t, err, "parse timelock ref address")
}
