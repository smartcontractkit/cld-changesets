package familyregistry

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

type testSeq struct{ id string }

type testChain struct {
	id int
}

func testRegistration(family string) Registration[testSeq, testChain] {
	seq := testSeq{id: family}
	return Registration[testSeq, testChain]{
		Family:   family,
		Sequence: &seq,
	}
}

func TestRegister_validationPanics(t *testing.T) {
	t.Parallel()

	reg := New[testSeq, testChain]("test-registry")

	tests := []struct {
		name string
		reg  Registration[testSeq, testChain]
	}{
		{
			name: "empty family",
			reg:  Registration[testSeq, testChain]{Family: "", Sequence: &testSeq{}},
		},
		{
			name: "nil sequence",
			reg:  Registration[testSeq, testChain]{Family: "evm", Sequence: nil},
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

	reg := New[testSeq, testChain]("test-registry")
	reg.Register(testRegistration("evm"))

	require.Panics(t, func() {
		reg.Register(testRegistration("evm"))
	})
}

func TestRegisteredFamilies_sorted(t *testing.T) {
	t.Parallel()

	reg := New[testSeq, testChain]("test-registry")
	reg.Register(testRegistration("solana"))
	reg.Register(testRegistration("evm"))
	reg.Register(testRegistration("aptos"))

	require.Equal(t, []string{"aptos", "evm", "solana"}, reg.RegisteredFamilies())
}

func TestGet_errors(t *testing.T) {
	t.Parallel()

	t.Run("none registered", func(t *testing.T) {
		t.Parallel()

		reg := New[testSeq, testChain]("test-registry")
		_, err := reg.Get("evm")
		require.ErrorContains(t, err, `test-registry: no sequence registered for family "evm" (none registered)`)
	})

	t.Run("other families registered", func(t *testing.T) {
		t.Parallel()

		reg := New[testSeq, testChain]("test-registry")
		reg.Register(testRegistration("evm"))
		reg.Register(testRegistration("solana"))

		_, err := reg.Get("aptos")
		require.ErrorContains(t, err, `test-registry: no sequence registered for family "aptos" (registered: evm, solana)`)
	})
}

func TestSequenceForFamily(t *testing.T) {
	t.Parallel()

	reg := New[testSeq, testChain]("test-registry")
	reg.Register(testRegistration("evm"))

	seq, err := reg.SequenceForFamily("evm")
	require.NoError(t, err)
	require.NotNil(t, seq)
	require.Equal(t, "evm", seq.id)
}

func TestSequenceForChainSelector_invalid(t *testing.T) {
	t.Parallel()

	reg := New[testSeq, testChain]("test-registry")
	_, err := reg.SequenceForChainSelector(0)
	require.Error(t, err)
}

func TestVerifyForFamily(t *testing.T) {
	t.Parallel()

	reg := New[testSeq, testChain]("test-registry")

	t.Run("missing family", func(t *testing.T) {
		t.Parallel()

		err := reg.VerifyForFamily("evm", cldf.Environment{}, nil)
		require.ErrorContains(t, err, "no sequence registered for family")
	})

	t.Run("nil verify hook", func(t *testing.T) {
		t.Parallel()

		local := New[testSeq, testChain]("test-registry")
		local.Register(testRegistration("evm"))

		err := local.VerifyForFamily("evm", cldf.Environment{}, nil)
		require.NoError(t, err)
	})

	t.Run("verify hook error wrapped", func(t *testing.T) {
		t.Parallel()

		const family = "evm"
		local := New[testSeq, testChain]("test-registry")
		local.Register(Registration[testSeq, testChain]{
			Family:   family,
			Sequence: &testSeq{id: family},
			Verify: func(_ cldf.Environment, _ []testChain) error {
				return errors.New("boom")
			},
		})

		err := local.VerifyForFamily(family, cldf.Environment{}, []testChain{{id: 1}})
		require.ErrorContains(t, err, "boom")
		require.ErrorContains(t, err, fmt.Sprintf("family %s:", family))
	})
}
