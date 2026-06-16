package refkey

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
)

func testVersion(t *testing.T) *semver.Version {
	t.Helper()

	return semver.MustParse("1.0.0")
}

func testAddressRef(t *testing.T) cldfdatastore.AddressRef {
	t.Helper()

	version := testVersion(t)

	return cldfdatastore.AddressRef{
		Address:       "0xabc",
		ChainSelector: 90000001,
		Type:          cldfdatastore.ContractType("TestContract"),
		Version:       version,
		Qualifier:     "q1",
	}
}

func TestNewAndFromAddressRef(t *testing.T) {
	t.Parallel()

	ref := testAddressRef(t)

	tests := []struct {
		name string
		got  RefKey
		want RefKey
	}{
		{
			name: "New",
			got: New(
				ref.ChainSelector,
				ref.Type,
				ref.Version,
				ref.Qualifier,
			),
			want: RefKey{
				ChainSelector: ref.ChainSelector,
				Type:          ref.Type,
				Version:       ref.Version,
				Qualifier:     ref.Qualifier,
			},
		},
		{
			name: "FromAddressRef",
			got:  FromAddressRef(ref),
			want: RefKey{
				ChainSelector: ref.ChainSelector,
				Type:          ref.Type,
				Version:       ref.Version,
				Qualifier:     ref.Qualifier,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.got)
		})
	}
}

func TestRefKey_Key(t *testing.T) {
	t.Parallel()

	ref := testAddressRef(t)

	tests := []struct {
		name    string
		key     RefKey
		wantErr error
	}{
		{
			name: "success",
			key:  FromAddressRef(ref),
		},
		{
			name:    "missing version",
			key:     RefKey{ChainSelector: ref.ChainSelector, Type: ref.Type},
			wantErr: cldfdatastore.ErrAddressRefVersionRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.key.Key()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
		})
	}
}

func TestRefKey_Resolve(t *testing.T) {
	t.Parallel()

	ref := testAddressRef(t)
	ds := cldfdatastore.NewMemoryDataStore()
	require.NoError(t, ds.Addresses().Add(ref))
	sealed := ds.Seal()

	tests := []struct {
		name    string
		key     RefKey
		env     cldf.Environment
		want    string
		wantErr string
	}{
		{
			name:    "missing datastore",
			key:     FromAddressRef(ref),
			env:     cldf.Environment{},
			wantErr: "missing datastore in environment",
		},
		{
			name: "success",
			key:  FromAddressRef(ref),
			env:  cldf.Environment{DataStore: sealed},
			want: ref.Address,
		},
		{
			name: "missing ref",
			key: RefKey{
				ChainSelector: ref.ChainSelector,
				Type:          cldfdatastore.ContractType("Missing"),
				Version:       ref.Version,
			},
			env:     cldf.Environment{DataStore: sealed},
			wantErr: "address ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.key.Resolve(tt.env)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got.Address)
		})
	}
}
