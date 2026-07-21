/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "valid-token").Return(claims, nil)

	response, err := s.introspectService.IntrospectToken(context.Background(), "valid-token", "")

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

	response, err := s.introspectService.IntrospectToken(context.Background(), "array-aud-token", "")

	assert.NoError(s.T(), err)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), []string{"api.example.com", "api2.example.com"}, response.Aud)
}

// A valid token missing optional claims is still active, with empty optional fields.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_MissingOptionalClaims_Active() {
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "sparse-token").
		Return(map[string]interface{}{}, nil)

	response, err := s.introspectService.IntrospectToken(context.Background(), "sparse-token", "")

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
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "invalid-token").
		Return(nil, errors.New("token verification failed"))

	response, err := s.introspectService.IntrospectToken(context.Background(), "invalid-token", "")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.False(s.T(), response.Active)
}

// A revoked but otherwise valid token is reported inactive (RFC 7009 deny-list enforcement).
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_RevokedToken_IsInactive() {
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "revoked-token").
		Return(nil, revocation.ErrTokenRevoked)

	response, err := s.introspectService.IntrospectToken(context.Background(), "revoked-token", "")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.False(s.T(), response.Active)
}

// When the deny list cannot be consulted, introspection fails closed with a server error rather
// than asserting the token is active.
func (s *TokenIntrospectionServiceTestSuite) TestIntrospectToken_EnforcementUnavailable_FailsClosed() {
	s.tokenValidatorMock.On("ValidateToken", mock.Anything, "some-token").
		Return(nil, revocation.ErrEnforcementUnavailable)

	response, err := s.introspectService.IntrospectToken(context.Background(), "some-token", "")

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

	response, err := s.introspectService.IntrospectToken(context.Background(), "dpop-token", "")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), response)
	assert.True(s.T(), response.Active)
	assert.Equal(s.T(), constants.TokenTypeDPoP, response.TokenType)
	assert.NotNil(s.T(), response.Cnf)
	assert.Equal(s.T(), "thumbprint-abc", response.Cnf.Jkt)
}
