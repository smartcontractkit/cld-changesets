package familyregistry_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/internal/familyregistry"
)

type chainInput struct {
	selector uint64
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	const name = "test changeset"

	reg := familyregistry.New[chainInput, string](name)

	const family = "test-family"

	reg.Register(familyregistry.Registration[chainInput, string]{
		Family:   family,
		Sequence: "seq",
		Verify: func(_ cldf.Environment, chains []chainInput) error {
			if len(chains) == 0 {
				return errors.New("no chains")
			}

			return nil
		},
	})

	require.Contains(t, reg.RegisteredFamilies(), family)

	seq, err := reg.SequenceForFamily(family)
	require.NoError(t, err)
	require.Equal(t, "seq", seq)

	require.NoError(t, reg.VerifyForFamily(family, cldf.Environment{}, []chainInput{{selector: 1}}))

	err = reg.VerifyForFamily(family, cldf.Environment{}, nil)
	require.ErrorContains(t, err, "no chains")
	require.ErrorContains(t, err, fmt.Sprintf("family %s:", family))
}

func TestRegistry_validationPanics(t *testing.T) {
	t.Parallel()

	reg := familyregistry.New[chainInput, *string]("test")

	tests := []struct {
		name string
		reg  familyregistry.Registration[chainInput, *string]
	}{
		{
			name: "empty family",
			reg: familyregistry.Registration[chainInput, *string]{
				Family: "",
				Sequence: func() *string {
					s := "seq"
					return &s
				}(),
			},
		},
		{
			name: "nil sequence",
			reg: familyregistry.Registration[chainInput, *string]{
				Family:   "fam",
				Sequence: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Panics(t, func() { reg.Register(tt.reg) })
		})
	}
}

func TestRegistry_missingFamily(t *testing.T) {
	t.Parallel()

	reg := familyregistry.New[chainInput, string]("test")

	_, err := reg.SequenceForFamily("missing")
	require.ErrorContains(t, err, `no sequence registered for family "missing"`)
	require.ErrorContains(t, err, "import a family package")
}
