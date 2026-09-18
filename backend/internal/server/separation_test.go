// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runtimePackages are the surfaces that serve traffic. A control plane runs none of them.
var runtimePackages = []string{
	"github.com/thunder-id/thunderid/internal/authn",
	"github.com/thunder-id/thunderid/internal/authzen",
	"github.com/thunder-id/thunderid/internal/oauth",
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dcr",
	"github.com/thunder-id/thunderid/internal/flow/flowexec",
	"github.com/thunder-id/thunderid/internal/openid4vci",
}

// The separation is a property of the binary, not of a setting, and this is what makes that
// checkable. A control plane that linked the OAuth server would still not route to it today, but
// nothing would stop the next change from doing so; a compiled-in package is one import away from
// being reachable.
func TestControlPlaneLinksNoRuntimePackage(t *testing.T) {
	linked := dependenciesOf(t, "../../cmd/cp")

	for _, pkg := range runtimePackages {
		if linked[pkg] {
			t.Errorf("the control plane binary links %s, which serves runtime traffic.\n"+
				"Something it imports reaches that package: find the path with\n"+
				"  go list -deps ./cmd/cp | grep %s", pkg, pkg)
		}
	}
}

// The counterpart, so a mistake that removes a surface from the data plane is caught here rather
// than by a login failing.
func TestDataPlaneLinksEveryRuntimePackage(t *testing.T) {
	linked := dependenciesOf(t, "../../cmd/server")

	for _, pkg := range runtimePackages {
		if !linked[pkg] {
			t.Errorf("the data plane binary does not link %s, so it serves none of that surface", pkg)
		}
	}
}

// Both planes serve the management APIs, which is why the shared build exists.
func TestBothPlanesLinkTheManagementServices(t *testing.T) {
	management := []string{
		"github.com/thunder-id/thunderid/internal/application",
		"github.com/thunder-id/thunderid/internal/user",
		"github.com/thunder-id/thunderid/internal/ou",
		"github.com/thunder-id/thunderid/internal/flow/mgt",
		"github.com/thunder-id/thunderid/internal/system/importer",
	}

	for plane, dir := range map[string]string{
		"control plane": "../../cmd/cp",
		"data plane":    "../../cmd/server",
	} {
		linked := dependenciesOf(t, dir)
		for _, pkg := range management {
			if !linked[pkg] {
				t.Errorf("the %s does not link %s", plane, pkg)
			}
		}
	}
}

// dependenciesOf returns every package linked into the binary built from dir.
func dependenciesOf(t *testing.T, dir string) map[string]bool {
	t.Helper()

	out, err := exec.Command("go", "list", "-deps", dir).Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", dir, err)
	}
	linked := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		linked[strings.TrimSpace(line)] = true
	}
	return linked
}

// A package can be linked into the control plane for its service while its HTTP routes belong to
// the data plane alone. OpenID4VP is the case: the management surface reads presentation
// definitions through the verifier, so the control plane holds it, but the wallet and verifier
// endpoints are runtime and must be mounted only where runtime traffic is served.
//
// The linkage tests above cannot see that difference, which is how an earlier version of this change
// shipped a control plane serving six OpenID4VP routes. This asserts the property those tests miss:
// the call that mounts them appears in the data plane's wiring and nowhere else.
func TestOnlyTheDataPlaneMountsOpenID4VPRoutes(t *testing.T) {
	callers := packagesCalling(t, "openid4vp.RegisterRoutes")

	for _, pkg := range callers {
		if pkg != "../dataplane" {
			t.Errorf("%s mounts the OpenID4VP routes; only the data plane may.\n"+
				"A control plane holds the verifier for its management reads and serves none of its "+
				"endpoints.", pkg)
		}
	}
	if len(callers) == 0 {
		t.Error("nothing mounts the OpenID4VP routes, so the data plane no longer serves them")
	}
}

// packagesCalling returns the internal packages whose non-test sources contain call.
func packagesCalling(t *testing.T, call string) []string {
	t.Helper()

	var found []string
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return err
		}
		// The path comes from walking this repository's own source tree, not from input.
		source, readErr := os.ReadFile(filepath.Clean(path)) //nolint:gosec // walking our own tree
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(source), call+"(") {
			found = append(found, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the source tree: %v", err)
	}
	return found
}
