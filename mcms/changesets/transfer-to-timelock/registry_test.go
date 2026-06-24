package transfertotimelock

import (
	"errors"
	"fmt"
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
	return familyregistry.New[Sequence, ChainInput]("test transfer-to-timelock")
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
	const family = "test-transfer-to-timelock-family-a"

	reg.Register(Registration{
		Family:   family,
		Sequence: testSequence("test-seq-a"),
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
		wantErr bool
	}{
		{
			name:   "verify success",
			chains: []ChainInput{{ChainSelector: 1}},
		},
		{
			name:    "verify hook error",
			chains:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := reg.VerifyForFamily(family, cldf.Environment{}, tt.chains)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
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
		{
			name: "empty family",
			reg:  Registration{Family: "", Sequence: testSequence("empty-family")},
		},
		{
			name: "nil sequence",
			reg:  Registration{Family: "test-transfer-to-timelock-family-b", Sequence: nil},
		},
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
	const family = "test-transfer-to-timelock-family-c"
	reg.Register(Registration{Family: family, Sequence: testSequence("test-seq-c")})

	require.Panics(t, func() {
		reg.Register(Registration{Family: family, Sequence: testSequence("test-seq-c-dup")})
	})
}

func TestSequenceForFamily_errors(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()

	tests := []struct {
		name   string
		family string
	}{
		{name: "missing family", family: "test-transfer-to-timelock-family-missing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := reg.SequenceForFamily(tt.family)
			require.ErrorContains(t, err, fmt.Sprintf(`no sequence registered for family %q`, tt.family))
		})
	}
}

func TestVerifyForFamily(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()

	const nilHookFamily = "test-transfer-to-timelock-family-d"
	reg.Register(Registration{Family: nilHookFamily, Sequence: testSequence("test-seq-d")})

	const wrapFamily = "test-transfer-to-timelock-family-e"
	reg.Register(Registration{
		Family:   wrapFamily,
		Sequence: testSequence("test-seq-e"),
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
			family:  "test-transfer-to-timelock-family-missing-verify",
			wantErr: "no sequence registered for family",
		},
		{
			name:   "nil verify hook",
			family: nilHookFamily,
		},
		{
			name:    "wrapped verify error",
			family:  wrapFamily,
			chains:  []ChainInput{{ChainSelector: 1}},
			wantErr: "boom",
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

			require.ErrorContains(t, err, tt.wantErr)
			if tt.family == wrapFamily {
				require.ErrorContains(t, err, fmt.Sprintf("family %s:", wrapFamily))
			}
		})
	}
}
