package evmreaders

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

func TestReaderGetRefs(t *testing.T) {
	t.Parallel()

	const selector uint64 = 90000001

	version := semver.MustParse("1.0.0")
	refs := map[datastore.ContractType]string{
		datastore.ContractType(mcmscontracts.RBACTimelock):                    "0x0000000000000000000000000000000000000100",
		datastore.ContractType(mcmscontracts.ProposerManyChainMultisig):       "0x0000000000000000000000000000000000000200",
		datastore.ContractType(mcmscontracts.CancellerManyChainMultisig):      "0x0000000000000000000000000000000000000300",
		datastore.ContractType(mcmscontracts.BypasserManyChainMultisig):       "0x0000000000000000000000000000000000000400",
		datastore.ContractType(mcmscontracts.ManyChainMultisig):               "0x0000000000000000000000000000000000000500",
		datastore.ContractType(mcmscontracts.ProposerAccessControllerAccount): "0x0000000000000000000000000000000000000600",
	}

	ds := datastore.NewMemoryDataStore()
	for typ, address := range refs {
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			Address:       address,
			ChainSelector: selector,
			Type:          typ,
			Version:       version,
		}))
	}
	env := readerTestEnv(ds.Seal())
	reader := Reader{}

	gotTimelock, err := reader.GetTimelockRef(env, selector, cldf.MCMSTimelockProposalInput{})
	require.NoError(t, err)
	require.Equal(t, refs[datastore.ContractType(mcmscontracts.RBACTimelock)], gotTimelock.Address)
	require.Equal(t, datastore.ContractType(mcmscontracts.RBACTimelock), gotTimelock.Type)

	tests := []struct {
		name    string
		action  mcmstypes.TimelockAction
		wantTyp datastore.ContractType
	}{
		{name: "default action uses proposer", wantTyp: datastore.ContractType(mcmscontracts.ProposerManyChainMultisig)},
		{name: "schedule uses proposer", action: mcmstypes.TimelockActionSchedule, wantTyp: datastore.ContractType(mcmscontracts.ProposerManyChainMultisig)},
		{name: "cancel uses canceller", action: mcmstypes.TimelockActionCancel, wantTyp: datastore.ContractType(mcmscontracts.CancellerManyChainMultisig)},
		{name: "bypass uses bypasser", action: mcmstypes.TimelockActionBypass, wantTyp: datastore.ContractType(mcmscontracts.BypasserManyChainMultisig)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, refErr := reader.GetMCMSRef(env, selector, cldf.MCMSTimelockProposalInput{TimelockAction: tt.action})
			require.NoError(t, refErr)
			require.Equal(t, refs[tt.wantTyp], got.Address)
			require.Equal(t, tt.wantTyp, got.Type)
		})
	}

	t.Run("qualified proposer", func(t *testing.T) {
		t.Parallel()

		qualifiedDS := datastore.NewMemoryDataStore()
		for typ, address := range refs {
			require.NoError(t, qualifiedDS.Addresses().Add(datastore.AddressRef{
				Address:       address,
				ChainSelector: selector,
				Type:          typ,
				Version:       version,
			}))
		}
		require.NoError(t, qualifiedDS.Addresses().Add(datastore.AddressRef{
			Address:       "0x0000000000000000000000000000000000000aaa",
			ChainSelector: selector,
			Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
			Version:       version,
			Qualifier:     "qualified",
		}))

		gotQualified, refErr := reader.GetMCMSRef(readerTestEnv(qualifiedDS.Seal()), selector, cldf.MCMSTimelockProposalInput{Qualifier: "qualified"})
		require.NoError(t, refErr)
		require.Equal(t, "0x0000000000000000000000000000000000000aaa", gotQualified.Address)
	})
}

func TestReaderErrors(t *testing.T) {
	t.Parallel()

	const selector uint64 = 90000001
	version := semver.MustParse("1.0.0")
	reader := Reader{}

	tests := []struct {
		name     string
		setup    func(t *testing.T) (cldf.Environment, uint64, cldf.MCMSTimelockProposalInput)
		run      func(t *testing.T, env cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) error
		wantErr  string
		contains bool
	}{
		{
			name: "invalid timelock action",
			setup: func(t *testing.T) (cldf.Environment, uint64, cldf.MCMSTimelockProposalInput) {
				t.Helper()
				return readerTestEnv(datastore.NewMemoryDataStore().Seal()), 1, cldf.MCMSTimelockProposalInput{
					TimelockAction: mcmstypes.TimelockAction("bad"),
				}
			},
			run: func(_ *testing.T, env cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) error {
				_, err := reader.GetMCMSRef(env, chainSelector, input)
				return err
			},
			wantErr: `invalid timelock action "bad"`,
		},
		{
			name: "missing datastore",
			setup: func(t *testing.T) (cldf.Environment, uint64, cldf.MCMSTimelockProposalInput) {
				t.Helper()
				return cldf.Environment{GetContext: context.Background}, 1, cldf.MCMSTimelockProposalInput{}
			},
			run: func(_ *testing.T, env cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) error {
				_, err := reader.GetTimelockRef(env, chainSelector, input)
				return err
			},
			wantErr: "datastore not available for chain 1",
		},
		{
			name: "missing qualified ref",
			setup: func(t *testing.T) (cldf.Environment, uint64, cldf.MCMSTimelockProposalInput) {
				t.Helper()
				return readerTestEnv(datastore.NewMemoryDataStore().Seal()), 1, cldf.MCMSTimelockProposalInput{Qualifier: "missing"}
			},
			run: func(_ *testing.T, env cldf.Environment, chainSelector uint64, input cldf.MCMSTimelockProposalInput) error {
				_, err := reader.GetMCMSRef(env, chainSelector, input)
				return err
			},
			contains: true,
			wantErr:  "Qualifier: missing",
		},
		{
			name: "call proxy ref",
			setup: func(t *testing.T) (cldf.Environment, uint64, cldf.MCMSTimelockProposalInput) {
				t.Helper()
				ds := datastore.NewMemoryDataStore()
				require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
					Address:       "0x0000000000000000000000000000000000000300",
					ChainSelector: selector,
					Type:          datastore.ContractType(mcmscontracts.CallProxy),
					Version:       version,
				}))

				return readerTestEnv(ds.Seal()), selector, cldf.MCMSTimelockProposalInput{}
			},
			run: func(t *testing.T, env cldf.Environment, chainSelector uint64, _ cldf.MCMSTimelockProposalInput) error {
				t.Helper()
				got, err := reader.GetCallProxyRef(env, chainSelector, "")
				if err != nil {
					return err
				}
				if got.Address != "0x0000000000000000000000000000000000000300" {
					t.Fatalf("unexpected call proxy address %q", got.Address)
				}

				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env, chainSelector, input := tt.setup(t)
			err := tt.run(t, env, chainSelector, input)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			if tt.contains {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestReaderGetChainMetadata_errors(t *testing.T) {
	t.Parallel()

	const selector uint64 = 90000001
	version := semver.MustParse("1.0.0")
	reader := Reader{}

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
		Address:       "0x0000000000000000000000000000000000000200",
		ChainSelector: selector,
		Type:          datastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       version,
	}))
	env := readerTestEnv(ds.Seal())

	_, err := reader.GetChainMetadata(env, selector, cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
	})
	require.ErrorContains(t, err, "build inspector for chain")

	_, err = reader.GetChainMetadata(cldf.Environment{GetContext: context.Background}, 1, cldf.MCMSTimelockProposalInput{})
	require.ErrorContains(t, err, "datastore not available")
}

func readerTestEnv(ds datastore.DataStore) cldf.Environment {
	return cldf.Environment{
		Logger:            logger.Nop(),
		ExistingAddresses: cldf.NewMemoryAddressBook(),
		DataStore:         ds,
		GetContext:        context.Background,
	}
}
