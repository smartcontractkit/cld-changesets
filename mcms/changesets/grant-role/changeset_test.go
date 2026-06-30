package grantrole

import (
	"context"
	"errors"
	"fmt"
	"testing"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/offchain/ocr"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmssdk "github.com/smartcontractkit/mcms/sdk"
	"github.com/stretchr/testify/require"
)

func testEnvironment(t *testing.T, ds datastore.DataStore) cldf.Environment {
	t.Helper()

	return *cldf.NewEnvironment(
		"test",
		logger.Test(t),
		nil,
		ds,
		nil,
		nil,
		func() context.Context { return t.Context() },
		ocr.OCRSecrets{},
		cldf_chain.NewBlockChains(nil),
	)
}

func TestChangeset_VerifyPreconditions_NoDatastore(t *testing.T) {
	t.Parallel()

	input := Input{
		Cfg: Config{
			GrantsByChain: map[uint64][]RoleGrant{
				chainselectors.TEST_90000001.Selector: {{
					Role:      mcmssdk.TimelockRoleProposer,
					Addresses: []string{"0x0000000000000000000000000000000000000001"},
				}},
			},
		},
	}

	err := Changeset{}.VerifyPreconditions(testEnvironment(t, nil), input)
	require.EqualError(t, err, "datastore is required for grant-role")
}

func TestChangeset_VerifyPreconditions_InvalidInput(t *testing.T) {
	t.Parallel()

	validAddress := "0x0000000000000000000000000000000000000001"
	tests := []struct {
		name    string
		input   Input
		wantErr string
	}{
		{
			name:    "no grants",
			input:   Input{},
			wantErr: "no role grants provided",
		},
		{
			name: "empty grants for chain",
			input: Input{Cfg: Config{GrantsByChain: map[uint64][]RoleGrant{
				chainselectors.TEST_90000001.Selector: {},
			}}},
			wantErr: fmt.Sprintf("chain %d: no role grants provided", chainselectors.TEST_90000001.Selector),
		},
		{
			name: "unsupported role",
			input: Input{Cfg: Config{GrantsByChain: map[uint64][]RoleGrant{
				chainselectors.TEST_90000001.Selector: {{Role: mcmssdk.TimelockRole(99), Addresses: []string{validAddress}}},
			}}},
			wantErr: fmt.Sprintf("chain %d grants[0]: unsupported timelock role Unknown", chainselectors.TEST_90000001.Selector),
		},
		{
			name: "no addresses",
			input: Input{Cfg: Config{GrantsByChain: map[uint64][]RoleGrant{
				chainselectors.TEST_90000001.Selector: {{Role: mcmssdk.TimelockRoleProposer}},
			}}},
			wantErr: fmt.Sprintf("chain %d grants[0]: no addresses provided", chainselectors.TEST_90000001.Selector),
		},
		{
			name: "empty address",
			input: Input{Cfg: Config{GrantsByChain: map[uint64][]RoleGrant{
				chainselectors.TEST_90000001.Selector: {{
					Role:      mcmssdk.TimelockRoleProposer,
					Addresses: []string{""},
				}},
			}}},
			wantErr: fmt.Sprintf("chain %d grants[0].addresses[0]: address must not be empty", chainselectors.TEST_90000001.Selector),
		},
		{
			name: "duplicate grant",
			input: Input{Cfg: Config{GrantsByChain: map[uint64][]RoleGrant{
				chainselectors.TEST_90000001.Selector: {{
					Role:      mcmssdk.TimelockRoleProposer,
					Addresses: []string{validAddress, validAddress},
				}},
			}}},
			wantErr: fmt.Sprintf("chain %d grants[0].addresses[1]: duplicate grant for role Proposer and address 0x0000000000000000000000000000000000000001", chainselectors.TEST_90000001.Selector),
		},
		{
			name: "invalid MCMS input",
			input: Input{
				MCMS: &cldf.MCMSTimelockProposalInput{},
				Cfg: Config{GrantsByChain: map[uint64][]RoleGrant{
					chainselectors.TEST_90000001.Selector: {{
						Role:      mcmssdk.TimelockRoleProposer,
						Addresses: []string{validAddress},
					}},
				}},
			},
			wantErr: `invalid MCMS timelock proposal input: invalid MCMS timelock proposal input: invalid timelock action ""`,
		},
	}

	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Changeset{}.VerifyPreconditions(env, tt.input)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestChangeset_VerifyPreconditions_unsupportedFamily(t *testing.T) {
	t.Parallel()

	err := Changeset{}.VerifyPreconditions(
		testEnvironment(t, datastore.NewMemoryDataStore().Seal()),
		Input{
			Cfg: Config{
				GrantsByChain: map[uint64][]RoleGrant{
					chainselectors.APTOS_MAINNET.Selector: {{
						Role:      mcmssdk.TimelockRoleProposer,
						Addresses: []string{"0x0000000000000000000000000000000000000001"},
					}},
				},
			},
		},
	)
	require.EqualError(t, err, `mcms grant-role: no sequence registered for family "aptos" (none registered)`)
}

func TestChangeset_Apply_unsupportedFamily(t *testing.T) {
	t.Parallel()

	_, err := Changeset{}.Apply(cldf.Environment{}, Input{
		Cfg: Config{
			GrantsByChain: map[uint64][]RoleGrant{
				chainselectors.APTOS_MAINNET.Selector: {{
					Role:      mcmssdk.TimelockRoleProposer,
					Addresses: []string{"0x0000000000000000000000000000000000000001"},
				}},
			},
		},
	})
	require.EqualError(t, err, fmt.Sprintf(`chain selector %d: mcms grant-role: no sequence registered for family "aptos" (none registered)`, chainselectors.APTOS_MAINNET.Selector))
}

func TestBuildOutput(t *testing.T) {
	t.Parallel()

	env := testEnvironment(t, datastore.NewMemoryDataStore().Seal())

	t.Run("success without MCMS", func(t *testing.T) {
		t.Parallel()

		out, err := buildOutput(env, nil, sequenceutils.OnChainOutput{}, nil)
		require.NoError(t, err)
		require.NotNil(t, out.DataStore)
	})

	t.Run("returns partial output on sequence error", func(t *testing.T) {
		t.Parallel()

		out, err := buildOutput(env, nil, sequenceutils.OnChainOutput{}, errors.New("sequence failed"))
		require.EqualError(t, err, "sequence failed")
		require.NotNil(t, out.DataStore)
	})
}
