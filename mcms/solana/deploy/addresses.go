package soldeploy

import (
	"slices"

	solanago "github.com/gagliardetto/solana-go"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
)

func loadChainState(ds cldfdatastore.DataStore, chainSelector uint64) (*legacysolana.MCMSWithTimelockState, error) {
	var refs []cldfdatastore.AddressRef
	if ds != nil {
		refs = ds.Addresses().Filter(cldfdatastore.AddressRefByChainSelector(chainSelector))
	}

	return legacysolana.MaybeLoadMCMSWithTimelockChainStateV2(refs)
}

func addressRefsFromDatastore(ds cldfdatastore.MutableDataStore) ([]cldfdatastore.AddressRef, error) {
	if ds == nil {
		return nil, nil
	}

	return ds.Addresses().Fetch()
}

func collectOutputRefs(
	memDS cldfdatastore.MutableDataStore,
	state *legacysolana.MCMSWithTimelockState,
	existingDS cldfdatastore.DataStore,
	chainSelector uint64,
) ([]cldfdatastore.AddressRef, error) {
	refs, err := addressRefsFromDatastore(memDS)
	if err != nil {
		return nil, err
	}

	for _, programRef := range programRefsFromState(state, chainSelector) {
		if refAlreadyTracked(existingDS, refs, programRef) {
			continue
		}
		refs = append(refs, programRef)
	}

	return refs, nil
}

func programRefsFromState(state *legacysolana.MCMSWithTimelockState, chainSelector uint64) []cldfdatastore.AddressRef {
	programs := []struct {
		program solanago.PublicKey
		typ     cldfdatastore.ContractType
	}{
		{state.AccessControllerProgram, cldfdatastore.ContractType(mcmscontracts.AccessControllerProgram)},
		{state.McmProgram, cldfdatastore.ContractType(mcmscontracts.ManyChainMultisigProgram)},
		{state.TimelockProgram, cldfdatastore.ContractType(mcmscontracts.RBACTimelockProgram)},
	}

	refs := make([]cldfdatastore.AddressRef, 0, len(programs))
	for _, program := range programs {
		if program.program.IsZero() {
			continue
		}
		refs = append(refs, cldfdatastore.AddressRef{
			ChainSelector: chainSelector,
			Address:       program.program.String(),
			Type:          program.typ,
			Version:       &semvers.V1_0_0,
		})
	}

	return refs
}

func refAlreadyTracked(
	existingDS cldfdatastore.DataStore,
	refs []cldfdatastore.AddressRef,
	candidate cldfdatastore.AddressRef,
) bool {
	if hasMatchingRef(refs, candidate) {
		return true
	}
	if existingDS == nil {
		return false
	}

	existing := existingDS.Addresses().Filter(
		cldfdatastore.AddressRefByChainSelector(candidate.ChainSelector),
		cldfdatastore.AddressRefByType(candidate.Type),
	)
	for _, ref := range existing {
		if ref.Address == candidate.Address {
			return true
		}
	}

	return false
}

func hasMatchingRef(refs []cldfdatastore.AddressRef, candidate cldfdatastore.AddressRef) bool {
	for _, ref := range refs {
		if ref.ChainSelector == candidate.ChainSelector &&
			ref.Type == candidate.Type &&
			ref.Address == candidate.Address {
			return true
		}
	}

	return false
}

func decorateAddressRefs(
	refs []cldfdatastore.AddressRef,
	qualifier, label string,
) []cldfdatastore.AddressRef {
	if qualifier == "" && label == "" {
		return refs
	}

	out := make([]cldfdatastore.AddressRef, len(refs))
	for i, ref := range refs {
		out[i] = ref
		if qualifier != "" && isMCMSContract(ref.Type) {
			out[i].Qualifier = qualifier
		}
		if label != "" {
			out[i].Labels = cldfdatastore.NewLabelSet(label)
		}
	}

	return out
}

func isMCMSContract(contractType cldfdatastore.ContractType) bool {
	mcmsTypes := []cldfdatastore.ContractType{
		cldfdatastore.ContractType(mcmscontracts.RBACTimelock),
		cldfdatastore.ContractType(mcmscontracts.RBACTimelockProgram),
		cldfdatastore.ContractType(mcmscontracts.ManyChainMultisig),
		cldfdatastore.ContractType(mcmscontracts.ManyChainMultisigProgram),
		cldfdatastore.ContractType(mcmscontracts.ProposerManyChainMultisig),
		cldfdatastore.ContractType(mcmscontracts.BypasserManyChainMultisig),
		cldfdatastore.ContractType(mcmscontracts.CancellerManyChainMultisig),
		cldfdatastore.ContractType(mcmscontracts.CallProxy),
	}

	return slices.Contains(mcmsTypes, contractType)
}

func qualifierFromConfig(q *string) string {
	if q == nil {
		return ""
	}

	return *q
}

func labelFromConfig(l *string) string {
	if l == nil {
		return ""
	}

	return *l
}
