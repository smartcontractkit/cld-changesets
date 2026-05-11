package soltestutils

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProgramsCacheDir(t *testing.T) {
	t.Parallel()

	got := programsCacheDir()
	require.True(t, filepath.IsAbs(got), "got %q", got)
	require.True(t, strings.HasSuffix(got, filepath.Join("cld-changesets", "solana_programs")), "got %q", got)
	require.NotContains(t, got, filepath.Join("pkg", "mod"))
}
