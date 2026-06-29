package soltestutils

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	cldf "github.com/smartcontractkit/chainlink-deployments-framework/deployment"

	"github.com/smartcontractkit/cld-changesets/legacy/pkg/family/solana/solutils"
)

// programIDMu serializes Solana integration tests that mutate global gobinding
// program IDs via SetProgramID. solana-go bindings use process-wide state, so
// parallel package tests otherwise race and fail with "Program is not deployed".
var programIDMu sync.Mutex

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

	return mcmsProgramsPath, copyProgramIDs(mcmsProgramIDs)
}

func copyProgramIDs(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for name, id := range src {
		dst[name] = id
	}

	return dst
}

// PreloadMCMS provides a convenience function to preload the MCMS program artifacts and address
// book for a given selector.
func PreloadMCMS(t *testing.T, selector uint64) (string, map[string]string, *cldf.AddressBookMap) {
	t.Helper()

	programIDMu.Lock()
	t.Cleanup(programIDMu.Unlock)

	programsPath, programIDs := sharedMCMSPrograms(t)
	ab := PreloadAddressBookWithMCMSPrograms(t, selector)

	return programsPath, programIDs, ab
}
