// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
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
	security.InitSystemPermissions("")
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

// accessToken builds a self-issued access token carrying the system scope, so the go-sdk guard's
// own scope check passes and the request reaches the audience verification under test.
func accessToken() string {
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": jwt.TokenTypeAccessToken})
	payload, _ := json.Marshal(map[string]any{
		"sub":   "user123",
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"scope": security.GetSystemRootPermission(),
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
