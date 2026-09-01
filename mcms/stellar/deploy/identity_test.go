package stellardeploy

import (
	"errors"
	"testing"

	protocolrpc "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	cldftesthelpers "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/mcms/proposalutils/testhelpers"

	stellardeployment "github.com/smartcontractkit/chainlink-stellar/deployment"

	stellarcre "github.com/smartcontractkit/chainlink-stellar/deployment/cre"
	stellarmocks "github.com/smartcontractkit/mcms/sdk/stellar/mocks"

	stellartestutils "github.com/smartcontractkit/cld-changesets/mcms/stellar/internal"
)

const testNetworkPassphrase = "Standalone Network ; February 2017"

// ledgerReaderMock returns a mockery RpcClient (from the mcms SDK's generated
// mocks) whose GetLedgerEntries answers with the given response and error.
// Maybe() keeps validation-only table rows, which never reach the ledger,
// from failing the mock's expectation check.
func ledgerReaderMock(
	t *testing.T,
	response protocolrpc.GetLedgerEntriesResponse,
	err error,
) *stellarmocks.RpcClient {
	t.Helper()

	m := stellarmocks.NewRpcClient(t)
	m.EXPECT().
		GetLedgerEntries(mock.Anything, mock.Anything).
		Return(response, err).
		Maybe()

	return m
}

func TestDeriveDeploymentSalt(t *testing.T) {
	t.Parallel()

	contractType := mcmscontracts.ProposerManyChainMultisig

	first := deriveDeploymentSalt(contractType, "", 0)

	t.Run("stable", func(t *testing.T) {
		t.Parallel()

		require.Equal(
			t,
			first,
			deriveDeploymentSalt(contractType, "", 0),
		)
	})

	t.Run("attempt changes salt", func(t *testing.T) {
		t.Parallel()

		require.NotEqual(
			t,
			first,
			deriveDeploymentSalt(contractType, "", 1),
		)
	})

	t.Run("qualifier changes salt", func(t *testing.T) {
		t.Parallel()

		require.NotEqual(
			t,
			first,
			deriveDeploymentSalt(contractType, "qualified", 0),
		)
	})

	t.Run("contract type changes salt", func(t *testing.T) {
		t.Parallel()

		require.NotEqual(
			t,
			first,
			deriveDeploymentSalt(
				mcmscontracts.CancellerManyChainMultisig,
				"",
				0,
			),
		)
	})
}

func TestComputeContractID(t *testing.T) {
	t.Parallel()

	deployerAddress := testStellarAccountAddress(t)

	salt := deriveDeploymentSalt(
		mcmscontracts.ProposerManyChainMultisig,
		"",
		0,
	)

	first, err := computeContractID(
		testNetworkPassphrase,
		deployerAddress,
		salt,
	)
	require.NoError(t, err)

	second, err := computeContractID(
		testNetworkPassphrase,
		deployerAddress,
		salt,
	)
	require.NoError(t, err)

	require.Equal(t, first, second)

	raw, err := strkey.Decode(strkey.VersionByteContract, first)
	require.NoError(t, err)
	require.Len(t, raw, 32)

	other, err := computeContractID(
		testNetworkPassphrase,
		deployerAddress,
		deriveDeploymentSalt(
			mcmscontracts.ProposerManyChainMultisig,
			"",
			1,
		),
	)
	require.NoError(t, err)
	require.NotEqual(t, first, other)
}

func TestComputeContractID_Validation(t *testing.T) {
	t.Parallel()

	deployerAddress := testStellarAccountAddress(t)
	salt := [32]byte{1}

	tests := []struct {
		name     string
		network  string
		deployer string
		wantErr  string
	}{
		{
			name:     "empty network passphrase",
			deployer: deployerAddress,
			wantErr:  "network passphrase is empty",
		},
		{
			name:    "empty deployer address",
			network: testNetworkPassphrase,
			wantErr: "deployer address is empty",
		},
		{
			name:     "invalid deployer address",
			network:  testNetworkPassphrase,
			deployer: "not-an-address",
			wantErr:  "decode deployer address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := computeContractID(
				tt.network,
				tt.deployer,
				salt,
			)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestResolveDeploymentIdentity_Validation(t *testing.T) {
	t.Parallel()

	validReader := ledgerReaderMock(t, protocolrpc.GetLedgerEntriesResponse{}, nil)
	validAddress := testStellarAccountAddress(t)
	validType := mcmscontracts.ProposerManyChainMultisig
	validWASM := []byte{1}

	tests := []struct {
		name         string
		reader       ledgerEntryReader
		network      string
		deployer     string
		contractType cldf.ContractType
		wasm         []byte
		wantErr      string
	}{
		{
			name:         "nil reader",
			network:      testNetworkPassphrase,
			deployer:     validAddress,
			contractType: validType,
			wasm:         validWASM,
			wantErr:      "ledger reader is nil",
		},
		{
			name:         "empty network passphrase",
			reader:       validReader,
			deployer:     validAddress,
			contractType: validType,
			wasm:         validWASM,
			wantErr:      "network passphrase is empty",
		},
		{
			name:         "empty deployer address",
			reader:       validReader,
			network:      testNetworkPassphrase,
			contractType: validType,
			wasm:         validWASM,
			wantErr:      "deployer address is empty",
		},
		{
			name:     "empty contract type",
			reader:   validReader,
			network:  testNetworkPassphrase,
			deployer: validAddress,
			wasm:     validWASM,
			wantErr:  "contract type is empty",
		},
		{
			name:         "empty wasm",
			reader:       validReader,
			network:      testNetworkPassphrase,
			deployer:     validAddress,
			contractType: validType,
			wantErr:      "WASM is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := resolveDeploymentIdentity(
				t.Context(),
				tt.reader,
				tt.network,
				tt.deployer,
				tt.contractType,
				"",
				tt.wasm,
			)

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestResolveDeploymentIdentity_FreeCandidate(t *testing.T) {
	t.Parallel()

	deployerAddress := testStellarAccountAddress(t)
	contractType := mcmscontracts.ProposerManyChainMultisig
	qualifier := "qualified"
	wasm := []byte{1, 2, 3}

	got, err := resolveDeploymentIdentity(
		t.Context(),
		ledgerReaderMock(t, protocolrpc.GetLedgerEntriesResponse{}, nil),
		testNetworkPassphrase,
		deployerAddress,
		contractType,
		qualifier,
		wasm,
	)
	require.NoError(t, err)

	wantSalt := deriveDeploymentSalt(
		contractType,
		qualifier,
		0,
	)
	wantContractID, err := computeContractID(
		testNetworkPassphrase,
		deployerAddress,
		wantSalt,
	)
	require.NoError(t, err)

	require.False(t, got.Existing)
	require.Equal(t, uint32(0), got.Attempt)
	require.Equal(t, wantSalt, got.Salt)
	require.Equal(t, wantContractID, got.ContractID)
}

func TestResolveDeploymentIdentity_LedgerError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("ledger unavailable")

	_, err := resolveDeploymentIdentity(
		t.Context(),
		ledgerReaderMock(t, protocolrpc.GetLedgerEntriesResponse{}, sentinel),
		testNetworkPassphrase,
		testStellarAccountAddress(t),
		mcmscontracts.ProposerManyChainMultisig,
		"",
		[]byte{1},
	)

	require.Error(t, err)
	require.ErrorIs(t, err, sentinel)
	require.ErrorContains(t, err, "resolve Stellar deployment identity")
}

func TestResolveDeploymentIdentity_AdoptsExistingAndAdvancesOnCollision(
	t *testing.T,
) {
	selector := chainselectors.STELLAR_LOCALNET.Selector
	rt := stellartestutils.NewStellarRuntime(t, selector)

	qualifier := "partial-recovery"

	cfg := cldftesthelpers.SingleGroupTimelockConfig(t)
	cfg.Qualifier = &qualifier

	stellartestutils.DeployMCMSWithTimelock(
		t,
		rt,
		selector,
		cfg,
	)

	proposer := stellartestutils.ResolveContract(
		t,
		rt.Environment(),
		selector,
		mcmscontracts.ProposerManyChainMultisig,
		qualifier,
	)

	chain, ok := rt.Environment().BlockChains.StellarChains()[selector]
	require.True(t, ok)

	deployer, err := stellardeployment.NewDeployerFromChain(chain)
	require.NoError(t, err)

	wasm, err := stellarcre.Artifact(stellarcre.MCMSWasm)
	require.NoError(t, err)

	t.Run("adopts matching deterministic deployment", func(t *testing.T) {
		identity, err := resolveDeploymentIdentity(
			t.Context(),
			chain.Client,
			chain.NetworkPassphrase,
			deployer.SignerAddress(),
			mcmscontracts.ProposerManyChainMultisig,
			qualifier,
			wasm,
		)
		require.NoError(t, err)

		require.True(t, identity.Existing)
		require.Equal(t, uint32(0), identity.Attempt)
		require.Equal(t, proposer, identity.ContractID)
		require.Equal(
			t,
			deriveDeploymentSalt(
				mcmscontracts.ProposerManyChainMultisig,
				qualifier,
				0,
			),
			identity.Salt,
		)
	})

	t.Run("different wasm advances to next deterministic candidate", func(t *testing.T) {
		differentWASM := append([]byte(nil), wasm...)
		differentWASM = append(differentWASM, 0)

		identity, err := resolveDeploymentIdentity(
			t.Context(),
			chain.Client,
			chain.NetworkPassphrase,
			deployer.SignerAddress(),
			mcmscontracts.ProposerManyChainMultisig,
			qualifier,
			differentWASM,
		)
		require.NoError(t, err)

		require.False(t, identity.Existing)
		require.Equal(t, uint32(1), identity.Attempt)
		require.NotEqual(t, proposer, identity.ContractID)
		require.Equal(
			t,
			deriveDeploymentSalt(
				mcmscontracts.ProposerManyChainMultisig,
				qualifier,
				1,
			),
			identity.Salt,
		)
	})
}

func TestContractScAddressRoundTrip(t *testing.T) {
	t.Parallel()

	contractID, err := computeContractID(
		testNetworkPassphrase,
		testStellarAccountAddress(t),
		[32]byte{1},
	)
	require.NoError(t, err)

	address, err := contractScAddress(contractID)
	require.NoError(t, err)

	got, err := contractIDFromScAddress(address)
	require.NoError(t, err)

	require.Equal(t, contractID, got)
}

func testStellarAccountAddress(t *testing.T) string {
	t.Helper()

	raw := make([]byte, 32)
	raw[0] = 1

	address, err := strkey.Encode(
		strkey.VersionByteAccountID,
		raw,
	)
	require.NoError(t, err)

	return address
}
