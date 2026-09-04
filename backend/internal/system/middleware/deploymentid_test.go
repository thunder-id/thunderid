// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// The point of the middleware is that a handler, and every store below it, can read the id from the
// context instead of reaching for the configuration.
func TestDeploymentIDMiddlewarePutsTheIDOnTheContext(t *testing.T) {
	original := deploymentIDForRequest
	deploymentIDForRequest = func(*http.Request) string { return "acme" }
	t.Cleanup(func() { deploymentIDForRequest = original })

	var seen string
	var carried bool
	handler := DeploymentIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, carried = deployment.IDFromContext(r.Context())
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/anything", nil))

	if !carried || seen != "acme" {
		t.Fatalf("expected the request to carry acme, got (%q, %v)", seen, carried)
	}
}

// With no identifier configured, the context is left alone rather than carrying an empty id, so a
// store falls back to its own configured value exactly as it did before.
func TestDeploymentIDMiddlewareLeavesTheContextAloneWithoutAnIdentifier(t *testing.T) {
	original := deploymentIDForRequest
	deploymentIDForRequest = func(*http.Request) string { return "" }
	t.Cleanup(func() { deploymentIDForRequest = original })

	var carried bool
	handler := DeploymentIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, carried = deployment.IDFromContext(r.Context())
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/anything", nil))

	if carried {
		t.Fatal("expected no deployment id on the context when none is configured")
	}
}
