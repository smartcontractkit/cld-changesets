// Package solanatest provides test utilities for Solana MCMS integration tests.
package solanatest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	"github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
)

// NewDataStoreWithMCMSPrograms seeds a datastore with canonical MCMS program IDs.
// The test validator preloads the same programs via WithSolanaContainer; program
// deploy is skipped because artifacts lack -keypair.json files required by
// solana program deploy for fixed program IDs.
func NewDataStoreWithMCMSPrograms(t *testing.T, selector uint64) datastore.DataStore {
	t.Helper()

	v := semvers.V1_0_0
	ds := datastore.NewMemoryDataStore()
	for _, entry := range []struct {
		addr string
		ct   datastore.ContractType
	}{
		{solutils.GetProgramID(solutils.ProgAccessController), datastore.ContractType(mcmscontracts.AccessControllerProgram)},
		{solutils.GetProgramID(solutils.ProgMCM), datastore.ContractType(mcmscontracts.ManyChainMultisigProgram)},
		{solutils.GetProgramID(solutils.ProgTimelock), datastore.ContractType(mcmscontracts.RBACTimelockProgram)},
	} {
		require.NoError(t, ds.Addresses().Add(datastore.AddressRef{
			ChainSelector: selector,
			Address:       entry.addr,
			Type:          entry.ct,
			Version:       &v,
		}))
	}

	return ds.Seal()
}
