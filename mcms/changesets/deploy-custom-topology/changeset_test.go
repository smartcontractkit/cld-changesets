package deploycustomtopology

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	chain_selectors "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/chain"
	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

var fakeEVMFamilySelector = chain_selectors.TEST_90000001.Selector

// fakeEVMTopologySeq emulates an EVM family sequence for unit tests: it records a
// deployed address and, when a timelock requests ownership transfers, emits a
// proposal group keyed by that timelock's qualifier.
var fakeEVMTopologySeq = operations.NewSequence(
	"fake-evm-deploy-topology",
	semver.MustParse("1.0.0"),
	"fake EVM deploy topology",
	func(_ operations.Bundle, _ Deps, in ChainInput) (sequenceutils.OnChainOutput, error) {
		v := semver.MustParse("1.0.0")
		out := sequenceutils.OnChainOutput{
			Metadata: cldfdatastore.MetadataBundle{
				Addresses: []cldfdatastore.AddressRef{{
					ChainSelector: in.ChainSelector,
					Address:       "0x0000000000000000000000000000000000000001",
					Type:          cldfdatastore.ContractType("FakeMCM"),
					Version:       v,
				}},
			},
		}

		for _, tl := range in.Config.Timelocks {
			if len(tl.TransferOwnership) == 0 {
				continue
			}
			out.BatchOps = append(out.BatchOps, mcmstypes.BatchOperation{
				ChainSelector: mcmstypes.ChainSelector(in.ChainSelector),
				Transactions: []mcmstypes.Transaction{{
					To:               "0x0000000000000000000000000000000000000002",
					Data:             []byte{0x01},
					AdditionalFields: json.RawMessage(`{}`),
				}},
			})
		}

		return out, nil
	},
)

func init() {
	Sequences.Register(Registration{Family: chain_selectors.FamilyEVM, Sequence: fakeEVMTopologySeq})
}

func testEnv(t *testing.T) cldf.Environment {
	t.Helper()

	return cldf.Environment{
		Logger:           logger.Test(t),
		OperationsBundle: operations.NewBundle(t.Context, logger.Test(t), operations.NewMemoryReporter()),
		BlockChains:      chain.NewBlockChains(nil),
	}
}

func addr(t *testing.T, hex string) *common.Address {
	t.Helper()
	a := common.HexToAddress(hex)

	return &a
}

func validChainConfig() ChainTopologyConfig {
	return ChainTopologyConfig{
		MCMs: []MCMSpec{{Ref: "proposer", Qualifier: "CCIP", ContractType: "ProposerManyChainMultisig"}},
		Timelocks: []TimelockSpec{{
			Ref:       "timelock",
			MinDelay:  big.NewInt(0),
			Qualifier: "CCIP",
			Roles:     RoleAssignments{Proposers: []RoleHolder{{MCMRef: "proposer"}}},
		}},
	}
}

func newMCMSInput() *cldf.MCMSTimelockProposalInput {
	return &cldf.MCMSTimelockProposalInput{
		TimelockAction: mcmstypes.TimelockActionSchedule,
		ValidUntil:     uint32(time.Now().Add(2 * time.Hour).UTC().Unix()), //nolint:gosec // test timestamp
		TimelockDelay:  mcmstypes.NewDuration(time.Second),
		Description:    "test",
	}
}

func TestChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	withTransfer := validChainConfig()
	withTransfer.Timelocks[0].TransferOwnership = []RoleHolder{{Address: addr(t, "0xabc")}}

	dupMCM := validChainConfig()
	dupMCM.MCMs = append(dupMCM.MCMs, MCMSpec{Ref: "proposer", Qualifier: "CCIP"})

	dupTimelock := validChainConfig()
	dupTimelock.Timelocks = append(dupTimelock.Timelocks, TimelockSpec{
		Ref:       "timelock",
		MinDelay:  big.NewInt(0),
		Qualifier: "RMN",
	})

	mcmTimelockRefCollision := validChainConfig()
	mcmTimelockRefCollision.Timelocks[0].Ref = "proposer"

	undeclaredRef := validChainConfig()
	undeclaredRef.Timelocks[0].Roles.Proposers = []RoleHolder{{MCMRef: "missing"}}

	bothRefAndAddr := validChainConfig()
	bothRefAndAddr.Timelocks[0].Roles.Proposers = []RoleHolder{{MCMRef: "proposer", Address: addr(t, "0xabc")}}

	neitherRefNorAddr := validChainConfig()
	neitherRefNorAddr.Timelocks[0].Roles.Proposers = []RoleHolder{{}}

	noQualifier := validChainConfig()
	noQualifier.Timelocks[0].Qualifier = ""

	tests := []struct {
		name    string
		input   Input
		wantErr string
	}{
		{
			name:    "no chain configs",
			input:   Input{Cfg: Config{}},
			wantErr: "no chain configs",
		},
		{
			name:    "empty chain",
			input:   Input{Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: {}}}},
			wantErr: "no MCMs or timelocks",
		},
		{
			name:    "unknown chain selector family",
			input:   Input{Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{1: validChainConfig()}}},
			wantErr: "chain selector 1",
		},
		{
			name:    "duplicate MCM ref",
			input:   Input{Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: dupMCM}}},
			wantErr: "duplicate MCM ref",
		},
		{
			name:    "duplicate timelock ref",
			input:   Input{Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: dupTimelock}}},
			wantErr: "duplicate timelock ref",
		},
		{
			name:    "timelock ref conflicts with MCM ref",
			input:   Input{Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: mcmTimelockRefCollision}}},
			wantErr: "conflicts with an MCM ref",
		},
		{
			name:    "undeclared mcmRef",
			input:   Input{Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: undeclaredRef}}},
			wantErr: "not declared",
		},
		{
			name:    "role holder with both ref and address",
			input:   Input{Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: bothRefAndAddr}}},
			wantErr: "exactly one of mcmRef or address",
		},
		{
			name:    "role holder with neither ref nor address",
			input:   Input{Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: neitherRefNorAddr}}},
			wantErr: "exactly one of mcmRef or address",
		},
		{
			name:    "timelock missing qualifier",
			input:   Input{Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: noQualifier}}},
			wantErr: "qualifier is required",
		},
		{
			name:    "transfer ownership without MCMS",
			input:   Input{Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: withTransfer}}},
			wantErr: "requires an MCMS input",
		},
		{
			name: "valid - no MCMS",
			input: Input{
				Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: validChainConfig()}},
			},
		},
		{
			name: "valid - with MCMS and transfer",
			input: Input{
				Cfg:  Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: withTransfer}},
				MCMS: newMCMSInput(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Changeset{}.VerifyPreconditions(testEnv(t), tt.input)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestChangeset_VerifyPreconditions_invalidMCMS(t *testing.T) {
	t.Parallel()

	bad := newMCMSInput()
	bad.TimelockAction = "not-a-real-action"

	err := Changeset{}.VerifyPreconditions(testEnv(t), Input{
		Cfg:  Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: validChainConfig()}},
		MCMS: bad,
	})
	require.ErrorContains(t, err, "invalid MCMS timelock proposal input")
}

func TestChangeset_Apply_deploysAndWritesAddresses(t *testing.T) {
	t.Parallel()

	out, err := Changeset{}.Apply(testEnv(t), Input{
		Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: validChainConfig()}},
	})
	require.NoError(t, err)
	require.Empty(t, out.MCMSTimelockProposals)

	addrs, err := out.DataStore.Addresses().Fetch()
	require.NoError(t, err)
	require.Len(t, addrs, 1)
	require.Equal(t, fakeEVMFamilySelector, addrs[0].ChainSelector)
}

func TestChangeset_Apply_proposalGroupsWithoutMCMSErrors(t *testing.T) {
	t.Parallel()

	withTransfer := validChainConfig()
	withTransfer.Timelocks[0].TransferOwnership = []RoleHolder{{Address: addr(t, "0xabc")}}

	_, err := Changeset{}.Apply(testEnv(t), Input{
		Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{fakeEVMFamilySelector: withTransfer}},
	})
	require.ErrorContains(t, err, "no MCMS input was provided")
}

func TestChangeset_Apply_unregisteredFamilyErrors(t *testing.T) {
	t.Parallel()

	// chain selector 1 has no resolvable family -> sequence lookup fails.
	_, err := Changeset{}.Apply(testEnv(t), Input{
		Cfg: Config{ChainConfigs: map[uint64]ChainTopologyConfig{1: validChainConfig()}},
	})
	require.ErrorContains(t, err, "chain selector 1")
}

func TestBatchOpsForQualifier(t *testing.T) {
	t.Parallel()

	op := func(sel uint64, to string) mcmstypes.BatchOperation {
		return mcmstypes.BatchOperation{
			ChainSelector: mcmstypes.ChainSelector(sel),
			Transactions:  []mcmstypes.Transaction{{To: to}},
		}
	}
	cfg := Config{ChainConfigs: map[uint64]ChainTopologyConfig{
		1: {
			MCMs: []MCMSpec{{
				Ref:          "bypasser",
				Qualifier:    "CCIP",
				ContractType: "BypasserManyChainMultisig",
			}},
			Timelocks: []TimelockSpec{{
				Ref:               "timelock",
				Qualifier:         "CCIP",
				TransferOwnership: []RoleHolder{{MCMRef: "bypasser"}},
			}},
		},
		2: {
			MCMs: []MCMSpec{{
				Ref:          "bypasser",
				Qualifier:    "RMN",
				ContractType: "BypasserManyChainMultisig",
			}},
			Timelocks: []TimelockSpec{{
				Ref:               "timelock",
				Qualifier:         "RMN",
				TransferOwnership: []RoleHolder{{MCMRef: "bypasser"}},
			}},
		},
	}}
	meta := cldfdatastore.MetadataBundle{
		Addresses: []cldfdatastore.AddressRef{
			{ChainSelector: 1, Qualifier: "CCIP", Type: "BypasserManyChainMultisig", Address: "0xccip"},
			{ChainSelector: 2, Qualifier: "RMN", Type: "BypasserManyChainMultisig", Address: "0xrmn"},
		},
	}
	batchOps := []mcmstypes.BatchOperation{op(1, "0xccip"), op(2, "0xrmn")}

	require.Len(t, batchOpsForQualifier(batchOps, meta, cfg, "CCIP"), 1)
	require.Len(t, batchOpsForQualifier(batchOps, meta, cfg, "RMN"), 1)
	require.Empty(t, batchOpsForQualifier(batchOps, meta, cfg, "NONE"))
}

func TestSortedQualifiersWithTransfers(t *testing.T) {
	t.Parallel()

	cfg := Config{ChainConfigs: map[uint64]ChainTopologyConfig{
		1: {Timelocks: []TimelockSpec{
			{Qualifier: "RMN", TransferOwnership: []RoleHolder{{Address: addr(t, "0x1")}}},
			{Qualifier: "CCIP", TransferOwnership: []RoleHolder{{Address: addr(t, "0x2")}}},
			{Qualifier: "EMPTY"},
		}},
	}}

	require.Equal(t, []string{"CCIP", "RMN"}, sortedQualifiersWithTransfers(cfg))
}
