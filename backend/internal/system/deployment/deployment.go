/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package deployment carries the per-request deployment identifier used to scope persistence.
//
// The id partitions every stored resource by the DEPLOYMENT_ID column. It is put on the request
// context at the edge, so a store reads it from one place rather than reaching for configuration.
// This server holds one deployment, so that id is the configured identifier.
package deployment

import (
	"context"
	"strings"

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
// The id is put on the context at the edge, so a request always carries one and that is what a store
// scopes by. The fallback covers the contexts that never passed through the edge: start-up tasks,
// background jobs and command line tooling, which supply the store's own configured identifier.
func Resolve(ctx context.Context, fallback string) string {
	if id, ok := fromContext(ctx); ok {
		return id
	}
	return fallback
}

// ResolveDefault returns the deployment id for the request using the server-configured identifier as
// the fallback, the same value stores resolve to. It is for callers that scope by the deployment id
// but have no configured value of their own, such as the cache layer. Where the id comes from a token
// claim, a context carrying none returns an empty string, matching Resolve.
func ResolveDefault(ctx context.Context) string {
	if id, ok := fromContext(ctx); ok {
		return id
	}
	if config.IsServerRuntimeInitialized() {
		return config.GetServerRuntime().Config.Server.Identifier
	}
	return ""
}

// IDFromContext reports the deployment id the context carries, and whether it carries one at all.
// It returns false for a context that never passed through the edge, such as a background job,
// letting a caller that must fail closed tell "scoped" from "unscoped".
func IDFromContext(ctx context.Context) (string, bool) {
	return fromContext(ctx)
}

// OrganizationOf returns the organization a deployment id belongs to.
//
// An id names a gateway as "<org>:<gateway>", and what an organization owns across its gateways is
// partitioned under the organization rather than any one of them. An id naming no organization is
// its own, which is what a single-tenant deployment has.
func OrganizationOf(id string) string {
	org, _, found := strings.Cut(id, ":")
	if !found || strings.TrimSpace(org) == "" {
		return id
	}
	return org
}
