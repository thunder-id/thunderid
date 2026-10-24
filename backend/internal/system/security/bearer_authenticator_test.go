// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

// TestBearerAuthenticator_ChecksTrustedIssuerTokenAgainstTheAuthenticatorAudience locks in that the
// audience a BearerAuthenticator is built with reaches the trusted-issuer verification, in place of
// trusted_issuer.audience, which is the REST API's audience.
func TestBearerAuthenticator_ChecksTrustedIssuerTokenAgainstTheAuthenticatorAudience(t *testing.T) {
	const mcpAudience = "https://resource-server:9443/mcp"
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{
		JWT: engineconfig.JWTConfig{Issuer: "https://localhost:8090"},
		Server: engineconfig.ServerConfig{SecurityConfig: engineconfig.SecurityConfig{
			TrustedIssuer: engineconfig.TrustedIssuerConfig{
				Issuer:   testFederatedIssuer,
				JWKSURL:  testFederatedJWKSURL,
				Audience: testResourceServerAudURL,
			},
		}},
	})
	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "test-kid"},
		map[string]interface{}{"sub": "user-1", "iss": testFederatedIssuer, "jti": "jti-2"},
	)
	mockJWT := jwtmock.NewJWTServiceInterfaceMock(t)
	mockJWT.On("VerifyJWTWithJWKS", mock.Anything, token, testFederatedJWKSURL, mcpAudience, testFederatedIssuer).
		Return(nil).Once()
	mockEnforcer := NewRevocationEnforcerInterfaceMock(t)
	mockEnforcer.On("EnsureNotRevoked", mock.Anything, mock.Anything).Return(nil).Once()

	securityCtx, err := NewBearerAuthenticator(mockJWT, mockEnforcer, mcpAudience).
		Authenticate(context.Background(), token)

	assert.NoError(t, err)
	assert.NotNil(t, securityCtx)
}
