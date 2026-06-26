package soldeploy

import (
	"fmt"

	solanago "github.com/gagliardetto/solana-go"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
	legacysolana "github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana"
)

// deployedAddresses holds the on-chain state of an MCMS+timelock deployment on one
// Solana chain. Zero values mean the corresponding program or account has not yet
// been deployed/initialized.
type deployedAddresses struct {
	AccessControllerProgram          solanago.PublicKey
	ProposerAccessControllerAccount  solanago.PublicKey
	ExecutorAccessControllerAccount  solanago.PublicKey
	CancellerAccessControllerAccount solanago.PublicKey
	BypasserAccessControllerAccount  solanago.PublicKey
	McmProgram                       solanago.PublicKey
	ProposerMCMSeed                  legacysolana.PDASeed
	CancellerMCMSeed                 legacysolana.PDASeed
	BypasserMCMSeed                  legacysolana.PDASeed
	TimelockProgram                  solanago.PublicKey
	TimelockSeed                     legacysolana.PDASeed
}

func (d deployedAddresses) hasProposerMCM() bool {
	return d.ProposerMCMSeed != (legacysolana.PDASeed{})
}
func (d deployedAddresses) hasCancellerMCM() bool {
	return d.CancellerMCMSeed != (legacysolana.PDASeed{})
}
func (d deployedAddresses) hasBypasserMCM() bool {
	return d.BypasserMCMSeed != (legacysolana.PDASeed{})
}
func (d deployedAddresses) hasTimelock() bool { return d.TimelockSeed != (legacysolana.PDASeed{}) }

// loadDeployedAddresses returns the current deployment state for the given chain
// and qualifier by reading address refs from the datastore. A zero value in any
// field means the corresponding program or account has not been deployed yet.
func loadDeployedAddresses(ds cldfdatastore.DataStore, chainSelector uint64, qualifier string) (deployedAddresses, error) {
	if ds == nil {
		return deployedAddresses{}, nil
	}

	var addrs deployedAddresses

	// findRef returns the address string for a given contract type at this
	// deploy package's version. Qualifier is always matched exactly, including "".
	// Returns ("", nil) when no ref matches; an error when multiple refs match.
	findRef := func(ct cldf.ContractType) (string, error) {
		version := semvers.V1_0_0
		refs := ds.Addresses().Filter(
			cldfdatastore.AddressRefByChainSelector(chainSelector),
			cldfdatastore.AddressRefByType(cldfdatastore.ContractType(ct)),
			cldfdatastore.AddressRefByQualifier(qualifier),
			cldfdatastore.AddressRefByVersion(&version),
		)
		switch len(refs) {
		case 0:
			return "", nil
		case 1:
			return refs[0].Address, nil
		default:
			return "", fmt.Errorf(
				"%w: chain selector %d contract type %s qualifier %q version %s: found %d refs",
				cldfdatastore.ErrAddressRefQueryAmbiguous,
				chainSelector, ct, qualifier, version, len(refs),
			)
		}
	}

	loadPubkey := func(ct cldf.ContractType, dest *solanago.PublicKey) error {
		addr, err := findRef(ct)
		if err != nil {
			return err
		}
		if addr == "" {
			return nil
		}
		pk, err := solanago.PublicKeyFromBase58(addr)
		if err != nil {
			return fmt.Errorf("parse %s address %q: %w", ct, addr, err)
		}
		*dest = pk

		return nil
	}

	// Plain base58 addresses (program IDs and AC accounts)
	for _, entry := range []struct {
		ct   cldf.ContractType
		dest *solanago.PublicKey
	}{
		{mcmscontracts.AccessControllerProgram, &addrs.AccessControllerProgram},
		{mcmscontracts.ProposerAccessControllerAccount, &addrs.ProposerAccessControllerAccount},
		{mcmscontracts.ExecutorAccessControllerAccount, &addrs.ExecutorAccessControllerAccount},
		{mcmscontracts.CancellerAccessControllerAccount, &addrs.CancellerAccessControllerAccount},
		{mcmscontracts.BypasserAccessControllerAccount, &addrs.BypasserAccessControllerAccount},
		{mcmscontracts.ManyChainMultisigProgram, &addrs.McmProgram},
		{mcmscontracts.RBACTimelockProgram, &addrs.TimelockProgram},
	} {
		if err := loadPubkey(entry.ct, entry.dest); err != nil {
			return deployedAddresses{}, err
		}
	}

	// Seed-encoded MCM instance addresses (programID:seed)
	for ct, dst := range map[cldf.ContractType]*legacysolana.PDASeed{
		mcmscontracts.ProposerManyChainMultisig:  &addrs.ProposerMCMSeed,
		mcmscontracts.CancellerManyChainMultisig: &addrs.CancellerMCMSeed,
		mcmscontracts.BypasserManyChainMultisig:  &addrs.BypasserMCMSeed,
	} {
		addr, err := findRef(ct)
		if err != nil {
			return deployedAddresses{}, err
		}
		if addr == "" {
			continue
		}
		programID, seed, err := legacysolana.DecodeAddressWithSeed(addr)
		if err != nil {
			return deployedAddresses{}, fmt.Errorf("decode %s address %q: %w", ct, addr, err)
		}
		if err := reconcileInstanceProgramID(
			&addrs.McmProgram, programID, ct, mcmscontracts.ManyChainMultisigProgram,
		); err != nil {
			return deployedAddresses{}, err
		}
		*dst = seed
	}

	// Seed-encoded timelock instance (programID:seed)
	if addr, err := findRef(mcmscontracts.RBACTimelock); err != nil {
		return deployedAddresses{}, err
	} else if addr != "" {
		programID, seed, err := legacysolana.DecodeAddressWithSeed(addr)
		if err != nil {
			return deployedAddresses{}, fmt.Errorf("decode %s address %q: %w", mcmscontracts.RBACTimelock, addr, err)
		}
		if err := reconcileInstanceProgramID(
			&addrs.TimelockProgram, programID, mcmscontracts.RBACTimelock, mcmscontracts.RBACTimelockProgram,
		); err != nil {
			return deployedAddresses{}, err
		}
		addrs.TimelockSeed = seed
	}

	return addrs, nil
}

// reconcileInstanceProgramID sets programLevel from embedded when unset, or errors on mismatch.
func reconcileInstanceProgramID(
	programLevel *solanago.PublicKey,
	embedded solanago.PublicKey,
	instanceType cldf.ContractType,
	programType cldf.ContractType,
) error {
	if programLevel.IsZero() {
		*programLevel = embedded

		return nil
	}
	if *programLevel != embedded {
		return fmt.Errorf(
			"%s instance ref embeds program %s but %s ref is %s",
			instanceType, embedded, programType, *programLevel,
		)
	}

	return nil
}
