package grantrole

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfchain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"
)

func TestGroupByFamily(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	gasBoost := &proposalutils.GasBoostConfig{}
	mcmsInput := &cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
	}
	grantee := common.HexToAddress("0x00000000000000000000000000000000000000aa")

	byFamily, err := groupByFamily(Input{
		MCMS: mcmsInput,
		Cfg: Config{
			GrantsByChain: map[uint64][]RoleGrant{
				selector: {{
					Role:      mcmssdk.TimelockRoleExecutor,
					Addresses: []common.Address{grantee},
				}},
			},
			GasBoostConfig: gasBoost,
		},
	})
	require.NoError(t, err)
	require.Len(t, byFamily[chainselectors.FamilyEVM], 1)
	require.Equal(t, selector, byFamily[chainselectors.FamilyEVM][0].ChainSelector)
	require.Equal(t, mcmsInput, byFamily[chainselectors.FamilyEVM][0].MCMS)
	require.Equal(t, gasBoost, byFamily[chainselectors.FamilyEVM][0].GasBoostConfig)
	require.Equal(t, []common.Address{grantee}, byFamily[chainselectors.FamilyEVM][0].Grants[0].Addresses)

	_, err = groupByFamily(Input{
		Cfg: Config{
			GrantsByChain: map[uint64][]RoleGrant{
				0: {{
					Role:      mcmssdk.TimelockRoleExecutor,
					Addresses: []common.Address{grantee},
				}},
			},
		},
	})
	require.EqualError(t, err, "chain selector 0: unknown chain selector 0")
}

func TestEnvFromDeps(t *testing.T) {
	t.Parallel()

	blockChains := cldfchain.NewBlockChains(nil)
	ds := datastore.NewMemoryDataStore().Seal()
	deps := Deps{
		BlockChains: blockChains,
		DataStore:   ds,
	}
	env := EnvFromDeps(deps)
	require.Equal(t, blockChains, env.BlockChains)
	require.Equal(t, ds, env.DataStore)
}
