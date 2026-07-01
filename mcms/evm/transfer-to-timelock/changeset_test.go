package evmtransfertotimelock_test

import (
	"crypto/ecdsa"
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
	"github.com/smartcontractkit/mcms"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/mcms/changesets/deploy"
	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"
	linkchangesets "github.com/smartcontractkit/cld-changesets/tokens/link/changesets"

	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/deploy"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/transfer-to-timelock"
)

func TestChangeset_TransferOwnershipToTimelock(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	rt, _, timelockAddr, linkToken := newTransferToTimelockTestEnv(t, selector)

	err := rt.Exec(
		runtime.ChangesetTask(transfertotimelock.Changeset{}, transfertotimelock.Input{
			Cfg: transfertotimelock.Config{
				ContractsByChain: map[uint64][]refkey.RefKey{
					selector: {linkTokenRef(selector)},
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

func TestChangeset_OnlyAcceptOwnership(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	rt, chain, timelockAddr, linkToken := newTransferToTimelockTestEnv(t, selector)

	tx, err := linkToken.TransferOwnership(chain.DeployerKey, timelockAddr)
	require.NoError(t, err)
	_, err = chain.Confirm(tx)
	require.NoError(t, err)

	owner, err := linkToken.Owner(nil)
	require.NoError(t, err)
	require.Equal(t, chain.DeployerKey.From, owner, "deployer remains owner until acceptOwnership")

	err = rt.Exec(
		runtime.ChangesetTask(transfertotimelock.Changeset{}, transfertotimelock.Input{
			Cfg: transfertotimelock.Config{
				OnlyAcceptOwnership: true,
				ContractsByChain: map[uint64][]refkey.RefKey{
					selector: {linkTokenRef(selector)},
				},
			},
			MCMS: &cldf.MCMSTimelockProposalInput{
				TimelockAction: mcmstypes.TimelockActionBypass,
				ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
				Description:    "Accept ownership on timelock",
				TimelockDelay:  mcmstypes.NewDuration(0),
			},
		}),
	)
	require.NoError(t, err)

	var proposal mcms.TimelockProposal
	var foundProposal bool
	for _, out := range rt.State().Outputs {
		if len(out.MCMSTimelockProposals) > 0 {
			proposal = out.MCMSTimelockProposals[0]
			foundProposal = true
		}
	}
	require.True(t, foundProposal, "expected one MCMS timelock proposal")
	require.Len(t, proposal.Operations, 1)
	require.Len(t, proposal.Operations[0].Transactions, 1)
	require.Equal(t, linkToken.Address().Hex(), proposal.Operations[0].Transactions[0].To)
	require.Equal(t, []byte{0x79, 0xba, 0x50, 0x97}, proposal.Operations[0].Transactions[0].Data)

	err = rt.Exec(runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}))
	require.NoError(t, err)
	require.Len(t, rt.State().Proposals, 1)
	require.True(t, rt.State().Proposals[0].IsExecuted)

	owner, err = linkToken.Owner(nil)
	require.NoError(t, err)
	require.Equal(t, timelockAddr, owner)
}

func newTransferToTimelockTestEnv(
	t *testing.T,
	selector uint64,
) (*runtime.Runtime, cldfevm.Chain, common.Address, *link_token.LinkToken) {
	t.Helper()

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

	linkToken := loadLinkTokenFromDataStore(t, chain, rt.State().DataStore)

	return rt, chain, timelockAddr, linkToken
}

func linkTokenRef(selector uint64) refkey.RefKey {
	tv := cldf.NewTypeAndVersion(linkcontracts.LinkToken, semvers.V1_0_0)

	return refkey.New(selector, datastore.ContractType(tv.Type.String()), &semvers.V1_0_0, "")
}

func loadLinkTokenFromDataStore(t *testing.T, chain cldfevm.Chain, ds datastore.DataStore) *link_token.LinkToken {
	t.Helper()

	linkTokenTV := cldf.NewTypeAndVersion(linkcontracts.LinkToken, semvers.V1_0_0)

	refs, err := ds.Addresses().Fetch()
	require.NoError(t, err)

	for _, ref := range refs {
		if ref.ChainSelector != chain.Selector {
			continue
		}

		if ref.Type == datastore.ContractType(linkTokenTV.Type.String()) && ref.Version != nil && ref.Version.String() == linkTokenTV.Version.String() {
			linkToken, err := link_token.NewLinkToken(common.HexToAddress(ref.Address), chain.Client)
			require.NoError(t, err)

			return linkToken
		}
	}

	require.Failf(t, "link token not found", "chain %s", chain.Name())

	return nil
}
