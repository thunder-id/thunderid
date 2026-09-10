// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package deployment carries the per-request deployment identifier used to scope persistence.
//
// The id partitions every stored resource by the DEPLOYMENT_ID column. It is put on the request
// context at the edge, so a store reads it from one place rather than reaching for configuration.
// This server holds one deployment, so that id is the configured identifier.
package deployment

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// ctxKey is the private context key under which the per-request deployment id is stored.
type ctxKey struct{}

// WithID returns a context carrying the given deployment id. An empty id is ignored so callers can
// pass an unconditionally-extracted claim without having to branch on its presence.
func WithID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// fromContext returns the deployment id carried by the context, if any.
func fromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok && id != ""
}

// Resolve returns the deployment id a store should scope by for this request.
//
// The id is put on the context at the edge, so a request always carries one and that is what a
// store scopes by. Contexts that never passed through the edge, such as start-up tasks, background
// jobs and command line tooling, fall back to the configured identifier, which this package reads
// so that a store does not have to reach for configuration itself.
//
// The fallback is empty until the server runtime is loaded, which is the correct answer that early:
// there is no deployment to name yet.
func Resolve(ctx context.Context) string {
	if id, ok := fromContext(ctx); ok {
		return id
	}
	if !config.IsServerRuntimeInitialized() {
		return ""
	}
	return config.GetServerRuntime().Config.Server.Identifier
}
