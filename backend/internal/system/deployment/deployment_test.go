// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deployment

import (
	"context"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// The edge puts the id on the context, so a request always carries one. The fallback is for the
// contexts that never reach the edge: start-up tasks, background jobs and command line tooling.
const (
	requestDeployment    = "acme"
	configuredDeployment = "configured-deployment"
)

// withConfiguredRuntime loads a server runtime naming configuredDeployment and resets it afterwards,
// so a test that needs the fallback does not leave the singleton set for the tests that need it
// absent.
func withConfiguredRuntime(t *testing.T) {
	t.Helper()
	config.ResetServerRuntime()
	cfg := &config.Config{Server: engineconfig.ServerConfig{Identifier: configuredDeployment}}
	if err := config.InitializeServerRuntime("", cfg); err != nil {
		t.Fatalf("failed to initialize the server runtime: %v", err)
	}
	t.Cleanup(config.ResetServerRuntime)
}

func TestResolve_PrefersTheDeploymentTheRequestNames(t *testing.T) {
	withConfiguredRuntime(t)

	ctx := WithID(context.Background(), requestDeployment)
	if got := Resolve(ctx); got != requestDeployment {
		t.Fatalf("expected the id the request carries, got %q", got)
	}
}

func TestResolve_FallsBackToTheConfiguredIdentifier(t *testing.T) {
	withConfiguredRuntime(t)

	if got := Resolve(context.Background()); got != configuredDeployment {
		t.Fatalf("expected the configured identifier for a context that carries no id, got %q", got)
	}
}

// Before the runtime is loaded there is no deployment to name, so resolving reports none rather
// than inventing one.
func TestResolve_IsEmptyBeforeTheRuntimeIsLoaded(t *testing.T) {
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)

	if got := Resolve(context.Background()); got != "" {
		t.Fatalf("expected no deployment before the runtime is loaded, got %q", got)
	}
}

// An empty id is ignored so a caller can pass an unconditionally-extracted claim without branching
// on whether it was present.
func TestWithID_EmptyIsIgnored(t *testing.T) {
	withConfiguredRuntime(t)

	ctx := WithID(context.Background(), "")
	if got := Resolve(ctx); got != configuredDeployment {
		t.Fatalf("expected the fallback for an empty id, got %q", got)
	}
}
