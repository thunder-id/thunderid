// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// MinNodeVersion is the lowest Node.js version the sample apps (installed and
// run via npm by the try-* commands) are supported on.
const MinNodeVersion = "22.23.1"

// NodeUpgradeHint returns instructions for updating Node.js via nvm, plus the
// download URL for developers who don't use nvm.
func NodeUpgradeHint() string {
	return fmt.Sprintf(
		"Update with nvm:\n  nvm install %s\n  nvm use %s\nOr download it from https://nodejs.org/en/download",
		MinNodeVersion, MinNodeVersion,
	)
}

// DetectNodeVersion runs "node --version" and returns the version string
// without the leading "v" (e.g. "22.23.1").
func DetectNodeVersion() (string, error) {
	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return "", fmt.Errorf("node.js not found — v%s or later is required to run sample apps; install it from "+
			"https://nodejs.org/en/download", MinNodeVersion)
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "v"), nil
}

// MeetsMinNodeVersion reports whether version is >= MinNodeVersion.
func MeetsMinNodeVersion(version string) bool {
	return compareVersions(version, MinNodeVersion) >= 0
}

// compareVersions compares two dot-separated numeric version strings,
// returning -1, 0, or 1 depending on whether a is less than, equal to, or
// greater than b. Missing or non-numeric components are treated as 0.
func compareVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv int
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}
