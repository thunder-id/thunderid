// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/security"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testHostname = "localhost"
	testPort     = 8090
	// testDerivedIdentifier is what DefaultGuard derives from the server config above when
	// server.security.mcp.audience is unset.
	testDerivedIdentifier = "https://localhost:8090/mcp"
	// testConfiguredIdentifier is deliberately unrelated to the server's own URL, so a test using
	// it cannot pass by accidentally falling back to the derived value.
	testConfiguredIdentifier = "https://id.example.com/mcp"
	testProtocolVersion      = "2026-07-28"
	testWaitTimeout          = 5 * time.Second
)

// fakeRevocationEnforcer accepts every identity; security.RevocationEnforcerInterface has no
// generated mock exported outside the security package.
type fakeRevocationEnforcer struct{}

func (*fakeRevocationEnforcer) EnsureNotRevoked(context.Context, security.RevocationIdentity) error {
	return nil
}

type DefaultGuardTestSuite struct {
	suite.Suite
	mockJWT *jwtmock.JWTServiceInterfaceMock
}

func TestDefaultGuardTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultGuardTestSuite))
}

func (suite *DefaultGuardTestSuite) SetupTest() {
	suite.mockJWT = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	security.InitControlPlanePermissions("")
}

func (suite *DefaultGuardTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// initRuntime installs a server runtime whose MCP audience is audience, or unset when nil.
func (suite *DefaultGuardTestSuite) initRuntime(audience *string) {
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.Server.Hostname = testHostname
	cfg.Server.Port = testPort
	cfg.Server.SecurityConfig.MCP = engineconfig.MCPConfig{Audience: audience}
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))
}

// accessToken builds a self-issued access token carrying the system scope, so the guard admits the
// caller and the request reaches the audience verification under test.
func accessToken() string {
	return accessTokenWithScope(security.GetSystemRootPermission())
}

func accessTokenWithScope(scope string) string {
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": jwt.TokenTypeAccessToken})
	payload, _ := json.Marshal(map[string]any{
		"sub":   "user123",
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"scope": scope,
	})
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

// audienceReachingVerification drives a request through the guard and reports the audience the
// guard actually required, as observed by the JWT service it delegates to.
func (suite *DefaultGuardTestSuite) audienceReachingVerification(
	guard func(http.Handler) http.Handler, token string,
) string {
	var observed string
	suite.mockJWT.On("VerifyJWT", mock.Anything, token, mock.Anything, "").
		Run(func(args mock.Arguments) { observed = args.String(2) }).Return(nil)

	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	guard(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(httptest.NewRecorder(), req)

	return observed
}

// TestDefaultGuard_UnconfiguredAudienceUsesDerivedIdentifier locks in that omitting
// server.security.mcp.audience keeps today's behavior: the identifier derived from the server URL.
func (suite *DefaultGuardTestSuite) TestDefaultGuard_UnconfiguredAudienceUsesDerivedIdentifier() {
	suite.initRuntime(nil)

	guard, metadata := DefaultGuard(suite.mockJWT, &fakeRevocationEnforcer{})

	suite.Equal(testDerivedIdentifier, metadata.Resource)
	suite.Equal(testDerivedIdentifier, suite.audienceReachingVerification(guard, accessToken()))
}

// The endpoint admits any caller holding a system permission, since each tool call is checked against
// its own permission; a caller holding none is refused. On the data plane the organization tree counts
// as well; the control plane has none.
func (suite *DefaultGuardTestSuite) TestDefaultGuard_AdmitsAnySystemPermission() {
	statuses := map[string]map[string]int{
		"ControlPlane": {
			"system:user:view": http.StatusOK,
			"org:user:view":    http.StatusForbidden,
			"orders:read":      http.StatusForbidden,
		},
		"DataPlane": {
			"system":        http.StatusOK,
			"org":           http.StatusOK,
			"org:user:view": http.StatusOK,
			"orders:read":   http.StatusForbidden,
		},
	}
	suite.initRuntime(nil)
	guard, _ := DefaultGuard(suite.mockJWT, &fakeRevocationEnforcer{})

	security.ForEachPermissionPlaneForTest(func(plane string) {
		suite.Run(plane, func() {
			for scope, status := range statuses[plane] {
				token := accessTokenWithScope(scope)
				suite.mockJWT.On("VerifyJWT", mock.Anything, token, mock.Anything, "").Return(nil).Maybe()

				req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				rec := httptest.NewRecorder()
				guard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })).
					ServeHTTP(rec, req)

				suite.Equal(status, rec.Code, scope)
				if status == http.StatusForbidden {
					challenge := rec.Header().Get("WWW-Authenticate")
					suite.Contains(challenge, `error="insufficient_scope"`)
					suite.Contains(challenge, `resource_metadata="`)
					suite.NotContains(challenge, "scope=\"system\"")
				}
			}
		})
	})
}

// The REST gate marks an anonymous request on the public /mcp path as a runtime caller; the guard
// still asks it for a bearer token rather than letting it reach a tool.
func (suite *DefaultGuardTestSuite) TestDefaultGuard_RefusesAnAnonymousRuntimeCaller() {
	suite.initRuntime(nil)
	guard, _ := DefaultGuard(suite.mockJWT, &fakeRevocationEnforcer{})

	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, nil)
	req = req.WithContext(security.WithRuntimeContext(req.Context()))
	rec := httptest.NewRecorder()
	reached := false
	guard(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })).ServeHTTP(rec, req)

	suite.False(reached)
	suite.Equal(http.StatusUnauthorized, rec.Code)
	suite.Contains(rec.Header().Get("WWW-Authenticate"), `resource_metadata="`)
}

// TestDefaultGuard_ConfiguredAudienceDrivesGuardAndMetadata is the point of the MCP audience being
// configurable at all: the enforced audience and the RFC 9728 "resource" the server advertises must
// be the same value. If they diverge, a spec-compliant client requests a token for the advertised
// resource and is rejected by a guard expecting something else.
func (suite *DefaultGuardTestSuite) TestDefaultGuard_ConfiguredAudienceDrivesGuardAndMetadata() {
	audience := testConfiguredIdentifier
	suite.initRuntime(&audience)

	guard, metadata := DefaultGuard(suite.mockJWT, &fakeRevocationEnforcer{})

	suite.Equal(testConfiguredIdentifier, metadata.Resource)
	suite.Equal(testConfiguredIdentifier, suite.audienceReachingVerification(guard, accessToken()))
}

// An MCP request answers after the server's write timeout has passed when the import or apply time
// allows it, since a tool call can run either; with neither configured the server's timeout applies.
func TestToolWriteDeadlineOutlastsTheServerWriteTimeout(t *testing.T) {
	get := func(applySeconds int) (string, error) {
		config.ResetServerRuntime()
		cfg := &config.Config{}
		cfg.Gateway.ApplyTimeoutSeconds = applySeconds
		require.NoError(t, config.InitializeServerRuntime("", cfg))
		defer config.ResetServerRuntime()
		server := httptest.NewUnstartedServer(withToolWriteDeadline(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(300 * time.Millisecond)
				_, _ = w.Write([]byte("done"))
			})))
		server.Config.WriteTimeout = 100 * time.Millisecond
		server.Start()
		defer server.Close()
		resp, err := server.Client().Get(server.URL)
		if err != nil {
			return "", err
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		return string(body), err
	}

	body, err := get(3600)
	require.NoError(t, err)
	require.Equal(t, "done", body)

	_, err = get(0)
	require.Error(t, err)
}

// blockingCall observes one blocking tool registered by addBlockingTool.
type blockingCall struct {
	started     chan struct{}
	canceled    chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
}

// unblock lets the tool return. It is safe to call more than once.
func (c *blockingCall) unblock() {
	c.releaseOnce.Do(func() { close(c.release) })
}

// addBlockingTool registers a tool that runs until its request is cancelled or the test releases
// it. It stands in for a long-running tool, since ThunderID's own tools are short service calls.
// Callers defer unblock: httptest.Server.Close waits for running handlers, so a test that fails
// before the tool returns would otherwise hang instead of failing.
func addBlockingTool(server *mcpsdk.Server, name string) *blockingCall {
	call := &blockingCall{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
		release:  make(chan struct{}),
	}
	server.AddTool(&mcpsdk.Tool{Name: name, InputSchema: map[string]any{"type": "object"}},
		func(ctx context.Context, _ *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			close(call.started)
			select {
			case <-ctx.Done():
				close(call.canceled)
				return nil, ctx.Err()
			case <-call.release:
				return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "released"}}}, nil
			}
		})
	return call
}

// serveMCP mounts server through Initialize behind a guard that admits every request as a root caller,
// so the tool permission check passes, and serves it over a real listener so a client can close the
// connection mid-request. It installs a server runtime, which the mounted handler reads on every request.
func (suite *DefaultGuardTestSuite) serveMCP(server *mcpsdk.Server) *httptest.Server {
	suite.initRuntime(nil)
	mux := http.NewServeMux()
	rootCaller := security.NewSecurityContextForTest("user-1", "ou-1", "",
		[]string{security.GetSystemRootPermission()}, nil)
	Initialize(mux, server, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(security.WithSecurityContextTest(r.Context(), rootCaller)))
		})
	}, nil)
	httpServer := httptest.NewServer(mux)
	suite.T().Cleanup(httpServer.Close)
	return httpServer
}

// toolCallRequest builds a stateless 2026-07-28 tools/call request for tool. Canceling ctx closes the
// connection, and with it the request's response stream.
func (suite *DefaultGuardTestSuite) toolCallRequest(
	ctx context.Context, httpServer *httptest.Server, tool string,
) *http.Request {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"_meta": map[string]any{
				"io.modelcontextprotocol/protocolVersion":    testProtocolVersion,
				"io.modelcontextprotocol/clientCapabilities": map[string]any{},
				"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "test", "version": "1.0.0"},
			},
			"name":      tool,
			"arguments": map[string]any{},
		},
	})
	suite.Require().NoError(err)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, httpServer.URL+MCPEndpointPath,
		bytes.NewReader(payload))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", testProtocolVersion)
	req.Header.Set("Mcp-Method", "tools/call")
	req.Header.Set("Mcp-Name", tool)
	return req
}

// toolCallResponse is what sendAsync observed for one tools/call request.
type toolCallResponse struct {
	status int
	body   string
	err    error
}

// sendAsync sends req on its own goroutine, since a call to a blocking tool does not return until
// the tool does.
func sendAsync(httpServer *httptest.Server, req *http.Request) <-chan toolCallResponse {
	responses := make(chan toolCallResponse, 1)
	go func() {
		resp, err := httpServer.Client().Do(req)
		if err != nil {
			responses <- toolCallResponse{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		responses <- toolCallResponse{status: resp.StatusCode, body: string(body), err: err}
	}()
	return responses
}

// waitFor fails the test with failure unless ch closes within testWaitTimeout.
func (suite *DefaultGuardTestSuite) waitFor(ch <-chan struct{}, failure string) {
	select {
	case <-ch:
	case <-time.After(testWaitTimeout):
		suite.FailNow(failure)
	}
}

// TestInitialize_ClosedResponseStreamCancelsToolCall covers MCP 2026-07-28 Streamable HTTP
// cancellation: a client closing a request's response stream cancels the request.
func (suite *DefaultGuardTestSuite) TestInitialize_ClosedResponseStreamCancelsToolCall() {
	server := newServer("test")
	call := addBlockingTool(server, "block")
	defer call.unblock()
	httpServer := suite.serveMCP(server)

	ctx, cancel := context.WithCancel(context.Background())
	responses := sendAsync(httpServer, suite.toolCallRequest(ctx, httpServer, "block"))
	suite.waitFor(call.started, "tool call did not start")

	cancel()

	suite.waitFor(call.canceled, "closing the response stream did not cancel the tool call")
	suite.Error((<-responses).err)
}

func (suite *DefaultGuardTestSuite) TestInitialize_ClosedResponseStreamLeavesOtherCallsRunning() {
	server := newServer("test")
	closedCall := addBlockingTool(server, "closed")
	openCall := addBlockingTool(server, "open")
	defer closedCall.unblock()
	defer openCall.unblock()
	httpServer := suite.serveMCP(server)

	ctx, cancel := context.WithCancel(context.Background())
	closedResponses := sendAsync(httpServer, suite.toolCallRequest(ctx, httpServer, "closed"))
	openResponses := sendAsync(httpServer, suite.toolCallRequest(context.Background(), httpServer, "open"))
	suite.waitFor(closedCall.started, "closed tool call did not start")
	suite.waitFor(openCall.started, "open tool call did not start")

	cancel()
	suite.waitFor(closedCall.canceled, "closing the response stream did not cancel the tool call")
	<-closedResponses

	openCall.unblock()
	response := <-openResponses
	suite.Require().NoError(response.err)
	suite.Equal(http.StatusOK, response.status)
	suite.Contains(response.body, `"released"`)
}

// A token from the trusted issuer reaches a tool only when it was issued for the MCP resource the
// metadata advertises, not for the audience the REST API accepts (thunder-id/thunderid#6037).
func (suite *DefaultGuardTestSuite) TestDefaultGuard_ChecksATrustedIssuerTokenAgainstTheMCPResource() {
	const issuer, restAudience = "https://a.example.com", "https://localhost:8090"
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.Server.Hostname = testHostname
	cfg.Server.Port = testPort
	cfg.Server.SecurityConfig.TrustedIssuer = engineconfig.TrustedIssuerConfig{
		Issuer: issuer, JWKSURL: issuer + "/oauth2/jwks", Audience: restAudience,
	}
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))

	header, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": jwt.TokenTypeAccessToken})
	payload, _ := json.Marshal(map[string]any{
		"sub": "user123", "iss": issuer, "exp": float64(time.Now().Add(time.Hour).Unix()),
		"scope": security.GetSystemRootPermission(),
	})
	token := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	var observed string
	suite.mockJWT.On("VerifyJWTWithJWKS", mock.Anything, token, issuer+"/oauth2/jwks", mock.Anything, issuer).
		Run(func(args mock.Arguments) { observed = args.String(3) }).Return(nil)

	guard, metadata := DefaultGuard(suite.mockJWT, &fakeRevocationEnforcer{})
	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	reached := false
	guard(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true })).
		ServeHTTP(httptest.NewRecorder(), req)

	suite.True(reached)
	suite.Equal(testDerivedIdentifier, metadata.Resource)
	suite.Equal(metadata.Resource, observed, "the token must be checked against the advertised resource")
}
