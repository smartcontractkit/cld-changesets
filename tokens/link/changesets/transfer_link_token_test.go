package changesets

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	chain_selectors "github.com/smartcontractkit/chain-selectors"
	cldf_chain "github.com/smartcontractkit/chainlink-deployments-framework/chain"
	cldf_evm "github.com/smartcontractkit/chainlink-deployments-framework/chain/evm"
	"github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	mcmscontracts "github.com/smartcontractkit/chainlink-deployments-framework/engine/cld/contracts/mcms"
	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/environment"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/initial/link_token"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/internal/semvers"
)

func TestTransferLinkTokenRejectsPreconditions(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector

	linkTV := linkTokenTypeAndVersion()
	proposerTV := cldf.NewTypeAndVersion(mcmscontracts.ProposerManyChainMultisig, semvers.V1_0_0)
	timelockTV := cldf.NewTypeAndVersion(mcmscontracts.RBACTimelock, semvers.V1_0_0)

	const (
		linkAddr     = "0x1111111111111111111111111111111111111111"
		proposerAddr = "0x2222222222222222222222222222222222222222"
		timelockAddr = "0x3333333333333333333333333333333333333333"
	)

	fullDS := func(t *testing.T) datastore.DataStore {
		t.Helper()
		ds := datastore.NewMemoryDataStore()
		require.NoError(t, saveAddressRef(ds, selector, linkAddr, linkTV, ""))
		require.NoError(t, saveAddressRef(ds, selector, proposerAddr, proposerTV, ""))
		require.NoError(t, saveAddressRef(ds, selector, timelockAddr, timelockTV, ""))

		return ds.Seal()
	}

	baseEnv := cldf.Environment{
		BlockChains: cldf_chain.NewBlockChainsFromSlice([]cldf_chain.BlockChain{
			cldf_evm.Chain{Selector: selector},
		}),
	}

	validInput := TransferLinkTokenInput{
		ChainSelector: selector,
		To:            common.HexToAddress("0x00000000000000000000000000000000000000Ab"),
		Amount:        big.NewInt(1_000_000),
	}

	tcs := []struct {
		name    string
		env     cldf.Environment
		input   TransferLinkTokenInput
		wantErr string
	}{
		{
			name:    "zero chain selector",
			env:     cldf.Environment{DataStore: fullDS(t)},
			input:   TransferLinkTokenInput{ChainSelector: 0, To: validInput.To, Amount: validInput.Amount},
			wantErr: "chain selector must be non-zero",
		},
		{
			name:    "zero recipient",
			env:     cldf.Environment{DataStore: fullDS(t)},
			input:   TransferLinkTokenInput{ChainSelector: selector, To: common.Address{}, Amount: validInput.Amount},
			wantErr: "recipient address must be non-zero",
		},
		{
			name:    "nil amount",
			env:     cldf.Environment{DataStore: fullDS(t)},
			input:   TransferLinkTokenInput{ChainSelector: selector, To: validInput.To, Amount: nil},
			wantErr: "amount must be positive",
		},
		{
			name:    "nil datastore",
			env:     cldf.Environment{},
			input:   validInput,
			wantErr: "datastore is required",
		},
		{
			name: "missing link token",
			env: func() cldf.Environment {
				ds := datastore.NewMemoryDataStore()
				require.NoError(t, saveAddressRef(ds, selector, proposerAddr, proposerTV, ""))
				require.NoError(t, saveAddressRef(ds, selector, timelockAddr, timelockTV, ""))
				e := baseEnv
				e.DataStore = ds.Seal()

				return e
			}(),
			input:   validInput,
			wantErr: "no LinkToken address found",
		},
		{
			name: "missing proposer",
			env: func() cldf.Environment {
				ds := datastore.NewMemoryDataStore()
				require.NoError(t, saveAddressRef(ds, selector, linkAddr, linkTV, ""))
				require.NoError(t, saveAddressRef(ds, selector, timelockAddr, timelockTV, ""))
				e := baseEnv
				e.DataStore = ds.Seal()

				return e
			}(),
			input:   validInput,
			wantErr: "no ProposerManyChainMultisig address found",
		},
		{
			name: "missing timelock",
			env: func() cldf.Environment {
				ds := datastore.NewMemoryDataStore()
				require.NoError(t, saveAddressRef(ds, selector, linkAddr, linkTV, ""))
				require.NoError(t, saveAddressRef(ds, selector, proposerAddr, proposerTV, ""))
				e := baseEnv
				e.DataStore = ds.Seal()

				return e
			}(),
			input:   validInput,
			wantErr: "no RBACTimelock address found",
		},
		{
			name: "link token exists only under different qualifier",
			env: func() cldf.Environment {
				ds := datastore.NewMemoryDataStore()
				require.NoError(t, saveAddressRef(ds, selector, linkAddr, linkTV, "staging"))
				require.NoError(t, saveAddressRef(ds, selector, proposerAddr, proposerTV, ""))
				require.NoError(t, saveAddressRef(ds, selector, timelockAddr, timelockTV, ""))
				e := baseEnv
				e.DataStore = ds.Seal()

				return e
			}(),
			input:   validInput, // Qualifier defaults to ""
			wantErr: "no LinkToken address found",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := TransferLinkTokenChangeset{}.VerifyPreconditions(tc.env, tc.input)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestTransferLinkTokenBuildsProposal(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector
	env, err := environment.New(t.Context(), environment.WithEVMSimulated(t, []uint64{selector}))
	require.NoError(t, err)

	chain := env.BlockChains.EVMChains()[selector]

	linkAddr, deployTx, _, err := link_token.DeployLinkToken(chain.DeployerKey, chain.Client)
	require.NoError(t, err)
	_, err = chain.Confirm(deployTx)
	require.NoError(t, err)

	proposerTV := cldf.NewTypeAndVersion(mcmscontracts.ProposerManyChainMultisig, semvers.V1_0_0)
	timelockTV := cldf.NewTypeAndVersion(mcmscontracts.RBACTimelock, semvers.V1_0_0)
	const (
		proposerAddr = "0x2222222222222222222222222222222222222222"
		timelockAddr = "0x3333333333333333333333333333333333333333"
	)

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, saveAddressRef(ds, selector, linkAddr.Hex(), linkTokenTypeAndVersion(), ""))
	require.NoError(t, saveAddressRef(ds, selector, proposerAddr, proposerTV, ""))
	require.NoError(t, saveAddressRef(ds, selector, timelockAddr, timelockTV, ""))
	env.DataStore = ds.Seal()

	input := TransferLinkTokenInput{
		ChainSelector: selector,
		To:            common.HexToAddress("0x00000000000000000000000000000000000000Ab"),
		Amount:        big.NewInt(1_000_000),
		TimelockDelay: mcmstypes.Duration{},
	}

	err = TransferLinkTokenChangeset{}.VerifyPreconditions(*env, input)
	require.NoError(t, err)

	output, err := TransferLinkTokenChangeset{}.Apply(*env, input)
	require.NoError(t, err)
	require.Len(t, output.MCMSTimelockProposals, 1)
	require.Len(t, output.MCMSTimelockProposals[0].Operations, 1)
	require.Equal(t, linkAddr.Hex(), output.MCMSTimelockProposals[0].Operations[0].Transactions[0].To)
}

func TestTransferLinkTokenValidUntilIsRespected(t *testing.T) {
	t.Parallel()

	selector := chain_selectors.TEST_90000001.Selector
	env, err := environment.New(t.Context(), environment.WithEVMSimulated(t, []uint64{selector}))
	require.NoError(t, err)

	chain := env.BlockChains.EVMChains()[selector]

	linkAddr, deployTx, _, err := link_token.DeployLinkToken(chain.DeployerKey, chain.Client)
	require.NoError(t, err)
	_, err = chain.Confirm(deployTx)
	require.NoError(t, err)

	proposerTV := cldf.NewTypeAndVersion(mcmscontracts.ProposerManyChainMultisig, semvers.V1_0_0)
	timelockTV := cldf.NewTypeAndVersion(mcmscontracts.RBACTimelock, semvers.V1_0_0)
	const (
		proposerAddr = "0x2222222222222222222222222222222222222222"
		timelockAddr = "0x3333333333333333333333333333333333333333"
	)

	ds := datastore.NewMemoryDataStore()
	require.NoError(t, saveAddressRef(ds, selector, linkAddr.Hex(), linkTokenTypeAndVersion(), ""))
	require.NoError(t, saveAddressRef(ds, selector, proposerAddr, proposerTV, ""))
	require.NoError(t, saveAddressRef(ds, selector, timelockAddr, timelockTV, ""))
	env.DataStore = ds.Seal()

	// Use a fixed expiry far enough in the future that the proposal is valid.
	fixedExpiry := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	input := TransferLinkTokenInput{
		ChainSelector: selector,
		To:            common.HexToAddress("0x00000000000000000000000000000000000000Ab"),
		Amount:        big.NewInt(1_000_000),
		ValidUntil:    &fixedExpiry,
	}

	output, err := TransferLinkTokenChangeset{}.Apply(*env, input)
	require.NoError(t, err)
	// #nosec G115 -- fixedExpiry (year 2030) is well within uint32 range
	require.Equal(t, uint32(fixedExpiry.Unix()), output.MCMSTimelockProposals[0].ValidUntil)
}
