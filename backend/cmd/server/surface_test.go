// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// controlPlaneOnlyPackages are the management packages only the Control Plane serves. The all-in-one
// server also runs as the Data Plane, which works on organization unit IDs alone.
var controlPlaneOnlyPackages = []string{
	"internal/project",
}

// TestServerLinksNoControlPlaneOnlyPackage fails if the all-in-one server links a package only the
// Control Plane serves.
func TestServerLinksNoControlPlaneOnlyPackage(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/thunder-id/thunderid/cmd/server").Output()
	require.NoError(t, err, "go list -deps")
	deps := strings.Fields(string(out))

	for _, pkg := range controlPlaneOnlyPackages {
		full := "github.com/thunder-id/thunderid/" + pkg
		for _, dep := range deps {
			if dep == full || strings.HasPrefix(dep, full+"/") {
				t.Errorf("the all-in-one server links Control Plane only package %s", dep)
			}
		}
	}
}
