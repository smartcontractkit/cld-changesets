package solutils

// Program names
const (
	ProgMCM              = "mcm"
	ProgTimelock         = "timelock"
	ProgAccessController = "access_controller"
)

// MCMSProgramNames names grouped by their usage.
var (
	MCMSProgramNames = []string{ProgMCM, ProgTimelock, ProgAccessController}
)

// Repositories that contain the program artifacts.
const (
	repoCCIP = "chainlink-ccip"
)

// Directory maps program names to their corresponding program information, including program ID, repository, and buffer size for upgrades.
var Directory = directory{

	// MCMS Programs
	ProgMCM:              {ID: "5vNJx78mz7KVMjhuipyr9jKBKcMrKYGdjGkgE4LUmjKk", Repo: repoCCIP, ProgramBufferBytes: 1 * 1024 * 1024},
	ProgTimelock:         {ID: "DoajfR5tK24xVw51fWcawUZWhAXD8yrBJVacc13neVQA", Repo: repoCCIP, ProgramBufferBytes: 1 * 1024 * 1024},
	ProgAccessController: {ID: "6KsN58MTnRQ8FfPaXHiFPPFGDRioikj9CdPvPxZJdCjb", Repo: repoCCIP, ProgramBufferBytes: 1 * 1024 * 1024},
}

// GetProgramID returns the program ID for a given program name.
//
// Returns the program ID for the given program name or an empty string if the program is not
// found.
func GetProgramID(name string) string {
	info, ok := Directory[name]
	if !ok {
		return ""
	}

	return info.ID
}

// GetProgramBufferBytes returns the size of the program buffer in bytes for the given program name.
//
// Returns 0 if the program is not found or if the program is not upgradable.
func GetProgramBufferBytes(name string) int {
	info, ok := Directory[name]
	if !ok {
		return 0
	}

	return info.ProgramBufferBytes
}

// programInfo contains the information about a program.
type programInfo struct {
	// ID is the program ID of the program.
	ID string

	// Repo is the repository name of where the program is located.
	Repo string

	// ProgramBufferBytes is the size of the program buffer in bytes. Used for upgrades.
	// Can be left blank if the program is not upgradable.
	//
	// https://docs.google.com/document/d/1Fk76lOeyS2z2X6MokaNX_QTMFAn5wvSZvNXJluuNV1E/edit?tab=t.0#heading=h.uij286zaarkz
	// https://docs.google.com/document/d/1nCNuam0ljOHiOW0DUeiZf4ntHf_1Bw94Zi7ThPGoKR4/edit?tab=t.0#heading=h.hju45z55bnqd
	ProgramBufferBytes int
}

// directory maps the program name to the program information.
type directory map[string]programInfo
