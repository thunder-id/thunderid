// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package introspect

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type TokenIntrospectionServiceTestSuite struct {
	suite.Suite
	tokenValidatorMock *tokenservicemock.TokenValidatorInterfaceMock
	introspectService  TokenIntrospectionServiceInterface
}

func TestTokenIntrospectionServiceTestSuite(t *testing.T) {
	suite.Run(t, new(TokenIntrospectionServiceTestSuite))
}

func (s *TokenIntrospectionServiceTestSuite) SetupTest() {
	s.tokenValidatorMock = tokenservicemock.NewTokenValidatorInterfaceMock(s.T())
	s.introspectService = newTokenIntrospectionService(s.tokenValidatorMock)
}

func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_EmptyToken() {
	response, err := s.introspectService.IntrospectToken(context.Background(), "", "", "client123")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "token is required")
	assert.Nil(s.T(), response)
}

// A valid token is reported active with its claims surfaced in the response.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_ValidToken_Active() {
	claims := map[string]interface{}{
		"jti":       "token-id-123",
		"scope":     "openid profile",
		"client_id": "client123",
		"username":  "user@example.com",
		"sub":       "user123",
		"aud":       "api.example.com",
		"iss":       "https://example.com",
	}
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "valid-token").Return(claims, nil)

	response, err := s.introspectService.IntrospectToken(context.Background(), "valid-token", "", "client123")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), constants.TokenTypeBearer, response.TokenType)
	assert.Equal(s.T(), "openid profile", response.Scope)
	assert.Equal(s.T(), "client123", response.ClientID)
	assert.Equal(s.T(), "user@example.com", response.Username)
	assert.Equal(s.T(), "user123", response.Sub)
	assert.Equal(s.T(), "api.example.com", response.Aud)
	assert.Equal(s.T(), "https://example.com", response.Iss)
	assert.Equal(s.T(), "token-id-123", response.Jti)
}

// An array audience claim is surfaced as a string slice.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_ArrayAudience() {
	claims := map[string]interface{}{
		"aud": []interface{}{"api.example.com", "api2.example.com"},
	}
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "array-aud-token").Return(claims, nil)

	response, err := s.introspectService.IntrospectToken(context.Background(), "array-aud-token", "", "api.example.com")

	assert.NoError(s.T(), err)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), []string{"api.example.com", "api2.example.com"}, response.Aud)
}

// A valid token missing optional claims is still active, with empty optional fields. The token keeps
// the subject that makes it attributable to the caller, since a token naming no client, subject or
// audience belongs to nobody and is reported inactive.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_MissingOptionalClaims_Active() {
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "sparse-token").
		Return(map[string]interface{}{"sub": "client123"}, nil)

	response, err := s.introspectService.IntrospectToken(context.Background(), "sparse-token", "", "client123")

	assert.NoError(s.T(), err)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), constants.TokenTypeBearer, response.TokenType)
	assert.Empty(s.T(), response.Scope)
	assert.Empty(s.T(), response.ClientID)
	assert.Empty(s.T(), response.Jti)
}

// An invalid token (bad signature, expired, malformed, …) is reported inactive per RFC 7662.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_InvalidToken_IsInactive() {
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "invalid-token").
		Return(nil, errors.New("token verification failed"))

	response, err := s.introspectService.IntrospectToken(context.Background(), "invalid-token", "", "client123")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.False(s.T(), response.Active)
}

// A revoked but otherwise valid token is reported inactive (RFC 7009 deny-list enforcement).
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_RevokedToken_IsInactive() {
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "revoked-token").
		Return(nil, revocation.ErrTokenRevoked)

	response, err := s.introspectService.IntrospectToken(context.Background(), "revoked-token", "", "client123")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.False(s.T(), response.Active)
}

// When the deny list cannot be consulted, introspection fails closed with a server error rather
// than asserting the token is active.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_EnforcementUnavailable_FailsClosed() {
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "some-token").
		Return(nil, revocation.ErrEnforcementUnavailable)

	response, err := s.introspectService.IntrospectToken(context.Background(), "some-token", "", "client123")

	assert.Error(s.T(), err)
	assert.Nil(s.T(), response)
}

// A token carrying cnf.jkt is reported with token_type=DPoP and the cnf claim is surfaced.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_DPoPBoundToken_SurfacesCnfAndDPoPType() {
	claims := map[string]interface{}{
		"sub":       "user123",
		"client_id": "client123",
		"cnf":       map[string]interface{}{"jkt": "thumbprint-abc"},
	}
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "dpop-token").Return(claims, nil)

	response, err := s.introspectService.IntrospectToken(context.Background(), "dpop-token", "", "client123")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), constants.TokenTypeDPoP, response.TokenType)
	assert.NotNil(s.T(), response.Cnf)
	assert.Equal(s.T(), "thumbprint-abc", response.Cnf.Jkt)
}

// A client that is not a party to the token gets {"active": false} rather than the token's metadata,
// so the endpoint cannot be used to enumerate other clients' tokens.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_ForeignClient_IsInactive() {
	claims := map[string]interface{}{
		"jti":       "token-id-123",
		"scope":     "openid profile",
		"client_id": "client123",
		"sub":       "user123",
		"aud":       "api.example.com",
	}
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "other-client-token").Return(claims, nil)

	response, err := s.introspectService.IntrospectToken(
		context.Background(), "other-client-token", "", "intruder-client")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.False(s.T(), response.Active)
	assert.Empty(s.T(), response.Sub)
	assert.Empty(s.T(), response.Scope)
	assert.Empty(s.T(), response.Jti)
}

// A refresh token carries no client_id and names the client as its subject, so the issuing client
// still introspects it successfully.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_RefreshTokenSubject_IsActive() {
	claims := map[string]interface{}{
		"sub":              "client123",
		"aud":              "https://example.com",
		"access_token_sub": "user123",
	}
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "refresh-token").Return(claims, nil)

	response, err := s.introspectService.IntrospectToken(context.Background(), "refresh-token", "", "client123")

	assert.NoError(s.T(), err)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), "client123", response.Sub)
}

// A refresh token belonging to another client is inactive for the caller.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_ForeignRefreshToken_IsInactive() {
	claims := map[string]interface{}{
		"sub": "client123",
		"aud": "https://example.com",
	}
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "foreign-refresh-token").Return(claims, nil)

	response, err := s.introspectService.IntrospectToken(
		context.Background(), "foreign-refresh-token", "", "intruder-client")

	assert.NoError(s.T(), err)
	assert.False(s.T(), response.Active)
}

// A resource server the token is audienced to is a party to it, so it may introspect even though the
// token was issued to a different client.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_AudienceMember_IsActive() {
	claims := map[string]interface{}{
		"client_id": "client123",
		"sub":       "user123",
		"aud":       "https://api.example.com",
	}
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "rs-token").Return(claims, nil)

	response, err := s.introspectService.IntrospectToken(
		context.Background(), "rs-token", "", "https://api.example.com")

	assert.NoError(s.T(), err)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), "client123", response.ClientID)
}

// The audience check also covers a multi valued aud claim.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_AudienceArrayMember_IsActive() {
	claims := map[string]interface{}{
		"client_id": "client123",
		"aud":       []interface{}{"https://api.example.com", "https://api2.example.com"},
	}
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "multi-aud-token").Return(claims, nil)

	response, err := s.introspectService.IntrospectToken(
		context.Background(), "multi-aud-token", "", "https://api2.example.com")

	assert.NoError(s.T(), err)
	assert.True(s.T(), response.Active)
}

// A token naming no client, subject or audience is attributable to nobody and stays inactive.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_UnattributableToken_IsInactive() {
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "anonymous-token").
		Return(map[string]interface{}{"scope": "openid"}, nil)

	response, err := s.introspectService.IntrospectToken(
		context.Background(), "anonymous-token", "", "client123")

	assert.NoError(s.T(), err)
	assert.False(s.T(), response.Active)
}

// An unauthenticated caller reaching the service with no client identity introspects nothing.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_NoCallerClientID_IsInactive() {
	claims := map[string]interface{}{
		"client_id": "client123",
		"sub":       "user123",
	}
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "unattributed-caller-token").Return(claims, nil)

	response, err := s.introspectService.IntrospectToken(
		context.Background(), "unattributed-caller-token", "", "")

	assert.NoError(s.T(), err)
	assert.False(s.T(), response.Active)
}
