// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

// selfIssuedBearerToken returns a self-issued access token carrying every revocation claim.
func selfIssuedBearerToken(issuedAt time.Time) string {
	return buildFakeJWT(accessTokenHeader(), map[string]interface{}{
		"iss":       testLocalIssuer,
		"sub":       "user-1",
		"jti":       "jti-1",
		"tfid":      "tfid-1",
		"client_id": "app-1",
		"aud":       []interface{}{testResourceIdentifier},
		"scope":     "read:docs write:docs",
		"iat":       float64(issuedAt.Unix()),
		"exp":       float64(issuedAt.Add(time.Hour).Unix()),
	})
}

func initLocalIssuerRuntime(t *testing.T) {
	config.ResetServerRuntime()
	assert.NoError(t, config.InitializeServerRuntime("", &config.Config{
		JWT: engineconfig.JWTConfig{Issuer: testLocalIssuer},
	}))
	t.Cleanup(config.ResetServerRuntime)
}

func TestBearerAuthenticator_EnforcesEveryRevocationDimension(t *testing.T) {
	initLocalIssuerRuntime(t)
	issuedAt := time.Unix(1_700_000_000, 0).UTC()
	token := selfIssuedBearerToken(issuedAt)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(t)
	mockJWT.On("VerifyJWT", mock.Anything, token, testResourceIdentifier, "").Return(nil)
	enforcer := NewRevocationEnforcerInterfaceMock(t)
	enforcer.On("EnsureNotRevoked", mock.Anything, RevocationIdentity{
		JTI:           "jti-1",
		TokenFamilyID: "tfid-1",
		Subject:       "user-1",
		AppKey:        "app-1",
		Audience:      testResourceIdentifier,
		Scopes:        []string{"read:docs", "write:docs"},
		EstablishedAt: issuedAt,
	}).Return(nil).Once()

	securityCtx, err := NewBearerAuthenticator(mockJWT, enforcer, testResourceIdentifier).
		Authenticate(context.Background(), token)

	assert.NoError(t, err)
	assert.NotNil(t, securityCtx)
}

func TestBearerAuthenticator_RejectsTokenTheEnforcerReportsRevoked(t *testing.T) {
	initLocalIssuerRuntime(t)
	token := selfIssuedBearerToken(time.Now())

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(t)
	mockJWT.On("VerifyJWT", mock.Anything, token, testResourceIdentifier, "").Return(nil)
	enforcer := NewRevocationEnforcerInterfaceMock(t)
	enforcer.On("EnsureNotRevoked", mock.Anything, mock.MatchedBy(func(identity RevocationIdentity) bool {
		return identity.Audience == testResourceIdentifier && identity.AppKey == "app-1"
	})).Return(errors.New("scope revoked")).Once()

	securityCtx, err := NewBearerAuthenticator(mockJWT, enforcer, testResourceIdentifier).
		Authenticate(context.Background(), token)

	assert.ErrorIs(t, err, errInvalidToken)
	assert.Nil(t, securityCtx)
}
