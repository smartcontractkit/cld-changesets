package changesets

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	linkcontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/link"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	evmstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/evm"
	linkchangesets "github.com/smartcontractkit/cld-changesets/link/changesets"

	cldfproposalutils "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"

	"github.com/smartcontractkit/chainlink-common/pkg/utils/tests"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"

	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"
)

func TestTransferToMCMSWithTimelockV2(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
	))
	require.NoError(t, err)

	chain := rt.Environment().BlockChains.EVMChains()[selector]

	// Setup contracts
	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(linkchangesets.DeployLinkToken), []uint64{selector}),
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	addrs, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)

	state, err := evmstate.MaybeLoadMCMSWithTimelockChainState(chain, addrs)
	require.NoError(t, err)

	linkToken, err := loadLinkTokenFromAddressBook(chain, addrs)
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(TransferToMCMSWithTimelockV2), TransferToMCMSWithTimelockConfig{
			ContractsByChain: map[uint64][]common.Address{
				selector: {linkToken.Address()},
			},
			MCMSConfig: cldfproposalutils.TimelockConfig{
				MinDelay: 0,
			},
		}),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)
	require.Len(t, rt.State().Proposals, 1)
	require.True(t, rt.State().Proposals[0].IsExecuted)

	// We expect now that the link token is owned by the MCMS timelock.
	linkToken, err = loadLinkTokenFromAddressBook(chain, addrs)
	require.NoError(t, err)

	o, err := linkToken.Owner(nil)
	require.NoError(t, err)
	require.Equal(t, state.Timelock.Address(), o)

	// Try a rollback to the deployer.
	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(TransferToDeployer), TransferToDeployerConfig{
			ContractAddress: linkToken.Address(),
			ChainSel:        selector,
		}),
	)
	require.NoError(t, err)

	o, err = linkToken.Owner(nil)
	require.NoError(t, err)
	require.Equal(t, chain.DeployerKey.From, o)
}

func TestTransferToMCMSWithTimelockV2DataStore(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
	))
	require.NoError(t, err)

	chain := rt.Environment().BlockChains.EVMChains()[selector]

	// Setup contracts
	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(linkchangesets.DeployLinkToken), []uint64{selector}),
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	addrs, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)

	state, err := evmstate.MaybeLoadMCMSWithTimelockChainState(chain, addrs)
	require.NoError(t, err)

	linkToken, err := loadLinkTokenFromDataStore(chain, rt.State().DataStore)
	require.NoError(t, err)

	// Remove LinkToken from AddressBook to simulate datastore only having the contract address
	// TODO: migrate DeployLinkToken to use datastore only
	linkAb := cldf.NewMemoryAddressBookFromMap(map[uint64]map[string]cldf.TypeAndVersion{
		selector: {
			linkToken.Address().String(): cldf.NewTypeAndVersion(linkcontracts.LinkToken, semvers.V1_0_0),
		},
	})
	err = rt.State().AddressBook.Remove(linkAb)
	require.NoError(t, err)

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(TransferToMCMSWithTimelockV2), TransferToMCMSWithTimelockConfig{
			ContractsByChain: map[uint64][]common.Address{
				selector: {linkToken.Address()},
			},
			MCMSConfig: cldfproposalutils.TimelockConfig{
				MinDelay: 0,
			},
		}),
		runtime.SignAndExecuteProposalsTask([]*ecdsa.PrivateKey{cldftesthelpers.TestXXXMCMSSigner}),
	)
	require.NoError(t, err)
	require.Len(t, rt.State().Proposals, 1)
	require.True(t, rt.State().Proposals[0].IsExecuted)

	// We expect now that the link token is owned by the MCMS timelock.
	o, err := linkToken.Owner(nil)
	require.NoError(t, err)
	require.Equal(t, state.Timelock.Address(), o)

	// Try a rollback to the deployer.
	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(TransferToDeployer), TransferToDeployerConfig{
			ContractAddress: linkToken.Address(),
			ChainSel:        selector,
		}),
	)
	require.NoError(t, err)

	o, err = linkToken.Owner(nil)
	require.NoError(t, err)
	require.Equal(t, chain.DeployerKey.From, o)
}

func TestRenounceTimelockDeployerConfigValidate(t *testing.T) {
	tests.SkipFlakey(t, "https://smartcontract-it.atlassian.net/browse/DX-724")
	t.Parallel()

	selector1 := chain_selectors.TEST_90000001.Selector
	selector2 := chain_selectors.TEST_90000002.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector1, selector2}),
	))
	require.NoError(t, err)

	// Deploy MCMS to selector 1 only, so we have a chain without MCMS
	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector1: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	for _, test := range []struct {
		name   string
		config RenounceTimelockDeployerConfig
		env    cldf.Environment
		err    string
	}{
		{
			name: "valid config",
			env:  rt.Environment(),
			config: RenounceTimelockDeployerConfig{
				ChainSel: selector1,
			},
		},
		{
			name: "invalid chain selector",
			env:  rt.Environment(),
			config: RenounceTimelockDeployerConfig{
				ChainSel: 0,
			},
			err: "invalid chain selector: chain selector must be set",
		},
		{
			name: "chain does not exists on env",
			env:  rt.Environment(),
			config: RenounceTimelockDeployerConfig{
				ChainSel: chain_selectors.ETHEREUM_TESTNET_SEPOLIA.Selector,
			},
			err: "chain selector: 16015286601757825753 not found in environment",
		},
		{
			name: "no MCMS deployed",
			env:  rt.Environment(),
			config: RenounceTimelockDeployerConfig{
				ChainSel: selector2,
			},
			// chain does not match any existing addresses
			err: "timelock not found on chain 5548718428018410741",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := test.config.Validate(test.env)
			if test.err != "" {
				require.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRenounceTimelockDeployer(t *testing.T) { //nolint:paralleltest
	selector := chain_selectors.TEST_90000001.Selector
	rt, err := runtime.New(t.Context(), runtime.WithEnvOpts(
		environment.WithEVMSimulated(t, []uint64{selector}),
	))
	require.NoError(t, err)

	chain := rt.Environment().BlockChains.EVMChains()[selector]

	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(DeployMCMSWithTimelockV2), map[uint64]cldfproposalutils.MCMSWithTimelockConfig{
			selector: cldftesthelpers.SingleGroupTimelockConfig(t),
		}),
	)
	require.NoError(t, err)

	addrs, err := rt.State().AddressBook.AddressesForChain(selector)
	require.NoError(t, err)

	state, err := evmstate.MaybeLoadMCMSWithTimelockChainState(chain, addrs)
	require.NoError(t, err)

	tl := state.Timelock
	require.NotNil(t, tl)

	adminRole, err := tl.ADMINROLE(nil)
	require.NoError(t, err)

	r, err := tl.GetRoleMemberCount(&bind.CallOpts{}, adminRole)
	require.NoError(t, err)
	require.Equal(t, int64(2), r.Int64())

	// Revoke Deployer
	err = rt.Exec(
		runtime.ChangesetTask(cldf.CreateLegacyChangeSet(RenounceTimelockDeployer), RenounceTimelockDeployerConfig{
			ChainSel: selector,
		}),
	)
	require.NoError(t, err)

	// Check that the deployer is no longer an admin
	r, err = tl.GetRoleMemberCount(&bind.CallOpts{}, adminRole)
	require.NoError(t, err)
	require.Equal(t, int64(1), r.Int64())

	// Retrieve the admin address
	admin, err := tl.GetRoleMember(&bind.CallOpts{}, adminRole, big.NewInt(0))
	require.NoError(t, err)

	// Check that the admin is the timelock
	require.Equal(t, tl.Address(), admin)
}

func loadLinkTokenFromAddressBook(chain cldf_evm.Chain, addresses map[string]cldf.TypeAndVersion) (*link_token.LinkToken, error) {
	linkToken := cldf.NewTypeAndVersion(linkcontracts.LinkToken, semvers.V1_0_0)

	// Convert map keys to a slice
	wantTypes := []cldf.TypeAndVersion{linkToken}

	// Ensure we either have the bundle or not.
	_, err := cldf.EnsureDeduped(addresses, wantTypes)
	if err != nil {
		return nil, fmt.Errorf("unable to check link token on chain %s error: %w", chain.Name(), err)
	}

	for address, tvStr := range addresses {
		if tvStr.Type == linkToken.Type && tvStr.Version.String() == linkToken.Version.String() {
			return link_token.NewLinkToken(common.HexToAddress(address), chain.Client)
		}
	}

	return nil, fmt.Errorf("link token not found on chain %s", chain.Name())
}

func loadLinkTokenFromDataStore(chain cldf_evm.Chain, dataStore datastore.DataStore) (*link_token.LinkToken, error) {
	linkToken := cldf.NewTypeAndVersion(linkcontracts.LinkToken, semvers.V1_0_0)

	refs, err := dataStore.Addresses().Fetch()
	if err != nil {
		return nil, err
	}

	for _, ref := range refs {
		if ref.ChainSelector != chain.Selector {
			continue
		}

		if ref.Type == datastore.ContractType(linkToken.Type.String()) && ref.Version.String() == linkToken.Version.String() {
			return link_token.NewLinkToken(common.HexToAddress(ref.Address), chain.Client)
		}
	}

	return nil, fmt.Errorf("link token not found on chain %s", chain.Name())
}
