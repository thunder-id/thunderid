// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// controlPlanePackage is the Control Plane binary, named by import path rather than by a relative
// path because `go test` runs with the package directory as the working directory.
const controlPlanePackage = "github.com/thunder-id/thunderid/cmd/cpserver"

// runtimePackages are the packages that make a server serve traffic: token issuance, login and
// registration, flow execution, authorization evaluation, and the stores and caches they need.
//
// The Control Plane authors configuration and validates it. It must not link any of these, because
// linking them is what drags in their dependencies and turns the Control Plane back into a server
// that could serve runtime traffic. Route gating alone would not achieve that: the code would still
// be in the binary.
var runtimePackages = []string{
	"internal/flow/executor",
	"internal/flow/flowexec",
	"internal/oauth",
	"internal/openid4vci",
	"internal/authn",
	"internal/authnprovider",
	"internal/authz",
	"internal/authzen",
	"internal/consent",
	"internal/runtimestore",
	"internal/attributecache",
	"internal/actorprovider",
	"internal/vc/openid4vp",
}

// allowedRuntimeLeaves are packages under a runtime tree that hold only types, constants and
// configuration shapes. None declares an Initialize or touches an http.ServeMux, so linking one
// constructs no service and registers no route.
//
// They are reachable because the Control Plane authors the configuration these types describe: an
// OAuth application's settings are written here even though no token is ever issued here.
var allowedRuntimeLeaves = map[string]struct{}{
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model":     {},
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants": {},
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/utils":     {},
	"github.com/thunder-id/thunderid/internal/oauth/config":           {},
	"github.com/thunder-id/thunderid/internal/authnprovider/common":   {},
}

// TestControlPlaneLinksNoRuntime fails if the Control Plane binary links a runtime package.
//
// This is the check that keeps the plane split honest. Wiring a service into servicemanager.go that
// reaches a runtime package compiles perfectly well and fails nothing else, so without this the
// separation decays on the next feature that finds it convenient.
func TestControlPlaneLinksNoRuntime(t *testing.T) {
	deps := listDeps(t, controlPlanePackage)

	for _, pkg := range runtimePackages {
		full := "github.com/thunder-id/thunderid/" + pkg
		for _, dep := range deps {
			if _, allowed := allowedRuntimeLeaves[dep]; allowed {
				continue
			}
			if dep == full || strings.HasPrefix(dep, full+"/") {
				t.Errorf("the Control Plane binary links runtime package %s\n"+
					"wire the management service it needs instead, or narrow the dependency", dep)
			}
		}
	}
}

// TestControlPlaneLinksManagement is the other half: the Control Plane must still carry the
// management services, so a change that trims it too far is caught as well.
func TestControlPlaneLinksManagement(t *testing.T) {
	deps := listDeps(t, controlPlanePackage)
	index := make(map[string]struct{}, len(deps))
	for _, d := range deps {
		index[d] = struct{}{}
	}

	for _, pkg := range []string{
		"internal/application",
		"internal/agent",
		"internal/flow/mgt",
		"internal/flow/executormeta",
		"internal/idp",
		"internal/ou",
		"internal/role",
		"internal/system/importer",
		"internal/system/export",
	} {
		full := "github.com/thunder-id/thunderid/" + pkg
		if _, ok := index[full]; !ok {
			t.Errorf("the Control Plane binary no longer links management package %s", pkg)
		}
	}
}

// listDeps returns the full transitive package list of the given package.
func listDeps(t *testing.T, pkg string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", pkg).Output()
	require.NoError(t, err, "go list -deps %s", pkg)
	return strings.Fields(string(out))
}
