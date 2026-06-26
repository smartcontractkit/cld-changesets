package evmgrantrole_test

import (
	"testing"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/require"

	grantrole "github.com/smartcontractkit/cld-changesets/mcms/changesets/grant-role"
	evmgrantrole "github.com/smartcontractkit/cld-changesets/mcms/evm/grant-role"
)

func TestRegistration(t *testing.T) {
	t.Parallel()

	reg := evmgrantrole.Registration()
	require.Equal(t, chainselectors.FamilyEVM, reg.Family)
	require.NotNil(t, reg.Sequence)
	require.NotNil(t, reg.Verify)
}

func TestRegistryHasEVMFamily(t *testing.T) {
	t.Parallel()

	require.Contains(t, grantrole.Registry.RegisteredFamilies(), chainselectors.FamilyEVM)

	seq, err := grantrole.Registry.SequenceForFamily(chainselectors.FamilyEVM)
	require.NoError(t, err)
	require.NotNil(t, seq)
}
