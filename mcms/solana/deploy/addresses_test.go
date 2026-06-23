package soldeploy

import (
	"testing"

	solanago "github.com/gagliardetto/solana-go"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
	"github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
)

func TestRefAlreadyTracked(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	mcmProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgMCM))
	otherProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgTimelock))

	candidate := cldfdatastore.AddressRef{
		ChainSelector: selector,
		Address:       mcmProgram.String(),
		Type:          cldfdatastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
		Version:       &semvers.V1_0_0,
	}

	t.Run("tracked in pending refs", func(t *testing.T) {
		t.Parallel()
		require.True(t, refAlreadyTracked(nil, []cldfdatastore.AddressRef{candidate}, candidate))
	})

	t.Run("same type different address in existing datastore", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(cldfdatastore.AddressRef{
			ChainSelector: selector,
			Address:       otherProgram.String(),
			Type:          candidate.Type,
			Version:       &semvers.V1_0_0,
		}))

		require.False(t, refAlreadyTracked(ds.Seal(), nil, candidate))
	})

	t.Run("matching address in existing datastore", func(t *testing.T) {
		t.Parallel()

		ds := cldfdatastore.NewMemoryDataStore()
		require.NoError(t, ds.Addresses().Add(candidate))

		require.True(t, refAlreadyTracked(ds.Seal(), nil, candidate))
	})
}

func TestCollectOutputRefs(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	mcmProgram := solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgMCM))
	accountAddr := "ProposerAccessControllerAccount111111111111111111111"

	memDS := cldfdatastore.NewMemoryDataStore()
	require.NoError(t, memDS.Addresses().Add(cldfdatastore.AddressRef{
		ChainSelector: selector,
		Address:       accountAddr,
		Type:          cldfdatastore.ContractType(mcmscontracts.ProposerAccessControllerAccount),
		Version:       &semvers.V1_0_0,
	}))

	state := &legacysolana.MCMSWithTimelockState{
		MCMSWithTimelockPrograms: &legacysolana.MCMSWithTimelockPrograms{
			McmProgram:              mcmProgram,
			AccessControllerProgram: solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgAccessController)),
			TimelockProgram:         solanago.MustPublicKeyFromBase58(solutils.GetProgramID(solutils.ProgTimelock)),
		},
	}

	existingDS := cldfdatastore.NewMemoryDataStore()
	require.NoError(t, existingDS.Addresses().Add(cldfdatastore.AddressRef{
		ChainSelector: selector,
		Address:       mcmProgram.String(),
		Type:          cldfdatastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
		Version:       &semvers.V1_0_0,
	}))

	refs, err := collectOutputRefs(memDS, state, existingDS.Seal(), selector)
	require.NoError(t, err)
	require.Len(t, refs, 3)

	types := make(map[cldfdatastore.ContractType]string, len(refs))
	for _, ref := range refs {
		types[ref.Type] = ref.Address
	}

	require.Equal(t, accountAddr, types[cldfdatastore.ContractType(mcmscontracts.ProposerAccessControllerAccount)])
	require.Contains(t, types, cldfdatastore.ContractType(mcmscontracts.AccessControllerProgram))
	require.Contains(t, types, cldfdatastore.ContractType(mcmscontracts.RBACTimelockProgram))
	require.NotContains(t, types, cldfdatastore.ContractType(mcmscontracts.ManyChainMultisigProgram))
}

func TestDecorateAddressRefs(t *testing.T) {
	t.Parallel()

	selector := chainselectors.TEST_22222222222222222222222222222222222222222222.Selector
	ref := cldfdatastore.AddressRef{
		ChainSelector: selector,
		Address:       "11111111111111111111111111111111",
		Type:          cldfdatastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		Version:       &semvers.V1_0_0,
	}

	decorated := decorateAddressRefs([]cldfdatastore.AddressRef{ref}, "prod", "test-label")
	require.Len(t, decorated, 1)
	require.Equal(t, "prod", decorated[0].Qualifier)
	require.True(t, decorated[0].Labels.Contains("test-label"))

	unchanged := decorateAddressRefs([]cldfdatastore.AddressRef{ref}, "", "")
	require.Equal(t, []cldfdatastore.AddressRef{ref}, unchanged)
}
