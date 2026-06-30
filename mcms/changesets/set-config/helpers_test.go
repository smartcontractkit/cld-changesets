package setconfig_test

import (
	"context"
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	cldfsol "github.com/smartcontractkit/chainlink-deployments-framework/chain/solana"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/internal/testutil/solanatest"

	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config/all"
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
	evmreaders "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/transfer-to-timelock"
	_ "github.com/smartcontractkit/cld-changesets/mcms/solana/deploy"
)

func contractRef(chainSelector uint64, contractType cldf.ContractType, qualifier string) refkey.RefKey {
	return refkey.New(chainSelector, cldfdatastore.ContractType(contractType), &semvers.V1_0_0, qualifier)
}

func mcmsTargets(
	chainSelector uint64,
	proposer, canceller, bypasser mcmstypes.Config,
) []setconfig.ContractSetConfig {
	return []setconfig.ContractSetConfig{
		{Ref: contractRef(chainSelector, mcmscontracts.ProposerManyChainMultisig, ""), Config: proposer},
		{Ref: contractRef(chainSelector, mcmscontracts.CancellerManyChainMultisig, ""), Config: canceller},
		{Ref: contractRef(chainSelector, mcmscontracts.BypasserManyChainMultisig, ""), Config: bypasser},
	}
}

func newMCMSInput(action mcmstypes.TimelockAction, description, qualifier string) *cldf.MCMSTimelockProposalInput {
	delay := mcmstypes.NewDuration(time.Second)
	if action == mcmstypes.TimelockActionBypass {
		delay = mcmstypes.NewDuration(0)
	}

	return &cldf.MCMSTimelockProposalInput{
		TimelockAction: action,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  delay,
		Qualifier:      qualifier,
		Description:    description,
	}
}

func setConfigInput(targets []setconfig.ContractSetConfig, mcms *cldf.MCMSTimelockProposalInput) setconfig.Input {
	return setconfig.Input{
		Cfg:  setconfig.Config{Targets: targets},
		MCMS: mcms,
	}
}

func newEVMRuntime(t *testing.T, selectors ...uint64) *runtime.Runtime {
	t.Helper()

	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, selectors),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)

	return rt
}

func deployEVMMCMSWithTimelock(t *testing.T, rt *runtime.Runtime, selectors ...uint64) {
	t.Helper()

	cfg := cldftesthelpers.SingleGroupTimelockConfig(t)
	configByChain := make(map[uint64]cldfproposalutils.MCMSWithTimelockConfig, len(selectors))
	for _, selector := range selectors {
		configByChain[selector] = cfg
	}

	err := rt.Exec(runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
		ConfigByChain: configByChain,
	}))
	require.NoError(t, err)
}

func newEVMRuntimeWithDeploy(t *testing.T, selectors ...uint64) *runtime.Runtime {
	t.Helper()

	rt := newEVMRuntime(t, selectors...)
	deployEVMMCMSWithTimelock(t, rt, selectors...)

	return rt
}

func transferEVMMCMSToTimelock(t *testing.T, rt *runtime.Runtime, selector uint64) {
	t.Helper()

	mcmsRefs, _ := loadEVMMCMSRefs(t, rt, selector)

	err := rt.Exec(
		runtime.ChangesetTask(transfertotimelock.Changeset{}, transfertotimelock.Input{
			Cfg: transfertotimelock.Config{
				ContractsByChain: map[uint64][]common.Address{
					selector: {
						mcmsRefs.Proposer,
						mcmsRefs.Bypasser,
						mcmsRefs.Canceller,
					},
				},
			},
			MCMS: &cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionBypass,
				ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
				TimelockDelay:  mcmstypes.NewDuration(0),
				Description:    "transfer MCM to timelock",
			},
		}),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)
}

func newEVMRuntimeWithDeployAndTransfer(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	rt := newEVMRuntimeWithDeploy(t, selector)
	transferEVMMCMSToTimelock(t, rt, selector)

	return rt
}

type evmMCMSRefs struct {
	Timelock  common.Address
	Proposer  common.Address
	Canceller common.Address
	Bypasser  common.Address
}

func loadEVMMCMSRefs(t *testing.T, rt *runtime.Runtime, selector uint64) (evmMCMSRefs, cldf_evm.Chain) {
	t.Helper()

	env := rt.Environment()
	chain := env.BlockChains.EVMChains()[selector]

	reader := evmreaders.Reader{}
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

	return evmMCMSRefs{
		Timelock:  common.HexToAddress(timelock.Address),
		Proposer:  common.HexToAddress(proposer.Address),
		Canceller: common.HexToAddress(canceller.Address),
		Bypasser:  common.HexToAddress(bypasser.Address),
	}, chain
}

// newSolanaVerifyPreconditionsEnv builds a mock Solana environment for VerifyPreconditions
// only — no CTF container or on-chain deploy.
func newSolanaVerifyPreconditionsEnv(t *testing.T, selector uint64) cldf.Environment {
	t.Helper()

	ds := cldfdatastore.NewMemoryDataStore()
	version := semver.MustParse("1.0.0")
	for _, ref := range []struct {
		contractType cldf.ContractType
		address      string
	}{
		{mcmscontracts.RBACTimelock, "timelock-address"},
		{mcmscontracts.ProposerManyChainMultisig, "proposer-address"},
		{mcmscontracts.CancellerManyChainMultisig, "canceller-address"},
		{mcmscontracts.BypasserManyChainMultisig, "bypasser-address"},
	} {
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			Address:       ref.address,
			ChainSelector: selector,
			Type:          cldfdatastore.ContractType(ref.contractType),
			Version:       version,
		}))
	}

	return cldf.Environment{
		Logger:    logger.Test(t),
		DataStore: ds.Seal(),
		GetContext: func() context.Context {
			return t.Context()
		},
		BlockChains: cldf_chain.NewBlockChains(map[uint64]cldf_chain.BlockChain{
			selector: cldfsol.Chain{Selector: selector},
		}),
	}
}

func newSolanaRuntimeWithDeploy(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	programsPath, programIDs := soltestutils.LoadMCMSPrograms(t)
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithSolanaContainer(t, []uint64{selector}, programsPath, programIDs),
		environment.WithDatastore(solanatest.PreloadDatastoreWithMCMSPrograms(t, selector)),
		environment.WithLogger(logger.Test(t)),
	))
	require.NoError(t, err)
	require.Contains(t, rt.Environment().BlockChains.SolanaChains(), selector)

	err = rt.Exec(
		runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
			ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
				selector: cldftesthelpers.SingleGroupTimelockConfig(t),
			},
		}),
	)
	require.NoError(t, err)

	return rt
}
