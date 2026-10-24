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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/security"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testMCPIdentifier = "https://localhost:8090/mcp"
	testTrustedIssuer = "https://trusted.example.com"
)

// fakeRevocationEnforcer accepts every identity; security.RevocationEnforcerInterface has no
// generated mock exported outside the security package.
type fakeRevocationEnforcer struct{}

func (*fakeRevocationEnforcer) EnsureNotRevoked(context.Context, security.RevocationIdentity) error {
	return nil
}

// TestDefaultGuard_TrustedIssuerTokenIsCheckedAgainstTheMCPIdentifier locks in that, with a trusted
// issuer configured, the audience required of its tokens is the advertised MCP resource identifier
// and not trusted_issuer.audience, which belongs to the REST API.
func TestDefaultGuard_TrustedIssuerTokenIsCheckedAgainstTheMCPIdentifier(t *testing.T) {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	security.InitSystemPermissions("")
	cfg := &config.Config{}
	cfg.Server.Hostname = "localhost"
	cfg.Server.Port = 8090
	cfg.Server.SecurityConfig.TrustedIssuer = engineconfig.TrustedIssuerConfig{
		Issuer:   testTrustedIssuer,
		JWKSURL:  testTrustedIssuer + "/oauth2/jwks",
		Audience: "https://localhost:8090",
	}
	assert.NoError(t, config.InitializeServerRuntime("", cfg))

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(t)
	guard, metadata := DefaultGuard(mockJWT, &fakeRevocationEnforcer{})

	header, _ := json.Marshal(map[string]any{"alg": "RS256"})
	payload, _ := json.Marshal(map[string]any{
		"sub":   "user123",
		"iss":   testTrustedIssuer,
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
		"scope": security.GetSystemRootPermission(),
	})
	token := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	var observed string
	mockJWT.On("VerifyJWTWithJWKS", mock.Anything, token, cfg.Server.SecurityConfig.TrustedIssuer.JWKSURL,
		mock.Anything, testTrustedIssuer).
		Run(func(args mock.Arguments) { observed = args.String(3) }).Return(nil)

	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	guard(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, []string{testTrustedIssuer}, metadata.AuthorizationServers)
	assert.Equal(t, testMCPIdentifier, metadata.Resource)
	assert.Equal(t, testMCPIdentifier, observed)
}
