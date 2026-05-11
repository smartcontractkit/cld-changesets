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

// programsCacheDir returns where to store downloaded .so files. Leaf dir is solana_programs
// (under UserCacheDir/TempDir, so "cache" is implied; avoids read-only pkg/mod paths).
func programsCacheDir() string {
	root, err := os.UserCacheDir()
	if err != nil {
		root = os.TempDir()
	}
	return filepath.Join(root, "cld-changesets", "solana_programs")
}
