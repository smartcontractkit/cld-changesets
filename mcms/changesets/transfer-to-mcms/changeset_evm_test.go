package transfertomcms_test

import (
	"crypto/ecdsa"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfevm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	transfertomcms "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-mcms"
	linkchangesets "github.com/smartcontractkit/cld-changesets/tokens/link/changesets"

	_ "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-mcms/all"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy"
)

func TestTransferToMCMS_DataStore(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
	))
	require.NoError(t, err)

	chain := rt.Environment().BlockChains.EVMChains()[selector]

	err = rt.Exec(
		runtime.ChangesetTask(linkchangesets.DeployLinkTokenChangeset{}, linkchangesets.DeployLinkTokenInput{
			EVM: map[uint64]linkchangesets.EVMLinkConfig{selector: {}},
		}),
		runtime.ChangesetTask(deploy.Changeset{}, deploy.Input{
			ConfigByChain: map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
				selector: cldftesthelpers.SingleGroupTimelockConfig(t),
			},
		}),
	)
	require.NoError(t, err)

	reader, ok := cldf.GetMCMSReaderRegistry().Get(chainselectors.FamilyEVM)
	require.True(t, ok)
	timelockRef, err := reader.GetTimelockRef(rt.Environment(), selector, cldf.MCMSTimelockProposalInput{})
	require.NoError(t, err)
	timelockAddr := common.HexToAddress(timelockRef.Address)

	linkToken, err := loadLinkTokenFromDataStore(chain, rt.State().DataStore)
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(transfertomcms.Changeset{}, transfertomcms.Input{
			Cfg: transfertomcms.Config{
				ContractsByChain: map[uint64][]common.Address{
					selector: {linkToken.Address()},
				},
			},
			MCMS: &cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionBypass,
				ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
				Description:    "Transfer ownership to timelock",
				TimelockDelay:  mcmstypes.NewDuration(0),
			},
		}),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)
	require.Len(t, rt.State().Proposals, 1)
	require.True(t, rt.State().Proposals[0].IsExecuted)

	owner, err := linkToken.Owner(nil)
	require.NoError(t, err)
	require.Equal(t, timelockAddr, owner)
}

func loadLinkTokenFromDataStore(chain cldfevm.Chain, ds datastore.DataStore) (*link_token.LinkToken, error) {
	linkTokenTV := cldf.NewTypeAndVersion(linkcontracts.LinkToken, semvers.V1_0_0)

	refs, err := ds.Addresses().Fetch()
	if err != nil {
		return nil, err
	}

	for _, ref := range refs {
		if ref.ChainSelector != chain.Selector {
			continue
		}

		if ref.Type == datastore.ContractType(linkTokenTV.Type.String()) && ref.Version != nil && ref.Version.String() == linkTokenTV.Version.String() {
			return link_token.NewLinkToken(common.HexToAddress(ref.Address), chain.Client)
		}
	}

	return nil, fmt.Errorf("link token not found on chain %s", chain.Name())
}
