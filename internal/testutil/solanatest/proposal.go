package solanatest

import (
	"testing"

	"github.com/stretchr/testify/require"

	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	mcmslib "github.com/smartcontractkit/mcms"
	mcmssolana "github.com/smartcontractkit/mcms/sdk/solana"
	mcmstypes "github.com/smartcontractkit/mcms/types"

	solstate "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
)

// AssertMCMSSetConfigProposal verifies structure of a Solana set-config MCMS timelock proposal
// without executing it on-chain. Batch ops may appear as one merged operation per chain (new
// changeset output builder) or one operation per MCM role (legacy proposeutils path).
func AssertMCMSSetConfigProposal(
	t *testing.T,
	selector uint64,
	mcmsState *solstate.MCMSWithTimelockState,
	proposal mcmslib.TimelockProposal,
	wantAction mcmstypes.TimelockAction,
	wantDelay mcmstypes.Duration,
	wantDescription string,
) {
	t.Helper()

	require.Equal(t, wantAction, proposal.Action)
	require.Equal(t, wantDelay, proposal.Delay)
	require.Equal(t, wantDescription, proposal.Description)

	timelockAddr := mcmssolana.ContractAddress(mcmsState.TimelockProgram, mcmssolana.PDASeed(mcmsState.TimelockSeed))
	require.Equal(t, timelockAddr, proposal.TimelockAddresses[mcmstypes.ChainSelector(selector)])

	wantContractTypes := []string{
		string(mcmscontracts.ProposerManyChainMultisig),
		string(mcmscontracts.CancellerManyChainMultisig),
		string(mcmscontracts.BypasserManyChainMultisig),
	}
	require.NotEmpty(t, proposal.Operations)

	mcmProgram := mcmsState.McmProgram.String()
	gotContractTypes := make([]string, 0, len(wantContractTypes))

	for _, op := range proposal.Operations {
		require.Equal(t, mcmstypes.ChainSelector(selector), op.ChainSelector)
		require.NotEmpty(t, op.Transactions)

		for _, tx := range op.Transactions {
			require.Equal(t, mcmProgram, tx.To)
			require.NotEmpty(t, tx.Data)
			require.NotEmpty(t, tx.ContractType)
			gotContractTypes = append(gotContractTypes, tx.ContractType)
		}
	}

	require.ElementsMatch(t, wantContractTypes, uniqueStrings(gotContractTypes))
	for _, contractType := range wantContractTypes {
		require.Contains(t, gotContractTypes, contractType)
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		unique = append(unique, v)
	}

	return unique
}
