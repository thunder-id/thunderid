/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

package security

import (
	"context"
)

type contextKey string

const (
	// securityContextKey is the context key for storing security context.
	securityContextKey contextKey = "security_context"

	// runtimeContextKey is the context key for marking a context as an internal runtime caller.
	runtimeContextKey contextKey = "runtime_context"
)

// SecurityContext holds immutable authenticated subject information.
type SecurityContext struct {
	subject       string
	ouID          string
	token         string
	revocationID  string
	tokenFamilyID string
	permissions   []string
	attributes    map[string]interface{}
}

// newSecurityContext creates a new immutable SecurityContext.
func newSecurityContext(subject, ouID, token string,
	permissions []string, attributes map[string]interface{}) *SecurityContext {
	return &SecurityContext{
		subject:     subject,
		ouID:        ouID,
		token:       token,
		permissions: permissions,
		attributes:  attributes,
	}
}

// withSecurityContext adds security context to the request context.
func withSecurityContext(ctx context.Context, authCtx *SecurityContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, securityContextKey, authCtx)
}

// GetSubject retrieves the authenticated subject from the context.
// Returns empty string if no security context is present.
func GetSubject(ctx context.Context) string {
	authCtx := getSecurityContext(ctx)
	if authCtx != nil {
		return authCtx.subject
	}
	return ""
}

// GetOUID retrieves the organization unit ID from the context.
// Returns empty string if no security context is present.
func GetOUID(ctx context.Context) string {
	authCtx := getSecurityContext(ctx)
	if authCtx != nil {
		return authCtx.ouID
	}
	return ""
}

// GetPermissions retrieves the granted permissions for the authenticated caller from the context.
// Returns a defensive copy to prevent callers from modifying the underlying permissions data.
func GetPermissions(ctx context.Context) []string {
	authCtx := getSecurityContext(ctx)
	if authCtx != nil {
		result := make([]string, len(authCtx.permissions))
		copy(result, authCtx.permissions)
		return result
	}
	return []string{}
}

// GetAttribute retrieves a specific attribute from the security token.
// Returns defensive copies for mutable types (slices, maps) to prevent modification.
// Returns nil if no security context is present or attribute doesn't exist.
func GetAttribute(ctx context.Context, key string) interface{} {
	authCtx := getSecurityContext(ctx)
	if authCtx == nil {
		return nil
	}

	value, exists := authCtx.attributes[key]
	if !exists {
		return nil
	}

	// Return defensive copies for mutable types to prevent modification
	switch v := value.(type) {
	case []string:
		result := make([]string, len(v))
		copy(result, v)
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		copy(result, v)
		return result
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for k, val := range v {
			result[k] = val
		}
		return result
	default:
		// Immutable types (string, int, bool, etc.) are safe to return directly
		return value
	}
}

// WithRuntimeContext marks the context as an internal runtime caller.
// Runtime contexts bypass standard subject-based authorization checks without requiring an
// authenticated subject. This is intended for internal system operations initiated from public
// paths (e.g., flow executors during registration, token enrichment) where no user token is available.
func WithRuntimeContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, runtimeContextKey, true)
}

// IsRuntimeContext returns true if the context was marked as an internal runtime caller
// via WithRuntimeContext.
func IsRuntimeContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(runtimeContextKey).(bool)
	return v
}

// getSecurityContext is an internal helper to retrieve the security context.
// This function is unexported to prevent downstream services from accessing the raw context object.
func getSecurityContext(ctx context.Context) *SecurityContext {
	if ctx == nil {
		return nil
	}
	authCtx, _ := ctx.Value(securityContextKey).(*SecurityContext)
	return authCtx
}
