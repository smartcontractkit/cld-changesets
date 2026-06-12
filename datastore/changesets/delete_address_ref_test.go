package changesets

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	cldfdatastore "github.com/smartcontractkit/chainlink-deployments-framework/datastore"
	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"
	cldfoperations "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	cldflogger "github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/cld-changesets/datastore/internal/keys"
)

func TestDeleteAddressRefChangeset_VerifyPreconditions(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef1 := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	addressRef2 := cldfdatastore.AddressRef{Address: "0x02", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}

	tests := []struct {
		name    string
		env     cldf.Environment
		input   DeleteAddressRefChangesetInput
		wantErr string
	}{
		{
			name: "success: valid preconditions",
			env: cldf.Environment{
				DataStore: testDataStoreWithAddressRefs(t, addressRef1).Seal(),
			},
			input: DeleteAddressRefChangesetInput{
				AddressRefKeys: []keys.AddressRefKey{addressRefKey(addressRef1), addressRefKey(addressRef2)},
			},
		},
		{
			name: "failure: missing datastore",
			env:  cldf.Environment{},
			input: DeleteAddressRefChangesetInput{
				AddressRefKeys: []keys.AddressRefKey{addressRefKey(addressRef1), addressRefKey(addressRef2)},
			},
			wantErr: "missing datastore in environment",
		},
		{
			name: "failure: no address ref keys given",
			env: cldf.Environment{
				DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
			},
			input:   DeleteAddressRefChangesetInput{AddressRefKeys: []keys.AddressRefKey{}},
			wantErr: "missing address ref keys input",
		},
		{
			name: "failure: address ref entry does not exist",
			env: cldf.Environment{
				DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
			},
			input:   DeleteAddressRefChangesetInput{AddressRefKeys: []keys.AddressRefKey{addressRefKey(addressRef2)}},
			wantErr: fmt.Sprintf("address ref entry for chain selector %v, type %v, version %v and qualifier %q does not exist", addressRef2.ChainSelector, addressRef2.Type, addressRef2.Version, addressRef2.Qualifier),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := DeleteAddressRefChangeset{}.VerifyPreconditions(tt.env, tt.input)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDeleteAddressRefChangeset_MissingVersion(t *testing.T) {
	t.Parallel()

	input := DeleteAddressRefChangesetInput{
		AddressRefKeys: []keys.AddressRefKey{{
			ChainSelector: 1234,
			Type:          "MyContract",
			Qualifier:     "q1",
		}},
	}
	env := cldf.Environment{
		DataStore: cldfdatastore.NewMemoryDataStore().Seal(),
	}

	err := DeleteAddressRefChangeset{}.VerifyPreconditions(env, input)
	require.ErrorIs(t, err, cldfdatastore.ErrAddressRefVersionRequired)

	_, err = DeleteAddressRefChangeset{}.Apply(env, input)
	require.ErrorIs(t, err, cldfdatastore.ErrAddressRefVersionRequired)
}

func TestDeleteAddressRefChangeset_Apply(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef1 := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	addressRef2 := cldfdatastore.AddressRef{Address: "0x02", ChainSelector: 5678, Type: "OtherContract", Version: version, Qualifier: "q2"}

	tests := []struct {
		name            string
		env             cldf.Environment
		input           DeleteAddressRefChangesetInput
		wantDeletedKeys []string
		wantErr         string
	}{
		{
			name: "success: stages two address ref entries for deletion",
			env: cldf.Environment{
				DataStore:        testDataStoreWithAddressRefs(t, addressRef1, addressRef2).Seal(),
				OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
			},
			input: DeleteAddressRefChangesetInput{
				AddressRefKeys: []keys.AddressRefKey{addressRefKey(addressRef1), addressRefKey(addressRef2)},
			},
			wantDeletedKeys: []string{addressRef1.Key().String(), addressRef2.Key().String()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DeleteAddressRefChangeset{}.Apply(tt.env, tt.input)

			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Len(t, got.Reports, 1)
				memDS := got.DataStore.(*cldfdatastore.MemoryDataStore)
				require.ElementsMatch(t, tt.wantDeletedKeys, memDS.AddressRefStore.DeletedRemoteKeys)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestDeleteAddressRefChangeset_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	version := semver.MustParse("1.0.0")
	addressRef1 := cldfdatastore.AddressRef{Address: "0x01", ChainSelector: 1234, Type: "MyContract", Version: version, Qualifier: "q1"}
	addressRef2 := cldfdatastore.AddressRef{Address: "0x02", ChainSelector: 5678, Type: "OtherContract", Version: version, Qualifier: ""}

	raw := `{"addressRefKeys":[{"chainSelector":1234,"type":"MyContract","version":"1.0.0","qualifier":"q1"},{"chainSelector":5678,"type":"OtherContract","version":"1.0.0","qualifier":""}]}`

	var got DeleteAddressRefChangesetInput
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Equal(t, []keys.AddressRefKey{addressRefKey(addressRef1), addressRefKey(addressRef2)}, got.AddressRefKeys)

	env := cldf.Environment{
		DataStore:        testDataStoreWithAddressRefs(t, addressRef1, addressRef2).Seal(),
		OperationsBundle: cldfoperations.NewBundle(t.Context, cldflogger.Test(t), cldfoperations.NewMemoryReporter()),
	}

	require.NoError(t, DeleteAddressRefChangeset{}.VerifyPreconditions(env, got))

	out, err := DeleteAddressRefChangeset{}.Apply(env, got)
	require.NoError(t, err)
	memDS := out.DataStore.(*cldfdatastore.MemoryDataStore)
	require.ElementsMatch(t,
		[]string{addressRef1.Key().String(), addressRef2.Key().String()},
		memDS.AddressRefStore.DeletedRemoteKeys,
	)
}

func addressRefKey(ref cldfdatastore.AddressRef) keys.AddressRefKey {
	return keys.AddressRefKey{
		ChainSelector: ref.ChainSelector,
		Type:          ref.Type,
		Version:       ref.Version,
		Qualifier:     ref.Qualifier,
	}
}
