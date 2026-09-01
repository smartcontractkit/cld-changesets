package stellardeploy

import (
	"context"
	"strings"
	"testing"

	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

// noInvokeInvoker fails the test if any contract invocation is attempted. It
// backs the validation tests, which must reject bad inputs before touching the
// chain.
type noInvokeInvoker struct {
	t *testing.T
}

func (i noInvokeInvoker) InvokeContract(context.Context, string, string, []xdr.ScVal) (*xdr.ScVal, error) {
	i.t.Fatal("unexpected InvokeContract call")
	return nil, nil
}

func (i noInvokeInvoker) SimulateContract(context.Context, string, string, []xdr.ScVal) (*xdr.ScVal, error) {
	i.t.Fatal("unexpected SimulateContract call")
	return nil, nil
}

func (i noInvokeInvoker) GetEvents(context.Context, string, uint32, []string) ([]protocolrpc.EventInfo, error) {
	i.t.Fatal("unexpected GetEvents call")
	return nil, nil
}

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

			input := valid
			tt.mutate(&input)

			err := initializeMCMS(t.Context(), noInvokeInvoker{t: t}, input)

			require.ErrorContains(t, err, tt.wantErr)
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

			input := initializeTimelockInput{
				ContractID: testInitContractID(t, 10),
				MinDelay:   123,
				Proposers:  []string{testInitContractID(t, 20)},
				Cancellers: []string{testInitContractID(t, 30)},
				Bypassers:  []string{testInitContractID(t, 40)},
			}
			tt.mutate(t, &input)

			err := initializeTimelock(t.Context(), noInvokeInvoker{t: t}, input)

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
