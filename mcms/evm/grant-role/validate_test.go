package evmgrantrole

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfchain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfevm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"

	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
)

const (
	testTimelockAddr  = "0x0000000000000000000000000000000000000100"
	testMCMSAddr      = "0x0000000000000000000000000000000000000200"
	testCallProxyAddr = "0x0000000000000000000000000000000000000300"
)

type validateRefSpec struct {
	contractType cldf.ContractType
	address      string
	qualifier    string
}

func TestValidateMCMSRefs(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	mcmsInput := cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
	}

	tests := []struct {
		name    string
		refs    []validateRefSpec
		mcms    *cldf.MCMSTimelockProposalInput
		wantErr string
	}{
		{
			name: "nil MCMS only requires timelock",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, testTimelockAddr, ""},
			},
		},
		{
			name: "missing timelock ref",
			mcms: &mcmsInput,
			wantErr: fmt.Sprintf(
				"timelock not present on chain %d: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %d, Type: RBACTimelock}, found 0",
				chainselectors.TEST_90000001.Selector, chainselectors.TEST_90000001.Selector,
			),
		},
		{
			name: "missing mcms ref",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, testTimelockAddr, ""},
			},
			mcms: &mcmsInput,
			wantErr: fmt.Sprintf(
				"mcms not present on chain %d: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %d, Type: ProposerManyChainMultiSig}, found 0",
				chainselectors.TEST_90000001.Selector, chainselectors.TEST_90000001.Selector,
			),
		},
		{
			name: "missing call proxy ref",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, testTimelockAddr, ""},
				{mcmscontracts.ProposerManyChainMultisig, testMCMSAddr, ""},
			},
			mcms: &mcmsInput,
			wantErr: fmt.Sprintf(
				"validate call proxy ref for chain %d: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %d, Type: CallProxy}, found 0",
				chainselectors.TEST_90000001.Selector, chainselectors.TEST_90000001.Selector,
			),
		},
		{
			name: "success",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, testTimelockAddr, ""},
				{mcmscontracts.ProposerManyChainMultisig, testMCMSAddr, ""},
				{mcmscontracts.CallProxy, testCallProxyAddr, ""},
			},
			mcms: &mcmsInput,
		},
		{
			name: "invalid timelock address",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, "not-an-address", ""},
				{mcmscontracts.ProposerManyChainMultisig, testMCMSAddr, ""},
				{mcmscontracts.CallProxy, testCallProxyAddr, ""},
			},
			mcms: &mcmsInput,
			wantErr: fmt.Sprintf(
				`invalid timelock ref on chain %d: timelock address "not-an-address" is not a valid EVM address`,
				chainselectors.TEST_90000001.Selector,
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ds := datastore.NewMemoryDataStore()
			for _, ref := range tt.refs {
				addValidateRef(t, ds, chainselectors.TEST_90000001.Selector, ref.contractType, ref.address, version, ref.qualifier)
			}

			err := validateMCMSRefs(
				validateTestEnv(ds.Seal()),
				grantrole.SeqInput{
					ChainSelector: chainselectors.TEST_90000001.Selector,
					MCMS:          tt.mcms,
				},
			)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestValidateEVMChains(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	grantee := common.HexToAddress("0x00000000000000000000000000000000000000aa")

	tests := []struct {
		name    string
		env     cldf.Environment
		chains  []grantrole.SeqInput
		wantErr string
	}{
		{
			name: "chain not in environment",
			env:  validateTestEnv(datastore.NewMemoryDataStore().Seal()),
			chains: []grantrole.SeqInput{{
				ChainSelector: chainselectors.TEST_90000001.Selector,
				Grants: []grantrole.RoleGrant{{
					Role:      mcmssdk.TimelockRoleExecutor,
					Addresses: []common.Address{grantee},
				}},
			}},
			wantErr: fmt.Sprintf("EVM chain %d not found in environment", chainselectors.TEST_90000001.Selector),
		},
		{
			name: "unsupported role",
			env: grantRoleValidateEnv(t, chainselectors.TEST_90000001.Selector, version, []validateRefSpec{
				{mcmscontracts.RBACTimelock, testTimelockAddr, ""},
			}),
			chains: []grantrole.SeqInput{{
				ChainSelector: chainselectors.TEST_90000001.Selector,
				Grants: []grantrole.RoleGrant{{
					Role:      mcmssdk.TimelockRole(99),
					Addresses: []common.Address{grantee},
				}},
			}},
			wantErr: fmt.Sprintf("chain %d: grants[0]: unsupported timelock role Unknown", chainselectors.TEST_90000001.Selector),
		},
		{
			name: "success",
			env: grantRoleValidateEnv(t, chainselectors.TEST_90000001.Selector, version, []validateRefSpec{
				{mcmscontracts.RBACTimelock, testTimelockAddr, ""},
			}),
			chains: []grantrole.SeqInput{{
				ChainSelector: chainselectors.TEST_90000001.Selector,
				Grants: []grantrole.RoleGrant{{
					Role:      mcmssdk.TimelockRoleExecutor,
					Addresses: []common.Address{grantee},
				}},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateEVMChains(tt.env, tt.chains)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestParseTimelockAddress(t *testing.T) {
	t.Parallel()

	addr, err := parseTimelockAddress(testTimelockAddr)
	require.NoError(t, err)
	require.Equal(t, testTimelockAddr, addr.Hex())

	_, err = parseTimelockAddress("not-an-address")
	require.EqualError(t, err, `timelock address "not-an-address" is not a valid EVM address`)

	_, err = parseTimelockAddress("0x0000000000000000000000000000000000000000")
	require.EqualError(t, err, "timelock address must not be zero")
}

func TestTimelockAddress(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")

	t.Run("missing timelock ref", func(t *testing.T) {
		t.Parallel()

		_, err := timelockAddress(
			validateTestEnv(datastore.NewMemoryDataStore().Seal()),
			grantrole.SeqInput{ChainSelector: chainselectors.TEST_90000001.Selector},
		)
		require.EqualError(t, err, fmt.Sprintf(
			"resolve timelock for chain %d: no address ref matched query: expected exactly 1 ref matching query {ChainSelector: %d, Type: RBACTimelock}, found 0",
			chainselectors.TEST_90000001.Selector, chainselectors.TEST_90000001.Selector,
		))
	})

	t.Run("invalid timelock address", func(t *testing.T) {
		t.Parallel()

		ds := datastore.NewMemoryDataStore()
		addValidateRef(t, ds, chainselectors.TEST_90000001.Selector, mcmscontracts.RBACTimelock, "not-an-address", version, "")

		_, err := timelockAddress(
			validateTestEnv(ds.Seal()),
			grantrole.SeqInput{ChainSelector: chainselectors.TEST_90000001.Selector},
		)
		require.EqualError(t, err, `timelock address "not-an-address" is not a valid EVM address`)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ds := datastore.NewMemoryDataStore()
		addValidateRef(t, ds, chainselectors.TEST_90000001.Selector, mcmscontracts.RBACTimelock, testTimelockAddr, version, "")

		addr, err := timelockAddress(
			validateTestEnv(ds.Seal()),
			grantrole.SeqInput{ChainSelector: chainselectors.TEST_90000001.Selector},
		)
		require.NoError(t, err)
		require.Equal(t, testTimelockAddr, addr.Hex())
	})
}

func addValidateRef(
	t *testing.T,
	ds *datastore.MemoryDataStore,
	selector uint64,
	contractType cldf.ContractType,
	address string,
	version *semver.Version,
	qualifier string,
) {
	t.Helper()

	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       address,
		ChainSelector: selector,
		Type:          datastore.ContractType(contractType),
		Version:       version,
		Qualifier:     qualifier,
	}))
}

func validateTestEnv(ds datastore.DataStore) cldf.Environment {
	return cldf.Environment{
		Logger:     logger.Nop(),
		DataStore:  ds,
		GetContext: context.Background,
	}
}

func grantRoleValidateEnv(t *testing.T, selector uint64, version *semver.Version, refs []validateRefSpec) cldf.Environment {
	t.Helper()

	ds := datastore.NewMemoryDataStore()
	for _, ref := range refs {
		addValidateRef(t, ds, selector, ref.contractType, ref.address, version, ref.qualifier)
	}

	return cldf.Environment{
		Logger: logger.Nop(),
		BlockChains: cldfchain.NewBlockChains(map[uint64]cldfchain.BlockChain{
			selector: cldfevm.Chain{Selector: selector},
		}),
		DataStore:  ds.Seal(),
		GetContext: context.Background,
	}
}
