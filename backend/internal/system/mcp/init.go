// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package mcp provides MCP (Model Context Protocol) server functionality.
package mcp

import (
	"context"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
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
	serveConfiguredTools(mcpServer, serverConfig())

	httpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return mcpServer
	}, &mcpsdk.StreamableHTTPOptions{Stateless: true})

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

	// The MCP resource identifier: the required token audience and the "resource" advertised below.
	// Both read this one variable so they cannot drift — advertising one identifier while enforcing
	// another rejects clients that followed the metadata correctly.
	mcpResourceIdentifier := baseURL + MCPEndpointPath
	if configured := cfg.Server.SecurityConfig.MCP.Audience; configured != nil {
		mcpResourceIdentifier = *configured
	}

	// Where the metadata document is served, not the resource's identity — always this server.
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
	bearerAuthenticator := security.NewBearerAuthenticator(jwtService, revocationEnforcer, mcpResourceIdentifier)
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
		Resource:             mcpResourceIdentifier,
		AuthorizationServers: []string{authServer},
		ScopesSupported:      []string{rootPerm},
	}

	return guard, metadata
}

// serverConfig is what the deployment configures the MCP server to serve, nothing when no runtime is
// initialized.
func serverConfig() config.MCPServerConfig {
	if !config.IsServerRuntimeInitialized() {
		return config.MCPServerConfig{}
	}
	return config.GetServerRuntime().Config.MCP
}

// serveConfiguredTools drops the tools the deployment excludes, then adds the upstreams' tools beside
// the ones left. It runs once every package has registered its tools, so the server's own tools are
// known when an upstream's are listed.
func serveConfiguredTools(mcpServer *mcpsdk.Server, cfg config.MCPServerConfig) {
	if len(cfg.ExcludedTools) > 0 {
		mcpServer.RemoveTools(cfg.ExcludedTools...)
	}
	if len(cfg.Upstreams) == 0 {
		return
	}
	endpoints := make([]string, 0, len(cfg.Upstreams))
	for _, upstream := range cfg.Upstreams {
		endpoints = append(endpoints, strings.TrimSpace(upstream.URL))
	}
	mcpServer.AddReceivingMiddleware(newUpstreams(endpoints, localToolNames(mcpServer)).middleware)
}

// localToolNames lists the server's own tools by asking it, over an in-memory session.
func localToolNames(mcpServer *mcpsdk.Server) map[string]bool {
	ctx := context.Background()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "MCPServer"))
	names := map[string]bool{}
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	serverSession, err := mcpServer.Connect(ctx, serverTransport, nil)
	if err != nil {
		logger.Error(ctx, "Failed to read the MCP server's own tools", log.Error(err))
		return names
	}
	defer func() { _ = serverSession.Close() }()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "thunderid-mcp-probe", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		logger.Error(ctx, "Failed to read the MCP server's own tools", log.Error(err))
		return names
	}
	defer func() { _ = session.Close() }()
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			logger.Error(ctx, "Failed to read the MCP server's own tools", log.Error(err))
			return names
		}
		names[tool.Name] = true
	}
	return names
}
