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
// The deployment id partitions every stored resource by the DEPLOYMENT_ID column. By default it is
// the server-configured identifier (a single-tenant instance). When the security layer extracts a
// deployment claim from the caller's token (enabled by setting server.deployment_id_claim), it
// places that value in the request context, and stores resolve it per request instead of the
// configured default. This is what lets a single instance serve many tenants from one database.
package deployment

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
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
//   - When the context carries a token-derived id (any mode), that id wins.
//   - In "server" mode (the default) with no context id, the supplied fallback - the store's
//     configured identifier - is used. Passing the store's own configured value keeps single-tenant
//     behavior identical.
//   - In "token" mode with no context id, the configured identifier is deliberately NOT honored
//     (that is the whole point of token mode), so an empty id is returned. Requests always carry the
//     id because the security layer rejects authenticated requests without the claim; reaching this
//     branch means a background/system operation that must set an explicit tenant via WithID, and
//     the empty id surfaces that rather than silently scoping to the wrong tenant.
func Resolve(ctx context.Context, fallback string) string {
	if id, ok := fromContext(ctx); ok {
		return id
	}
	if sourceIsToken() {
		return ""
	}
	return fallback
}

// ResolveDefault returns the deployment id for the request using the server-configured identifier as
// the fallback - the same value stores resolve to in server mode. It is for callers that scope by the
// deployment id but have no per-store configured value of their own (e.g. the cache layer). In token
// mode with no context id it returns an empty string, matching Resolve.
func ResolveDefault(ctx context.Context) string {
	if id, ok := fromContext(ctx); ok {
		return id
	}
	if sourceIsToken() {
		return ""
	}
	if config.IsServerRuntimeInitialized() {
		return config.GetServerRuntime().Config.Server.Identifier
	}
	return ""
}

// sourceIsToken reports whether the instance derives the deployment id from the token rather than
// the configured identifier. False when the runtime is not yet initialized (server mode default).
func sourceIsToken() bool {
	if !config.IsServerRuntimeInitialized() {
		return false
	}
	return config.GetServerRuntime().Config.Server.DeploymentIDSource == engineconfig.DeploymentIDSourceToken
}

// IDFromContext reports the token-derived deployment id when the request carries one. It returns
// false for requests with no deployment claim (single-tenant, or non-request/background contexts),
// letting callers that must fail closed distinguish "tenant scoped" from "unscoped".
func IDFromContext(ctx context.Context) (string, bool) {
	return fromContext(ctx)
}
