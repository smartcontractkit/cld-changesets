package operations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	fwops "github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-deployments-framework/pkg/logger"

	"github.com/smartcontractkit/chainlink-deployments-framework/engine/test/runtime"
)

func newBundle(t *testing.T) fwops.Bundle {
	t.Helper()

	return fwops.NewBundle(func() context.Context { return t.Context() }, logger.Test(t), fwops.NewMemoryReporter())
}

func TestOpJDRegisterNode(t *testing.T) {
	t.Parallel()

	t.Run("registers new node and returns its ID", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		deps := JDOpDeps{Offchain: rt.Environment().Offchain, EnvName: "test"}
		report, err := fwops.ExecuteOperation(newBundle(t), OpJDRegisterNode, deps, RegisterNodeInput{
			Domain: "keystone",
			Name:   "oracle-1",
			CSAKey: "csa-key-1",
		})
		require.NoError(t, err)
		require.NotEmpty(t, report.Output.NodeID)
		require.False(t, report.Output.Skipped)
	})

	t.Run("second call with same CSA key is a no-op, returns same ID with Skipped=true", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		deps := JDOpDeps{Offchain: rt.Environment().Offchain, EnvName: "test"}
		input := RegisterNodeInput{Domain: "keystone", Name: "oracle-1", CSAKey: "csa-key-1"}

		first, err := fwops.ExecuteOperation(newBundle(t), OpJDRegisterNode, deps, input)
		require.NoError(t, err)
		require.False(t, first.Output.Skipped)

		second, err := fwops.ExecuteOperation(newBundle(t), OpJDRegisterNode, deps, input)
		require.NoError(t, err)
		require.True(t, second.Output.Skipped, "expected second registration to be skipped")
		require.Equal(t, first.Output.NodeID, second.Output.NodeID, "skipped op must return the existing node ID")
	})
}

func TestOpJDDisableNode(t *testing.T) {
	t.Parallel()

	t.Run("disables an existing node and returns its ID", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		deps := JDOpDeps{Offchain: rt.Environment().Offchain, EnvName: "test"}
		reg, err := fwops.ExecuteOperation(newBundle(t), OpJDRegisterNode, deps, RegisterNodeInput{
			Domain: "keystone",
			Name:   "oracle-1",
			CSAKey: "csa-key-1",
		})
		require.NoError(t, err)

		dis, err := fwops.ExecuteOperation(newBundle(t), OpJDDisableNode, deps, DisableNodeInput{CSAKey: "csa-key-1"})
		require.NoError(t, err)
		require.False(t, dis.Output.Skipped)
		require.Equal(t, reg.Output.NodeID, dis.Output.NodeID)
	})

	t.Run("node not found — Skipped=true, no error", func(t *testing.T) {
		t.Parallel()

		rt, err := runtime.New(t.Context())
		require.NoError(t, err)

		deps := JDOpDeps{Offchain: rt.Environment().Offchain, EnvName: "test"}
		report, err := fwops.ExecuteOperation(newBundle(t), OpJDDisableNode, deps, DisableNodeInput{CSAKey: "nonexistent-key"})
		require.NoError(t, err)
		require.True(t, report.Output.Skipped, "expected not found disable to be marked skipped")
		require.Empty(t, report.Output.NodeID)
	})
}
