package deploycustomtopology

import (
	"errors"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-deployments-framework/changeset/sequenceutils"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"

	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

func testSequence(id string) *Sequence {
	return operations.NewSequence(
		id,
		semver.MustParse("1.0.0"),
		"test sequence",
		func(_ operations.Bundle, _ Deps, _ ChainInput) (sequenceutils.OnChainOutput, error) {
			return sequenceutils.OnChainOutput{}, nil
		},
	)
}

func newTestRegistry() *familyregistry.Registry[Sequence, ChainInput] {
	return familyregistry.New[Sequence, ChainInput]("test deploy-custom-topology")
}

func TestSequenceForChainSelector(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()

	tests := []struct {
		name          string
		chainSelector uint64
	}{
		{name: "invalid selector", chainSelector: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := reg.SequenceForChainSelector(tt.chainSelector)
			require.Error(t, err)
		})
	}
}

func TestRegister_andLookup(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	const family = "test-deploy-custom-topology-family-a"

	reg.Register(Registration{
		Family:   family,
		Sequence: testSequence("test-topology-seq-a"),
		Verify: func(_ cldf.Environment, chains []ChainInput) error {
			if len(chains) == 0 {
				return errors.New("no chains")
			}

			return nil
		},
	})

	require.Contains(t, reg.RegisteredFamilies(), family)

	seq, err := reg.SequenceForFamily(family)
	require.NoError(t, err)
	require.NotNil(t, seq)

	tests := []struct {
		name    string
		chains  []ChainInput
		wantErr string
	}{
		{name: "verify success", chains: []ChainInput{{ChainSelector: 1}}},
		{name: "verify hook error", chains: nil, wantErr: "family test-deploy-custom-topology-family-a: no chains"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := reg.VerifyForFamily(family, cldf.Environment{}, tt.chains)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestRegister_validationPanics(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()

	tests := []struct {
		name string
		reg  Registration
	}{
		{name: "empty family", reg: Registration{Family: "", Sequence: testSequence("empty-family")}},
		{name: "nil sequence", reg: Registration{Family: "test-deploy-custom-topology-family-b", Sequence: nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Panics(t, func() { reg.Register(tt.reg) })
		})
	}
}

func TestRegister_duplicatePanics(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	const family = "test-deploy-custom-topology-family-c"
	reg.Register(Registration{Family: family, Sequence: testSequence("test-topology-seq-c")})

	require.Panics(t, func() {
		reg.Register(Registration{Family: family, Sequence: testSequence("test-topology-seq-c-dup")})
	})
}

func TestSequenceForFamily_errors(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()

	tests := []struct {
		name    string
		family  string
		wantErr string
	}{
		{
			name:    "missing family",
			family:  "test-deploy-custom-topology-family-missing",
			wantErr: `test deploy-custom-topology: no sequence registered for family "test-deploy-custom-topology-family-missing" (none registered)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := reg.SequenceForFamily(tt.family)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestVerifyForFamily(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()

	const nilHookFamily = "test-deploy-custom-topology-family-d"
	reg.Register(Registration{Family: nilHookFamily, Sequence: testSequence("test-topology-seq-d")})

	const wrapFamily = "test-deploy-custom-topology-family-e"
	reg.Register(Registration{
		Family:   wrapFamily,
		Sequence: testSequence("test-topology-seq-e"),
		Verify: func(_ cldf.Environment, _ []ChainInput) error {
			return errors.New("boom")
		},
	})

	tests := []struct {
		name    string
		family  string
		chains  []ChainInput
		wantErr string
	}{
		{
			name:    "missing family",
			family:  "test-deploy-custom-topology-family-missing-verify",
			wantErr: `test deploy-custom-topology: no sequence registered for family "test-deploy-custom-topology-family-missing-verify" (registered: test-deploy-custom-topology-family-d, test-deploy-custom-topology-family-e)`,
		},
		{name: "nil verify hook", family: nilHookFamily},
		{
			name:    "wrapped verify error",
			family:  wrapFamily,
			chains:  []ChainInput{{ChainSelector: 1}},
			wantErr: "family test-deploy-custom-topology-family-e: boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := reg.VerifyForFamily(tt.family, cldf.Environment{}, tt.chains)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.EqualError(t, err, tt.wantErr)
		})
	}
}
