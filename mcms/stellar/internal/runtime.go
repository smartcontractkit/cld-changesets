package stellarinternal

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	stellarprovider "github.com/smartcontractkit/chainlink-deployments-framework/chain/stellar/provider"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
)

// containerOnce ensures CTF framework initialization (docker network setup)
// happens only once per test binary, matching the shared-once convention every
// other chain's container loader follows in cldf's engine/test/onchain. A
// fresh Once per runtime re-runs the setup for every container and is a flake
// source when packages run in parallel.
var containerOnce = &sync.Once{}

func NewStellarRuntime(t *testing.T, selector uint64) *runtime.Runtime {
	t.Helper()

	provider := stellarprovider.NewCTFChainProvider(
		t,
		selector,
		stellarprovider.CTFChainProviderConfig{
			DeployerKeypairGen: stellarprovider.KeypairRandom(),
			Once:               containerOnce,
		},
	)

	blockchain, err := provider.Initialize(t.Context())
	require.NoError(t, err)

	rt, err := runtime.New(
		t.Context(),
		runtime.WithEnvOpts(
			environment.WithChains(blockchain),
			environment.WithLogger(logger.Test(t)),
		),
	)
	require.NoError(t, err)

	chain, ok := rt.Environment().BlockChains.StellarChains()[selector]
	require.True(t, ok)

	FundStellarSigner(t, chain)

	return rt
}

func DeployMCMSWithTimelock(
	t *testing.T,
	rt *runtime.Runtime,
	selector uint64,
	cfg cldfproposalutils.MCMSWithTimelockConfig,
) {
	t.Helper()

	err := rt.Exec(
		runtime.ChangesetTask(
			deploy.Changeset{},
			deploy.Input{
				ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
					selector: cfg,
				},
			},
		),
	)
	require.NoError(t, err)
}

func ContractRef(
	selector uint64,
	contractType cldf.ContractType,
	qualifier string,
) refkey.RefKey {
	return refkey.New(
		selector,
		cldfdatastore.ContractType(contractType),
		&semvers.V1_0_0,
		qualifier,
	)
}

func ResolveContract(
	t *testing.T,
	env cldf.Environment,
	selector uint64,
	contractType cldf.ContractType,
	qualifier string,
) string {
	t.Helper()

	ref, err := ContractRef(selector, contractType, qualifier).Resolve(env)
	require.NoError(t, err)

	return ref.Address
}
