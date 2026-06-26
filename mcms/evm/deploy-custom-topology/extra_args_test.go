package evmdeploytopology

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseEVMExtraArgs(t *testing.T) {
	t.Parallel()

	t.Run("nil yields defaults", func(t *testing.T) {
		t.Parallel()
		ea, err := parseEVMExtraArgs(nil)
		require.NoError(t, err)
		require.True(t, ea.shouldDeployCallProxy("timelock")) // default on
	})

	t.Run("typed value", func(t *testing.T) {
		t.Parallel()
		in := EVMExtraArgs{DeployCallProxyByTimelockRef: map[string]bool{"a": false}}
		ea, err := parseEVMExtraArgs(in)
		require.NoError(t, err)
		require.False(t, ea.shouldDeployCallProxy("a"))
		require.True(t, ea.shouldDeployCallProxy("b")) // unset -> default on
	})

	t.Run("typed pointer", func(t *testing.T) {
		t.Parallel()
		in := &EVMExtraArgs{DeployCallProxyByTimelockRef: map[string]bool{"a": false}}
		ea, err := parseEVMExtraArgs(in)
		require.NoError(t, err)
		require.False(t, ea.shouldDeployCallProxy("a"))
	})

	t.Run("nil pointer yields defaults", func(t *testing.T) {
		t.Parallel()
		var in *EVMExtraArgs
		ea, err := parseEVMExtraArgs(in)
		require.NoError(t, err)
		require.True(t, ea.shouldDeployCallProxy("a"))
	})

	t.Run("json round-trip from map", func(t *testing.T) {
		t.Parallel()
		in := map[string]any{"deployCallProxyByTimelockRef": map[string]any{"a": false, "b": true}}
		ea, err := parseEVMExtraArgs(in)
		require.NoError(t, err)
		require.False(t, ea.shouldDeployCallProxy("a"))
		require.True(t, ea.shouldDeployCallProxy("b"))
	})

	t.Run("unmarshalable yields error", func(t *testing.T) {
		t.Parallel()
		_, err := parseEVMExtraArgs(make(chan int))
		require.Error(t, err)
	})
}
