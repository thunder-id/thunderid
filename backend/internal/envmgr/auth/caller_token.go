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

// Package auth carries the caller's bearer token across a request so the environment manager can
// forward it to a control plane that trusts the same issuer.
//
// Only the context plumbing is kept from the standalone service: token validation itself is done by
// this server's own middleware before a request reaches these handlers.
package auth

import (
	"context"
	"net/http"
	"strings"
)

type tokenKey struct{}

// WithCallerToken records the caller's raw bearer token so it can be forwarded to a server that
// trusts the same issuer.
func WithCallerToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey{}, token)
}

// CallerTokenFromContext returns the caller's raw bearer token, if the request carried one.
func CallerTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(tokenKey{}).(string)
	return token
}

// RecordCallerToken stashes the request's bearer token on the context. The export and import calls
// the environment manager makes are performed as the caller, so without this every one of them
// would reach the control plane unauthenticated.
func RecordCallerToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if raw := strings.TrimPrefix(header, "Bearer "); raw != header && raw != "" {
			r = r.WithContext(WithCallerToken(r.Context(), raw))
		}
		next(w, r)
	}
}
