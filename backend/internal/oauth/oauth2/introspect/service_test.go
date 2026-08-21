// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package introspect

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
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

// tokenWithTyp builds a syntactically valid JWT whose typ header selects the validator the service
// routes to. The payload and signature are never inspected here because the validator is mocked;
// only the header matters for routing. The id keeps each token string distinct so mock expectations
// on different tokens do not collide.
func tokenWithTyp(typ, id string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"` + typ + `"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"id":"` + id + `"}`))
	return header + "." + payload + ".signature"
}

// accessTokenFor returns a token routed to the access-token validator (RFC 9068 at+jwt).
func accessTokenFor(id string) string { return tokenWithTyp(jwt.TokenTypeAccessToken, id) }

// genericTokenFor returns a token carrying the generic JWT typ, which refresh tokens share with ID
// tokens and flow assertions; the refresh validator's claim checks separate them.
func genericTokenFor(id string) string { return tokenWithTyp(jwt.TokenTypeJWT, id) }

// stubAccessToken makes the token resolve as a valid access token carrying the given raw claims.
func (s *TokenIntrospectionServiceTestSuite) stubAccessToken(token string, claims map[string]interface{}) {
	s.tokenValidatorMock.On("ValidateAccessToken", mock.Anything, token).
		Return(&tokenservice.AccessTokenClaims{Claims: claims}, nil)
}

// stubAccessTokenError makes an at+jwt fixture fail validation. Only the access-token validator is
// stubbed because the typ header routes the token there and no fallback runs.
func (s *TokenIntrospectionServiceTestSuite) stubAccessTokenError(token string, err error) {
	s.tokenValidatorMock.On("ValidateAccessToken", mock.Anything, token).Return(nil, err)
}

func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_EmptyToken() {
	response, err := s.introspectService.IntrospectToken(context.Background(), "", "")
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
	s.stubAccessToken(accessTokenFor("valid-token"), claims)

	response, err := s.introspectService.IntrospectToken(context.Background(), accessTokenFor("valid-token"), "")

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
	s.stubAccessToken(accessTokenFor("array-aud-token"), claims)

	response, err := s.introspectService.IntrospectToken(context.Background(), accessTokenFor("array-aud-token"), "")

	assert.NoError(s.T(), err)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), []string{"api.example.com", "api2.example.com"}, response.Aud)
}

// A valid token missing optional claims is still active, with empty optional fields.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_MissingOptionalClaims_Active() {
	s.stubAccessToken(accessTokenFor("sparse-token"), map[string]interface{}{})

	response, err := s.introspectService.IntrospectToken(context.Background(), accessTokenFor("sparse-token"), "")

	assert.NoError(s.T(), err)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), constants.TokenTypeBearer, response.TokenType)
	assert.Empty(s.T(), response.Scope)
	assert.Empty(s.T(), response.ClientID)
	assert.Empty(s.T(), response.Sub)
	assert.Empty(s.T(), response.Jti)
}

// An invalid token (bad signature, expired, malformed, …) is reported inactive per RFC 7662.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_InvalidToken_IsInactive() {
	s.stubAccessTokenError(accessTokenFor("invalid-token"), errors.New("token verification failed"))

	response, err := s.introspectService.IntrospectToken(context.Background(), accessTokenFor("invalid-token"), "")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.False(s.T(), response.Active)
}

// A revoked but otherwise valid token is reported inactive (RFC 7009 deny-list enforcement).
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_RevokedToken_IsInactive() {
	s.stubAccessTokenError(accessTokenFor("revoked-token"), revocation.ErrTokenRevoked)

	response, err := s.introspectService.IntrospectToken(context.Background(), accessTokenFor("revoked-token"), "")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.False(s.T(), response.Active)
}

// When the deny list cannot be consulted, introspection fails closed with a server error rather
// than asserting the token is active. The refresh path is never reached, so a revocation outage
// cannot be masked by falling through to the next validator.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_EnforcementUnavailable_FailsClosed() {
	s.tokenValidatorMock.On("ValidateAccessToken", mock.Anything, accessTokenFor("some-token")).
		Return(nil, revocation.ErrEnforcementUnavailable)

	response, err := s.introspectService.IntrospectToken(context.Background(), accessTokenFor("some-token"), "")

	assert.Error(s.T(), err)
	assert.Nil(s.T(), response)
}

// An introspecting resource server needs the subject's identity class too, so sub_type is surfaced.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_SurfacesSubType() {
	for _, subType := range []string{constants.SubTypeAgent, constants.SubTypeApp} {
		s.Run(subType, func() {
			token := accessTokenFor("client-token-" + subType)
			s.stubAccessToken(token, map[string]interface{}{
				"sub":       "entity123",
				"sub_type":  subType,
				"client_id": "client123",
			})

			response, err := s.introspectService.IntrospectToken(context.Background(), token, "")

			assert.NoError(s.T(), err)
			assert.True(s.T(), response.Active)
			assert.Equal(s.T(), subType, response.SubType)
		})
	}
}

// A token without the claim reports no subject type rather than a guessed one.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_OmitsAbsentSubType() {
	token := accessTokenFor("no-sub-type")
	s.stubAccessToken(token, map[string]interface{}{"sub": "user123"})

	response, err := s.introspectService.IntrospectToken(context.Background(), token, "")

	assert.NoError(s.T(), err)
	assert.True(s.T(), response.Active)
	assert.Empty(s.T(), response.SubType)
}

// RFC 7662 Section 2.1 covers refresh tokens as well as access tokens, so a refresh token is
// reported active with its claims surfaced.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_RefreshToken_Active() {
	claims := map[string]interface{}{
		"sub":              "client123",
		"access_token_sub": "user123",
		"scope":            "openid profile",
		"jti":              "refresh-jti",
	}
	s.tokenValidatorMock.On("ValidateRefreshToken", mock.Anything, genericTokenFor("refresh-token")).
		Return(&tokenservice.RefreshTokenClaims{Claims: claims}, nil)

	response, err := s.introspectService.IntrospectToken(context.Background(), genericTokenFor("refresh-token"), "")

	assert.NoError(s.T(), err)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), "refresh-jti", response.Jti)
	assert.Equal(s.T(), "openid profile", response.Scope)
}

// Anything this server signs that is not an access or refresh token is outside the scope of
// RFC 7662. ID tokens and flow assertions carry the generic JWT typ but none of the refresh claims,
// so the refresh validator rejects them and they are reported inactive.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_NonOAuthToken_IsInactive() {
	for _, id := range []string{"id-token", "flow-assertion"} {
		s.Run(id, func() {
			token := genericTokenFor(id)
			s.tokenValidatorMock.On("ValidateRefreshToken", mock.Anything, token).
				Return(nil, errors.New("missing or invalid 'access_token_sub' claim"))

			response, err := s.introspectService.IntrospectToken(context.Background(), token, "")

			assert.NoError(s.T(), err)
			assert.NotNil(s.T(), response)
			assert.False(s.T(), response.Active)
		})
	}
}

// A typ this server does not introspect is rejected on the header alone, without reaching either
// validator, so an unrecognized token type can never fall through to a claim-shape check.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_UnsupportedTokenType_IsInactive() {
	for _, typ := range []string{jwt.TokenTypeIDJAG, "id+jwt"} {
		s.Run(typ, func() {
			token := tokenWithTyp(typ, "unsupported")

			response, err := s.introspectService.IntrospectToken(context.Background(), token, "")

			assert.NoError(s.T(), err)
			assert.NotNil(s.T(), response)
			assert.False(s.T(), response.Active)
			s.tokenValidatorMock.AssertNotCalled(s.T(), "ValidateAccessToken", mock.Anything, token)
			s.tokenValidatorMock.AssertNotCalled(s.T(), "ValidateRefreshToken", mock.Anything, token)
		})
	}
}

// The deny list must fail closed on the refresh path too, not just the access-token path.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_EnforcementUnavailableOnRefreshPath_FailsClosed() {
	s.tokenValidatorMock.On("ValidateRefreshToken", mock.Anything, genericTokenFor("refresh-token")).
		Return(nil, revocation.ErrEnforcementUnavailable)

	response, err := s.introspectService.IntrospectToken(context.Background(), genericTokenFor("refresh-token"), "")

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
	s.stubAccessToken(accessTokenFor("dpop-token"), claims)

	response, err := s.introspectService.IntrospectToken(context.Background(), accessTokenFor("dpop-token"), "")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), constants.TokenTypeDPoP, response.TokenType)
	assert.NotNil(s.T(), response.Cnf)
	assert.Equal(s.T(), "thumbprint-abc", response.Cnf.Jkt)
}
