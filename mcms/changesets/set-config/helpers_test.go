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
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
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

	// TODO: remove legacymcms import once remaining MCMS changesets are migrated out of legacy/mcms/changesets.
	legacymcms "github.com/smartcontractkit/cld-changesets/legacy/mcms/changesets"
	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
	soltestutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/testutils"
	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config/all"
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

	err := rt.Exec(
		// TODO: replace with new deploy changeset when available
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(legacymcms.DeployMCMSWithTimelockV2), configByChain),
	)
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

	mcmsState, _ := evmMCMSChainState(t, rt, selector)

	err := rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(legacymcms.TransferToMCMSWithTimelockV2), legacymcms.TransferToMCMSWithTimelockConfig{
			ContractsByChain: map[uint64][]common.Address{
				selector: {
					mcmsState.ProposerMcm.Address(),
					mcmsState.BypasserMcm.Address(),
					mcmsState.CancellerMcm.Address(),
				},
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

func evmMCMSChainState(t *testing.T, rt *runtime.Runtime, selector uint64) (*evmstate.MCMSWithTimelockState, cldf_evm.Chain) {
	t.Helper()

	chain := rt.Environment().BlockChains.EVMChains()[selector]
	addrs, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)

	mcmsState, err := evmstate.MaybeLoadMCMSWithTimelockChainState(chain, addrs)
	require.NoError(t, err)

	return mcmsState, chain
}

// newSolanaVerifyPreconditionsEnv builds a mock Solana environment for VerifyPreconditions
// only — no CTF container or on-chain deploy.
func newSolanaVerifyPreconditionsEnv(t *testing.T, selector uint64) cldf.Environment {
	t.Helper()

	ds := datastore.NewMemoryDataStore()
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
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       ref.address,
			ChainSelector: selector,
			Type:          datastore.ContractType(ref.contractType),
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
