package stellardeploy

import (
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	stellarmocks "github.com/smartcontractkit/mcms/sdk/stellar/mocks"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

func testInitContractID(t *testing.T, seed byte) string {
	t.Helper()

	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = seed + byte(i)
	}

	id, err := strkey.Encode(strkey.VersionByteContract, raw)
	require.NoError(t, err)

	return id
}

func testInitMCMSConfig(t *testing.T) mcmstypes.Config {
	t.Helper()

	cfg, err := mcmstypes.NewConfig(
		1,
		[]common.Address{
			common.HexToAddress("0x1111111111111111111111111111111111111111"),
		},
		nil,
	)
	require.NoError(t, err)

	return cfg
}

func TestInitializeMCMS_NilInvoker(t *testing.T) {
	t.Parallel()

	err := initializeMCMS(t.Context(), nil, initializeMCMSInput{})

	require.ErrorContains(t, err, "invoker is nil")
}

func TestInitializeMCMS_InvalidInput(t *testing.T) {
	t.Parallel()

	cfg := testInitMCMSConfig(t)

	valid := initializeMCMSInput{
		ContractID:    testInitContractID(t, 1),
		Owner:         testStellarAccountAddress(t),
		ChainID:       chainselectors.STELLAR_LOCALNET.ChainID,
		Config:        &cfg,
		InstanceLabel: "PROPOSER",
	}

	tests := []struct {
		name    string
		mutate  func(*initializeMCMSInput)
		wantErr string
	}{
		{
			name: "empty contract ID",
			mutate: func(in *initializeMCMSInput) {
				in.ContractID = ""
			},
			wantErr: "contract ID is empty",
		},
		{
			name: "empty owner",
			mutate: func(in *initializeMCMSInput) {
				in.Owner = ""
			},
			wantErr: "owner is empty",
		},
		{
			name: "empty chain ID",
			mutate: func(in *initializeMCMSInput) {
				in.ChainID = ""
			},
			wantErr: "chain ID is empty",
		},
		{
			name: "nil config",
			mutate: func(in *initializeMCMSInput) {
				in.Config = nil
			},
			wantErr: "config is nil",
		},
		{
			name: "empty instance label",
			mutate: func(in *initializeMCMSInput) {
				in.InstanceLabel = ""
			},
			wantErr: "instance label is empty",
		},
		{
			name: "instance label too long",
			mutate: func(in *initializeMCMSInput) {
				in.InstanceLabel = strings.Repeat("a", 33)
			},
			wantErr: "exceeds 32 bytes",
		},
		{
			name: "invalid config",
			mutate: func(in *initializeMCMSInput) {
				invalidConfig := mcmstypes.Config{}
				in.Config = &invalidConfig
			},
			wantErr: "validate stellar MCMS config",
		},
		{
			name: "unknown chain ID",
			mutate: func(in *initializeMCMSInput) {
				in.ChainID = "unknown-chain-id"
			},
			wantErr: "get stellar network passphrase from chain ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dependency := stellarmocks.NewInvoker(t)

			input := valid
			tt.mutate(&input)

			err := initializeMCMS(t.Context(), dependency, input)

			require.ErrorContains(t, err, tt.wantErr)
			dependency.AssertNotCalled(t, "InvokeContract", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestInitializeTimelock_NilInvoker(t *testing.T) {
	t.Parallel()

	err := initializeTimelock(t.Context(), nil, initializeTimelockInput{})

	require.ErrorContains(t, err, "invoker is nil")
}

func TestInitializeTimelock_InvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*testing.T, *initializeTimelockInput)
		wantErr string
	}{
		{
			name: "empty contract ID",
			mutate: func(_ *testing.T, in *initializeTimelockInput) {
				in.ContractID = ""
			},
			wantErr: "contract ID is empty",
		},
		{
			name: "empty proposers",
			mutate: func(_ *testing.T, in *initializeTimelockInput) {
				in.Proposers = nil
			},
			wantErr: "proposers are empty",
		},
		{
			name: "empty cancellers",
			mutate: func(_ *testing.T, in *initializeTimelockInput) {
				in.Cancellers = nil
			},
			wantErr: "cancellers are empty",
		},
		{
			name: "empty bypassers",
			mutate: func(_ *testing.T, in *initializeTimelockInput) {
				in.Bypassers = nil
			},
			wantErr: "bypassers are empty",
		},
		{
			name: "empty proposer address",
			mutate: func(_ *testing.T, in *initializeTimelockInput) {
				in.Proposers = []string{""}
			},
			wantErr: "proposers[0] is empty",
		},
		{
			name: "empty canceller address",
			mutate: func(_ *testing.T, in *initializeTimelockInput) {
				in.Cancellers = []string{""}
			},
			wantErr: "cancellers[0] is empty",
		},
		{
			name: "empty bypasser address",
			mutate: func(_ *testing.T, in *initializeTimelockInput) {
				in.Bypassers = []string{""}
			},
			wantErr: "bypassers[0] is empty",
		},
		{
			name: "duplicate proposer",
			mutate: func(t *testing.T, in *initializeTimelockInput) {
				t.Helper()
				address := testInitContractID(t, 90)
				in.Proposers = []string{address, address}
			},
			wantErr: "proposers contains duplicate address",
		},
		{
			name: "duplicate canceller",
			mutate: func(t *testing.T, in *initializeTimelockInput) {
				t.Helper()
				address := testInitContractID(t, 91)
				in.Cancellers = []string{address, address}
			},
			wantErr: "cancellers contains duplicate address",
		},
		{
			name: "duplicate bypasser",
			mutate: func(t *testing.T, in *initializeTimelockInput) {
				t.Helper()
				address := testInitContractID(t, 92)
				in.Bypassers = []string{address, address}
			},
			wantErr: "bypassers contains duplicate address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dependency := stellarmocks.NewInvoker(t)

			input := initializeTimelockInput{
				ContractID: testInitContractID(t, 10),
				MinDelay:   123,
				Proposers:  []string{testInitContractID(t, 20)},
				Cancellers: []string{testInitContractID(t, 30)},
				Bypassers:  []string{testInitContractID(t, 40)},
			}
			tt.mutate(t, &input)

			err := initializeTimelock(t.Context(), dependency, input)

			require.ErrorContains(t, err, tt.wantErr)
			dependency.AssertNotCalled(t, "InvokeContract", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
