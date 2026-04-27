package soltestutils

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	solutils "github.com/smartcontractkit/cld-changesets/pkg/family/solana/utils"
)

// LoadMCMSPrograms loads the MCMS program artifacts into the given directory.
//
// Returns the path to the temporary test directory and a map of program names to IDs.
func LoadMCMSPrograms(t *testing.T, dir string) (string, map[string]string) {
	t.Helper()

	progIDs := loadProgramArtifacts(t,
		solutils.MCMSProgramNames, downloadChainlinkCCIPProgramArtifacts, dir,
	)

	return dir, progIDs
}

// PreloadMCMS provides a convenience function to preload the MCMS program artifacts and address
// book for a given selector.
func PreloadMCMS(t *testing.T, selector uint64) (string, map[string]string, *cldf.AddressBookMap) {
	t.Helper()

	dir := t.TempDir()

	_, programIDs := LoadMCMSPrograms(t, dir)

	ab := PreloadAddressBookWithMCMSPrograms(t, selector)

	return dir, programIDs, ab
}

// loadProgramArtifacts is a helper function that loads program artifacts into a temporary test directory.
// It downloads artifacts using the provided download function and copies the specified programs.
//
// Returns the map of program names to IDs.
func loadProgramArtifacts(t *testing.T, programNames []string, downloadFn downloadFunc, targetDir string) map[string]string {
	t.Helper()

	// Download the program artifacts using the provided download function
	cachePath := downloadFn(t)

	progIDs := make(map[string]string, len(programNames))

	// Copy the specific artifacts to the target directory and add the program ID to the map
	for _, name := range programNames {
		id := solutils.GetProgramID(name)
		require.NotEmpty(t, id, "program id not found for program name: %s", name)

		src := filepath.Join(cachePath, name+".so")
		dst := filepath.Join(targetDir, name+".so")

		func() {
			srcFile, err := os.Open(src)
			require.NoError(t, err)
			defer srcFile.Close()

			dstFile, err := os.Create(dst)
			require.NoError(t, err)
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			require.NoError(t, err)
		}()

		// Add the program ID to the map
		progIDs[name] = id
		t.Logf("copied solana program %s to %s", name, dst)
	}

	// Return the path to the cached artifacts and the map of program IDs
	return progIDs
}
