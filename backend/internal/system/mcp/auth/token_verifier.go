// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package auth provides authentication utilities for the MCP server.
package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/thunder-id/thunderid/internal/system/security"
)

// securityContextExtraKey is the key under which the authenticated security.SecurityContext is
// stored in TokenInfo.Extra. The go-sdk's TokenVerifier can only return a *TokenInfo — it has no
// way to attach anything else to the request context that RequireBearerToken hands to the next
// handler — so this is how the SecurityContext reaches the caller that mounts the guard (see
// mcp.DefaultGuard, which reads it back via SecurityContextFromTokenInfo).
const securityContextExtraKey = "securityContext"

// NewTokenVerifier creates a TokenVerifier that authenticates MCP requests using
// bearerAuthenticator — the same verification and revocation logic the REST API gate uses. This
// implements the auth.TokenVerifier function type from the MCP SDK.
func NewTokenVerifier(bearerAuthenticator *security.BearerAuthenticator) auth.TokenVerifier {
	return func(ctx context.Context, token string, req *http.Request) (*auth.TokenInfo, error) {
		securityCtx, err := bearerAuthenticator.Authenticate(ctx, token)
		if err != nil {
			return nil, auth.ErrInvalidToken
		}

		enrichedCtx := security.WithSecurityContext(ctx, securityCtx)

		var expiration time.Time
		if exp, ok := security.GetAttribute(enrichedCtx, "exp").(float64); ok {
			expiration = time.Unix(int64(exp), 0)
		}

		return &auth.TokenInfo{
			UserID:     security.GetSubject(enrichedCtx),
			Scopes:     security.GetPermissions(enrichedCtx),
			Expiration: expiration,
			Extra:      map[string]any{securityContextExtraKey: securityCtx},
		}, nil
	}
}

// SecurityContextFromTokenInfo returns the security.SecurityContext embedded in ti.Extra by the
// verifier built by NewTokenVerifier, or nil if ti is nil or carries none.
func SecurityContextFromTokenInfo(ti *auth.TokenInfo) *security.SecurityContext {
	if ti == nil {
		return nil
	}
	sc, _ := ti.Extra[securityContextExtraKey].(*security.SecurityContext)
	return sc
}
