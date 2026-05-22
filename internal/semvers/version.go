// Package semvers defines the canonical semver constants used as version
// labels throughout this repository (e.g. for contracts, operations, and
// sequences).
package semvers

import (
	"github.com/Masterminds/semver/v3"
)

var (
	V1_0_0 = *semver.MustParse("1.0.0")
	V1_5_0 = *semver.MustParse("1.5.0")
	V1_6_0 = *semver.MustParse("1.6.0")
)
