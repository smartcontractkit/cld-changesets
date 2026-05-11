package soltestutils

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
)

var (
	onceCCIP = &sync.Once{}
)

type downloadFunc func(t *testing.T) string

// downloadChainlinkCCIPProgramArtifacts downloads CCIP Solana artifacts (includes MCMS programs).
func downloadChainlinkCCIPProgramArtifacts(t *testing.T) string {
	t.Helper()

	cachePath := programsCacheDir()

	onceCCIP.Do(func() {
		err := solutils.DownloadChainlinkCCIPProgramArtifacts(t.Context(), cachePath, "", nil)
		require.NoError(t, err)
	})

	return cachePath
}

// programsCacheDir returns a writable directory for cached .so files (avoids pkg/mod when loaded as a module).
func programsCacheDir() string {
	root, err := os.UserCacheDir()
	if err != nil {
		root = os.TempDir()
	}
	return filepath.Join(root, "cld-changesets", "solana-test-programs")
}
