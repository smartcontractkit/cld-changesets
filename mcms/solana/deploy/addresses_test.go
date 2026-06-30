package soldeploy

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	solanago "github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/internal/testutil/datastoretest"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	solutils "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
)

func TestLoadDeployedAddresses(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	v100 := semvers.V1_0_0
	v090 := semver.MustParse("0.9.0")

	acProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgAccessController))
	mcmProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgMCM))
	proposerAC := solanago.MustPublicKeyFromBase58("11111111111111111111111111111112")
	var proposerSeed legacysolana.PDASeed
	copy(proposerSeed[:], "proposer-seed-123456789012345678") // 32 bytes
	proposerMCM := legacysolana.EncodeAddressWithSeed(mcmProgram, proposerSeed)

	t.Run("nil datastore", func(t *testing.T) {
		t.Parallel()
		addrs, err := loadDeployedAddresses(nil, selector, "")
		require.NoError(t, err)
		require.Equal(t, deployedAddresses{}, addrs)
	})

	t.Run("matches v1.0.0 program", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       acProgram.String(),
			Type:          cldfdatastore.ContractType(mcmscontracts.AccessControllerProgram),
			Version:       &v100,
		}))

		addrs, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.NoError(t, err)
		require.Equal(t, acProgram, addrs.AccessControllerProgram)
	})

	t.Run("ignores older version", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       acProgram.String(),
			Type:          cldfdatastore.ContractType(mcmscontracts.AccessControllerProgram),
			Version:       v090,
		}))

		addrs, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.NoError(t, err)
		require.True(t, addrs.AccessControllerProgram.IsZero())
	})

	t.Run("respects qualifier", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       proposerAC.String(),
			Type:          cldfdatastore.ContractType(mcmscontracts.ProposerAccessControllerAccount),
			Version:       &v100,
			Qualifier:     "prod",
		}))

		addrs, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.NoError(t, err)
		require.True(t, addrs.ProposerAccessControllerAccount.IsZero())

		addrs, err = loadDeployedAddresses(ds.Seal(), selector, "prod")
		require.NoError(t, err)
		require.Equal(t, proposerAC, addrs.ProposerAccessControllerAccount)
	})

	t.Run("loads MCM instance seed and program from encoded address", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       proposerMCM,
			Type:          cldfdatastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
			Version:       &v100,
		}))

		addrs, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.NoError(t, err)
		require.Equal(t, mcmProgram, addrs.McmProgram)
		require.Equal(t, proposerSeed, addrs.ProposerMCMSeed)
		require.True(t, addrs.hasProposerMCM())
	})

	t.Run("invalid base58 address", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       "not-a-valid-address",
			Type:          cldfdatastore.ContractType(mcmscontracts.AccessControllerProgram),
			Version:       &v100,
		}))

		_, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.Error(t, err)
	})

	t.Run("invalid encoded MCM address", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       "not-a-valid-encoded-address",
			Type:          cldfdatastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
			Version:       &v100,
		}))

		_, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.Error(t, err)
	})

	t.Run("duplicate refs", func(t *testing.T) {
		t.Parallel()

		_, err := loadDeployedAddresses(datastoretest.NewDataStore([]cldfdatastore.AddressRef{
			{
				ChainSelector: selector,
				Address:       acProgram.String(),
				Type:          cldfdatastore.ContractType(mcmscontracts.AccessControllerProgram),
				Version:       &v100,
			},
			{
				ChainSelector: selector,
				Address:       mcmProgram.String(),
				Type:          cldfdatastore.ContractType(mcmscontracts.AccessControllerProgram),
				Version:       &v100,
			},
		}), selector, "")
		require.ErrorIs(t, err, cldfdatastore.ErrAddressRefQueryAmbiguous)
	})

	t.Run("MCM instance program mismatch", func(t *testing.T) {
		t.Parallel()

		timelockProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgTimelock))
		staleProposerMCM := legacysolana.EncodeAddressWithSeed(timelockProgram, proposerSeed)

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       mcmProgram.String(),
			Type:          cldfdatastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
			Version:       &v100,
		}))
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       staleProposerMCM,
			Type:          cldfdatastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
			Version:       &v100,
		}))

		_, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.ErrorContains(t, err, "embeds program")
		require.ErrorContains(t, err, string(mcmscontracts.ProposerManyChainMultisig))
	})

	t.Run("timelock instance program mismatch", func(t *testing.T) {
		t.Parallel()

		timelockProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgTimelock))
		var timelockSeed legacysolana.PDASeed
		copy(timelockSeed[:], "timelock-seed-123456789012345678")
		staleTimelock := legacysolana.EncodeAddressWithSeed(mcmProgram, timelockSeed)

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       timelockProgram.String(),
			Type:          cldfdatastore.ContractType(mcmscontracts.RBACTimelockProgram),
			Version:       &v100,
		}))
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       staleTimelock,
			Type:          cldfdatastore.ContractType(mcmscontracts.RBACTimelock),
			Version:       &v100,
		}))

		_, err := loadDeployedAddresses(ds.Seal(), selector, "")
		require.ErrorContains(t, err, "embeds program")
		require.ErrorContains(t, err, string(mcmscontracts.RBACTimelock))
	})
}
