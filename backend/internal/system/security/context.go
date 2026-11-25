// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"strconv"
	"strings"
	"time"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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
	subject             string
	ouID                string
	token               string
	revocationID        string
	tokenFamilyID       string
	revocationSubject   string
	revocationAppKey    string
	revocationAudiences []string
	selfIssued          bool
	recorded            bool
	establishedAt       time.Time
	permissions         []string
	attributes          map[string]interface{}
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

// revocationIdentity returns the token attributes the revocation enforcer checks.
func (c *SecurityContext) revocationIdentity() RevocationIdentity {
	return RevocationIdentity{
		JTI:           c.revocationID,
		TokenFamilyID: c.tokenFamilyID,
		Subject:       c.revocationSubject,
		AppKey:        c.revocationAppKey,
		Audiences:     c.revocationAudiences,
		Scopes:        c.permissions,
		EstablishedAt: c.establishedAt,
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

// IsSelfIssued reports whether the caller presented a token this deployment issued. It is false for a
// trusted-issuer token, a management API key and a context without a security context.
func IsSelfIssued(ctx context.Context) bool {
	authCtx := getSecurityContext(ctx)
	return authCtx != nil && authCtx.selfIssued
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

// IsManagementAPIKey reports whether the context was authenticated by the management API key. Only
// that authenticator builds a context with its subject and no token, so a bearer token whose sub
// claim carries the same string is not taken for the key.
func IsManagementAPIKey(ctx context.Context) bool {
	authCtx := getSecurityContext(ctx)
	return authCtx != nil && authCtx.subject == managementAPIKeySubject && authCtx.token == ""
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

// IsDelegated reports whether the verified token on the context is a delegated token: one issued to a
// client acting for a user, which carries the client in an "act" claim (RFC 8693 §4.1). It reads only the
// verified token claims, so a request cannot supply or hide the answer.
func IsDelegated(ctx context.Context) bool {
	authCtx := getSecurityContext(ctx)
	if authCtx == nil {
		return false
	}
	return authCtx.attributes[claimActor] != nil
}

// GetClientID returns the OAuth client the verified token on the context was issued to, read from its
// client_id claim. It is empty when there is no token or the token names no client.
func GetClientID(ctx context.Context) string {
	clientID, _ := GetAttribute(ctx, claimClientID).(string)
	return clientID
}

// PermissionMaxAgeReaderInterface reads the maxAge a resource server marks a permission with.
type PermissionMaxAgeReaderInterface interface {
	GetPermissionMaxAge(
		ctx context.Context, resourceServerID string, permission string,
	) (int64, *tidcommon.ServiceError)
}

// RequireRecentAuthentication refuses the token when its auth_time is older than the maxAge the System
// resource server marks the self-service permission with. An empty permission, as on the control plane, checks
// nothing.
func RequireRecentAuthentication(
	ctx context.Context, reader PermissionMaxAgeReaderInterface, permission string,
) *tidcommon.ServiceError {
	if permission == "" {
		return nil
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	if reader == nil {
		logger.Error(ctx, "No reader for the maximum authentication age of the permission",
			log.String("permission", permission))
		return &tidcommon.InternalServerError
	}
	maxAge, svcErr := reader.GetPermissionMaxAge(ctx, serverconst.SystemResourceServerID,
		declaredSelfPermission(permission))
	if svcErr != nil {
		logger.Error(ctx, "Failed to read the maximum authentication age of the permission",
			log.String("permission", permission), log.String("code", svcErr.Code))
		return &tidcommon.InternalServerError
	}
	if authenticationExceedsMaxAge(ctx, maxAge, time.Now().Unix()) {
		return ErrorRecentAuthenticationRequired.WithParams(
			map[string]string{ErrorParamMaxAge: strconv.FormatInt(maxAge, 10)})
	}
	return nil
}

// declaredSelfPermission returns a self-service permission as the System resource server declares it, without
// the deployment's system permission prefix.
func declaredSelfPermission(permission string) string {
	if sysPerms == nil || sysPerms.SelfService == "" {
		return permission
	}
	return permissionRootSelf + strings.TrimPrefix(permission, sysPerms.SelfService)
}

// authenticationExceedsMaxAge reports whether the token's auth_time is more than maxAge seconds before now, in Unix
// seconds. A token without auth_time exceeds every limit; a maxAge of zero or less sets none.
func authenticationExceedsMaxAge(ctx context.Context, maxAge int64, now int64) bool {
	if maxAge <= 0 {
		return false
	}
	authTime, ok := GetAttribute(ctx, claimAuthTime).(float64)
	if !ok {
		return true
	}
	return now-int64(authTime) > maxAge
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

// WithSecurityContext attaches sc to ctx, so GetSubject, GetOUID, GetPermissions, and GetAttribute
// resolve for it. Exported for authenticated surfaces outside the REST gate (e.g. the MCP server)
// that build their own request context after calling BearerAuthenticator.Authenticate directly,
// rather than going through the securityService middleware that attaches it for REST requests.
func WithSecurityContext(ctx context.Context, sc *SecurityContext) context.Context {
	return withSecurityContext(ctx, sc)
}

// WithRecordedCaller rebuilds a caller recorded earlier in the same flow, for a step that caller started but
// does not send. The result is never a runtime context.
func WithRecordedCaller(ctx context.Context, subject, ouID string, permissions []string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, runtimeContextKey, false)
	sc := newSecurityContext(subject, ouID, "", permissions, nil)
	sc.recorded = true
	return withSecurityContext(ctx, sc)
}

// IsRecordedCaller reports whether the caller was rebuilt by WithRecordedCaller rather than presenting a
// credential on this request.
func IsRecordedCaller(ctx context.Context) bool {
	authCtx := getSecurityContext(ctx)
	return authCtx != nil && authCtx.recorded
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
