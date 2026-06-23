package deploycustomtopology

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
)

func testSequence(id string) *Sequence {
	return operations.NewSequence(
		id,
		semver.MustParse("1.0.0"),
		"test sequence",
		func(_ operations.Bundle, _ Deps, _ ChainInput) (ChainOutput, error) {
			return ChainOutput{}, nil
		},
	)
}

func TestSequenceForChainSelector(t *testing.T) {
	t.Parallel()

	_, err := Sequences.SequenceForChainSelector(0)
	require.Error(t, err)
}

func TestRegister_andLookup(t *testing.T) {
	t.Parallel()

	const family = "test-topology-family-a"

	Sequences.Register(Registration{
		Family:   family,
		Sequence: testSequence("test-topology-seq-a"),
		Verify: func(_ cldf.Environment, chains []ChainInput) error {
			if len(chains) == 0 {
				return errors.New("no chains")
			}

			return nil
		},
	})

	require.Contains(t, Sequences.RegisteredFamilies(), family)

	seq, err := Sequences.SequenceForFamily(family)
	require.NoError(t, err)
	require.NotNil(t, seq)

	tests := []struct {
		name    string
		chains  []ChainInput
		wantErr bool
	}{
		{name: "verify success", chains: []ChainInput{{ChainSelector: 1}}},
		{name: "verify hook error", chains: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Sequences.VerifyForFamily(family, cldf.Environment{}, tt.chains)
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

	tests := []struct {
		name string
		reg  Registration
	}{
		{name: "empty family", reg: Registration{Family: "", Sequence: testSequence("empty-family")}},
		{name: "nil sequence", reg: Registration{Family: "test-topology-family-b", Sequence: nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Panics(t, func() { Sequences.Register(tt.reg) })
		})
	}
}

func TestRegister_duplicatePanics(t *testing.T) {
	t.Parallel()

	const family = "test-topology-family-c"
	Sequences.Register(Registration{Family: family, Sequence: testSequence("test-topology-seq-c")})

	require.Panics(t, func() {
		Sequences.Register(Registration{Family: family, Sequence: testSequence("test-topology-seq-c-dup")})
	})
}

func TestSequenceForFamily_errors(t *testing.T) {
	t.Parallel()

	const family = "test-topology-family-missing"
	_, err := Sequences.SequenceForFamily(family)
	require.ErrorContains(t, err, fmt.Sprintf(`no sequence registered for family %q`, family))
}

func TestVerifyForFamily(t *testing.T) {
	t.Parallel()

	const nilHookFamily = "test-topology-family-d"
	Sequences.Register(Registration{Family: nilHookFamily, Sequence: testSequence("test-topology-seq-d")})

	const wrapFamily = "test-topology-family-e"
	Sequences.Register(Registration{
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
		{name: "missing family", family: "test-topology-family-missing-verify", wantErr: "no sequence registered for family"},
		{name: "nil verify hook", family: nilHookFamily},
		{name: "wrapped verify error", family: wrapFamily, chains: []ChainInput{{ChainSelector: 1}}, wantErr: "boom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Sequences.VerifyForFamily(tt.family, cldf.Environment{}, tt.chains)
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
