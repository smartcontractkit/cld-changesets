package evmtransfertotimelock

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfchain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldfevm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	transfertotimelock "github.com/smartcontractkit/cld-changesets/mcms/changesets/transfer-to-timelock"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"

	_ "github.com/smartcontractkit/cld-changesets/mcms/evm/readers"
)

const (
	testTimelockAddr = "0x0000000000000000000000000000000000000100"
	testMCMSAddr     = "0x0000000000000000000000000000000000000200"
)

func TestValidateMCMS(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
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
			name:    "missing timelock ref",
			mcms:    &mcmsInput,
			wantErr: "timelock not present",
		},
		{
			name: "missing mcms ref",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, testTimelockAddr},
			},
			mcms:    &mcmsInput,
			wantErr: "mcms not present",
		},
		{
			name: "success",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, testTimelockAddr},
				{mcmscontracts.ProposerManyChainMultisig, testMCMSAddr},
			},
			mcms: &mcmsInput,
		},
		{
			name: "invalid timelock address",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, "not-an-address"},
				{mcmscontracts.ProposerManyChainMultisig, testMCMSAddr},
			},
			mcms:    &mcmsInput,
			wantErr: "invalid timelock ref",
		},
		{
			name: "invalid mcms address",
			refs: []validateRefSpec{
				{mcmscontracts.RBACTimelock, testTimelockAddr},
				{mcmscontracts.ProposerManyChainMultisig, "0x0000000000000000000000000000000000000000"},
			},
			mcms:    &mcmsInput,
			wantErr: "invalid mcms ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ds := datastore.NewMemoryDataStore()
			for _, ref := range tt.refs {
				addValidateRef(t, ds, selector, ref.contractType, ref.address, version)
			}

			err := validateMCMS(
				validateTestEnv(ds.Seal()),
				transfertotimelock.ChainInput{
					ChainSelector: selector,
					MCMS:          tt.mcms,
				},
			)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestTimelockAddress_nilMCMS(t *testing.T) {
	t.Parallel()

	_, err := timelockAddress(
		validateTestEnv(datastore.NewMemoryDataStore().Seal()),
		transfertotimelock.ChainInput{ChainSelector: chainselectors.TEST_90000001.Selector},
	)
	require.ErrorContains(t, err, "MCMS timelock proposal input is required")
}

func TestTimelockAddress_invalidAddress(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	version := semver.MustParse("1.0.0")
	mcmsInput := cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
	}

	ds := datastore.NewMemoryDataStore()
	addValidateRef(t, ds, selector, mcmscontracts.RBACTimelock, "not-an-address", version)

	_, err := timelockAddress(
		validateTestEnv(ds.Seal()),
		transfertotimelock.ChainInput{
			ChainSelector: selector,
			MCMS:          &mcmsInput,
		},
	)
	require.ErrorContains(t, err, `invalid timelock address "not-an-address"`)
}

func TestValidateContracts_nilDeployerKey(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	version := semver.MustParse("1.0.0")
	ref := refkey.New(selector, "LinkToken", version, "")
	env := cldf.Environment{
		Logger: logger.Nop(),
		BlockChains: cldfchain.NewBlockChains(map[uint64]cldfchain.BlockChain{
			selector: cldfevm.Chain{Selector: selector},
		}),
	}

	err := validateContracts(env, transfertotimelock.ChainInput{
		ChainSelector: selector,
		Contracts:     []refkey.RefKey{ref},
		MCMS:          &cldf.MCMSTimelockProposalInput{},
	})
	require.ErrorContains(t, err, "missing deployer key")
}

func TestValidateContracts_duplicateContract(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	version := semver.MustParse("1.0.0")
	contract := common.HexToAddress("0x0000000000000000000000000000000000000abc")
	ref := refkey.New(selector, "LinkToken", version, "")

	ds := datastore.NewMemoryDataStore()
	addValidateRef(t, ds, selector, "LinkToken", contract.Hex(), version)
	addValidateRef(t, ds, selector, mcmscontracts.RBACTimelock, testTimelockAddr, version)
	addValidateRef(t, ds, selector, mcmscontracts.ProposerManyChainMultisig, testMCMSAddr, version)

	env := validateTestEnv(ds.Seal())
	env.BlockChains = cldfchain.NewBlockChains(map[uint64]cldfchain.BlockChain{
		selector: cldfevm.Chain{
			Selector:    selector,
			DeployerKey: &bind.TransactOpts{From: common.HexToAddress("0x1")},
		},
	})

	err := validateContracts(env, transfertotimelock.ChainInput{
		ChainSelector: selector,
		Contracts:     []refkey.RefKey{ref, ref},
		MCMS:          &cldf.MCMSTimelockProposalInput{},
	})
	require.ErrorContains(t, err, "duplicate contract ref")
}

func TestValidateContracts_duplicateResolvedAddress(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	version1 := semver.MustParse("1.0.0")
	version2 := semver.MustParse("1.1.0")
	contract := common.HexToAddress("0x0000000000000000000000000000000000000abc")
	ref1 := refkey.New(selector, "LinkToken", version1, "")
	ref2 := refkey.New(selector, "LinkTokenAlias", version2, "")

	ds := datastore.NewMemoryDataStore()
	addValidateRef(t, ds, selector, "LinkToken", contract.Hex(), version1)
	addValidateRef(t, ds, selector, "LinkTokenAlias", contract.Hex(), version2)
	addValidateRef(t, ds, selector, mcmscontracts.RBACTimelock, testTimelockAddr, version1)
	addValidateRef(t, ds, selector, mcmscontracts.ProposerManyChainMultisig, testMCMSAddr, version1)

	env := validateTestEnv(ds.Seal())
	env.BlockChains = cldfchain.NewBlockChains(map[uint64]cldfchain.BlockChain{
		selector: cldfevm.Chain{
			Selector:    selector,
			DeployerKey: &bind.TransactOpts{From: common.HexToAddress("0x1")},
		},
	})

	err := validateContracts(env, transfertotimelock.ChainInput{
		ChainSelector: selector,
		Contracts:     []refkey.RefKey{ref1, ref2},
		MCMS:          &cldf.MCMSTimelockProposalInput{},
	})
	require.ErrorContains(t, err, "duplicate contract address")
}

func TestResolveEVMAddress(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_90000001.Selector
	version := semver.MustParse("1.0.0")
	contract := common.HexToAddress("0x0000000000000000000000000000000000000abc")
	ref := refkey.New(selector, "LinkToken", version, "")

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       contract.Hex(),
		ChainSelector: selector,
		Type:          datastore.ContractType("LinkToken"),
		Version:       version,
	}))

	env := validateTestEnv(ds.Seal())

	got, err := resolveEVMAddress(env, selector, ref)
	require.NoError(t, err)
	require.Equal(t, contract, got)

	refWithoutSelector := refkey.RefKey{
		Type:    "LinkToken",
		Version: version,
	}
	got, err = resolveEVMAddress(env, selector, refWithoutSelector)
	require.NoError(t, err)
	require.Equal(t, contract, got)

	_, err = resolveEVMAddress(cldf.Environment{}, selector, ref)
	require.ErrorContains(t, err, "missing datastore")

	missingRef := refkey.New(selector, "MissingToken", version, "")
	key, keyErr := missingRef.Key()
	require.NoError(t, keyErr)
	_, err = resolveEVMAddress(env, selector, missingRef)
	require.EqualError(t, err, fmt.Sprintf("address ref %v: %v", key, datastore.ErrAddressRefNotFound))
}

func TestValidateContractOwner(t *testing.T) {
	t.Parallel()

	contract := common.HexToAddress("0x0000000000000000000000000000000000000abc")
	deployer := common.HexToAddress("0x0000000000000000000000000000000000000001")
	timelock := common.HexToAddress("0x0000000000000000000000000000000000000002")
	other := common.HexToAddress("0x0000000000000000000000000000000000000003")

	tests := []struct {
		name       string
		owner      common.Address
		onlyAccept bool
		wantErr    string
	}{
		{
			name:  "timelock already owns",
			owner: timelock,
		},
		{
			name:       "only accept with deployer owner",
			owner:      deployer,
			onlyAccept: true,
		},
		{
			name:       "only accept rejects third party",
			owner:      other,
			onlyAccept: true,
			wantErr:    "only accept ownership requires current owner to be deployer or timelock",
		},
		{
			name:    "full transfer requires deployer owner",
			owner:   deployer,
			wantErr: "",
		},
		{
			name:    "full transfer rejects third party",
			owner:   other,
			wantErr: "not owned by the deployer key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateContractOwner(contract, tt.owner, deployer, timelock, tt.onlyAccept)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

type validateRefSpec struct {
	contractType cldf.ContractType
	address      string
}

func addValidateRef(
	t *testing.T,
	ds *datastore.MemoryDataStore,
	selector uint64,
	contractType cldf.ContractType,
	address string,
	version *semver.Version,
) {
	t.Helper()

	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       address,
		ChainSelector: selector,
		Type:          datastore.ContractType(contractType),
		Version:       version,
		Qualifier:     "",
	}))
}

func validateTestEnv(ds datastore.DataStore) cldf.Environment {
	return cldf.Environment{
		Logger:     logger.Nop(),
		DataStore:  ds,
		GetContext: context.Background,
	}
}
