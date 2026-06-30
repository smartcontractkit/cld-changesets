package setconfig_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"

	"github.com/smartcontractkit/cld-changesets/datastore/refkey"
	"github.com/smartcontractkit/cld-changesets/internal/semvers"

	setconfig "github.com/smartcontractkit/cld-changesets/mcms/changesets/set-config"
)

func TestChangeset_VerifyPreconditions_zeroRefChainSelector(t *testing.T) {
	t.Parallel()

	cfg := cldftesthelpers.SingleGroupMCMS(t)
	input := setConfigInput(
		[]setconfig.ContractSetConfig{
			{
				Ref: refkey.RefKey{
					ChainSelector: 0,
					Type:          contractRef(chain_selectors.TEST_90000001.Selector, mcmscontracts.ProposerManyChainMultisig, "").Type,
					Version:       &semvers.V1_0_0,
				},
				Config: cfg,
			},
		},
		nil,
	)

	err := setconfig.Changeset{}.VerifyPreconditions(cldf.Environment{}, input)
	require.Error(t, err)
	require.EqualError(t, err, "targets[0]: ref chain selector is required")
}

func TestChangeset_Apply_unsupportedFamily(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.APTOS_TESTNET.Selector
	cfg := cldftesthelpers.SingleGroupMCMS(t)

	_, err := setconfig.Changeset{}.Apply(cldf.Environment{}, setConfigInput(
		[]setconfig.ContractSetConfig{
			{
				Ref: refkey.New(
					selector,
					contractRef(chain_selectors.TEST_90000001.Selector, mcmscontracts.ProposerManyChainMultisig, "").Type,
					&semvers.V1_0_0,
					"",
				),
				Config: cfg,
			},
		},
		nil,
	))
	require.ErrorContains(t, err, fmt.Sprintf("chain selector %d:", selector))
	require.ErrorContains(t, err, `no sequence registered for family "aptos"`)
}

func contractRef(chainSelector uint64, contractType cldf.ContractType, qualifier string) refkey.RefKey {
	return refkey.New(chainSelector, cldfdatastore.ContractType(contractType), &semvers.V1_0_0, qualifier)
}

func setConfigInput(targets []setconfig.ContractSetConfig, mcms *cldf.MCMSTimelockProposalInput) setconfig.Input {
	return setconfig.Input{
		Cfg:  setconfig.Config{Targets: targets},
		MCMS: mcms,
	}
}
