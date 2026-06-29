package soltestutils

import (
	"maps"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
)

// solTestExclusive serializes top-level Solana integration tests that mutate global
// gobinding program IDs via SetProgramID. solTestDepth allows nested subtests in
// the same test to re-enter without deadlocking.
var (
	solTestExclusive sync.Mutex
	solTestCountMu   sync.Mutex
	solTestDepth     int
)

func acquireSolanaTestIsolation(t *testing.T) {
	t.Helper()

	solTestCountMu.Lock()
	if solTestDepth == 0 {
		solTestExclusive.Lock()
	}
	solTestDepth++
	solTestCountMu.Unlock()

	t.Cleanup(func() {
		solTestCountMu.Lock()
		solTestDepth--
		release := solTestDepth == 0
		solTestCountMu.Unlock()
		if release {
			solTestExclusive.Unlock()
		}
	})
}

var (
	mcmsProgramsOnce sync.Once
	mcmsProgramsPath string
	mcmsProgramIDs   map[string]string
)

// sharedMCMSPrograms downloads MCMS Solana program artifacts once per test process
// and returns the shared cache directory plus program IDs.
func sharedMCMSPrograms(t *testing.T) (string, map[string]string) {
	t.Helper()

	mcmsProgramsOnce.Do(func() {
		mcmsProgramsPath = programsCacheDir()
		err := solutils.DownloadChainlinkCCIPProgramArtifacts(t.Context(), mcmsProgramsPath, "", nil)
		require.NoError(t, err)

		mcmsProgramIDs = make(map[string]string, len(solutils.MCMSProgramNames))
		for _, name := range solutils.MCMSProgramNames {
			id := solutils.GetProgramID(name)
			require.NotEmpty(t, id, "program id not found for program name: %s", name)
			require.FileExists(t, filepath.Join(mcmsProgramsPath, name+".so"))
			mcmsProgramIDs[name] = id
		}
	})

	programIDs := make(map[string]string, len(mcmsProgramIDs))
	maps.Copy(programIDs, mcmsProgramIDs)

	return mcmsProgramsPath, programIDs
}

// PreloadMCMS provides a convenience function to preload the MCMS program artifacts and address
// book for a given selector.
func PreloadMCMS(t *testing.T, selector uint64) (string, map[string]string, *cldf.AddressBookMap) {
	t.Helper()

	acquireSolanaTestIsolation(t)

	programsPath, programIDs := sharedMCMSPrograms(t)
	ab := PreloadAddressBookWithMCMSPrograms(t, selector)

	return programsPath, programIDs, ab
}
