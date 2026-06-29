package soltestutils

import (
	"os"
	"path/filepath"
)

// programsCacheDir returns where to store downloaded .so files. Leaf dir is solana_programs
// (under UserCacheDir/TempDir, so "cache" is implied; avoids read-only pkg/mod paths).
func programsCacheDir() string {
	root, err := os.UserCacheDir()
	if err != nil {
		root = os.TempDir()
	}

	return filepath.Join(root, "cld-changesets", "solana_programs")
}
