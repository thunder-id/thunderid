// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package mcp provides MCP (Model Context Protocol) server functionality.
package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	mcpauth "github.com/thunder-id/thunderid/internal/system/mcp/auth"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// NewServer creates the MCP server so other packages initializing during startup can register
// tools on it. Call this first; call Initialize once a guard is available to actually mount its
// HTTP routes (which may depend on services, such as token revocation, that aren't ready this
// early in startup).
func NewServer() *mcpsdk.Server {
	return newServer()
}

// Initialize mounts mcpServer's routes on mux, securing them with the given guard. resourceMeta, if
// non-nil, is published at OAuthProtectedResourceMetadataPath for MCP client discovery; callers
// that pass a guard with no discoverable authorization server (e.g. one that accepts every
// request) should pass nil.
func Initialize(
	mux *http.ServeMux,
	mcpServer *mcpsdk.Server,
	guard func(http.Handler) http.Handler,
	resourceMeta *oauthex.ProtectedResourceMetadata,
) {
	httpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return mcpServer
	}, nil)

	securedHandler := guard(httpHandler)

	// Register protected resource metadata endpoint, if the guard has an authorization server to
	// advertise.
	if resourceMeta != nil {
		mux.Handle(OAuthProtectedResourceMetadataPath, auth.ProtectedResourceMetadataHandler(resourceMeta))
	}

	// Register MCP routes
	mux.Handle(MCPEndpointPath, securedHandler)
	mux.Handle(MCPEndpointPath+"/", securedHandler)
}

// DefaultGuard builds the bearer-token guard and resource metadata for a binary that runs its own
// OAuth2 authorization server: requests must present a JWT issued by this server's own token
// endpoint, and must not be revoked. It authenticates using the exact same verification and
// revocation logic as the REST API gate (security.BearerAuthenticator), so a token that is
// rejected by one is rejected by the other for the same reason.
func DefaultGuard(jwtService jwt.JWTServiceInterface, revocationEnforcer security.RevocationEnforcerInterface,
) (func(http.Handler) http.Handler, *oauthex.ProtectedResourceMetadata) {
	cfg := config.GetServerRuntime().Config
	baseURL := config.GetServerURL(&cfg.Server)

	mcpURL := baseURL + MCPEndpointPath
	resourceMetadataURL := baseURL + OAuthProtectedResourceMetadataPath
	rootPerm := security.GetSystemRootPermission()

	// When a trusted issuer is configured, BearerAuthenticator already accepts tokens from it
	// (routed to JWKS verification in jwtAuthenticator.verifyToken) alongside self-issued ones. MCP
	// client discovery (RFC 9728) must point at that same issuer, not this server's own — otherwise
	// a client following the metadata is sent to authenticate against the wrong authorization
	// server.
	authServer := cfg.JWT.Issuer
	if trustedIssuer := cfg.Server.SecurityConfig.TrustedIssuer; trustedIssuer.IsConfigured() {
		authServer = trustedIssuer.Issuer
	}

	// Self-issued tokens must be scoped to this MCP resource specifically (RFC 8707 resource
	// indicator) — a token minted for some other purpose does not authenticate here just because
	// it happens to carry the required scope.
	bearerAuthenticator := security.NewBearerAuthenticator(jwtService, revocationEnforcer, mcpURL)
	tokenVerifier := mcpauth.NewTokenVerifier(bearerAuthenticator)
	sdkGuard := auth.RequireBearerToken(tokenVerifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: resourceMetadataURL,
		Scopes:              []string{rootPerm},
	})

	// The go-sdk's TokenVerifier can only return a *auth.TokenInfo — it has no way to attach
	// anything else to the request context that reaches the next handler, since RequireBearerToken
	// controls that itself. tokenVerifier stashes the authenticated security.SecurityContext in
	// TokenInfo.Extra to get it out; this wraps the SDK's guard to pull it back out and attach it
	// via security.WithSecurityContext, so MCP tool handlers see the same SecurityContext a REST
	// handler would for an equivalent token.
	guard := func(next http.Handler) http.Handler {
		return sdkGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ti := auth.TokenInfoFromContext(r.Context()); ti != nil {
				if secCtx := mcpauth.SecurityContextFromTokenInfo(ti); secCtx != nil {
					r = r.WithContext(security.WithSecurityContext(r.Context(), secCtx))
				}
			}
			next.ServeHTTP(w, r)
		}))
	}

	metadata := &oauthex.ProtectedResourceMetadata{
		Resource:             mcpURL,
		AuthorizationServers: []string{authServer},
		ScopesSupported:      []string{rootPerm},
	}

	return guard, metadata
}
