/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package token

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/granthandlersmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/scopemock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

type TokenServiceTestSuite struct {
	suite.Suite
	mockGrantProvider  *granthandlersmock.GrantHandlerProviderInterfaceMock
	mockScopeValidator *scopemock.ScopeValidatorInterfaceMock
	mockGrantHandler   *granthandlersmock.GrantHandlerInterfaceMock
	mockObsSvc         *observabilitymock.ObservabilityServiceInterfaceMock
	mockTransactioner  *MockTransactioner
}

// MockTransactioner is a simple implementation of Transactioner for testing.
type MockTransactioner struct{}

func (m *MockTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	return txFunc(ctx)
}

func TestTokenServiceSuite(t *testing.T) {
	suite.Run(t, new(TokenServiceTestSuite))
}

func (suite *TokenServiceTestSuite) SetupTest() {
	suite.mockGrantProvider = granthandlersmock.NewGrantHandlerProviderInterfaceMock(suite.T())
	suite.mockScopeValidator = scopemock.NewScopeValidatorInterfaceMock(suite.T())
	suite.mockGrantHandler = granthandlersmock.NewGrantHandlerInterfaceMock(suite.T())

	suite.mockObsSvc = observabilitymock.NewObservabilityServiceInterfaceMock(suite.T())
	suite.mockObsSvc.On("IsEnabled").Return(true).Maybe()
	suite.mockObsSvc.On("PublishEvent", mock.Anything).Return().Maybe()

	suite.mockTransactioner = &MockTransactioner{}

	// Common grant handler lookup; individual tests may override this.
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil).Maybe()
}

// newService builds a fresh tokenService using the suite's mocks.
func (suite *TokenServiceTestSuite) newService() TokenServiceInterface {
	return newTokenService(suite.mockGrantProvider, suite.mockScopeValidator, suite.mockObsSvc, suite.mockTransactioner)
}

// defaultApp returns an OAuthClient that allows the authorization_code grant.
func (suite *TokenServiceTestSuite) defaultApp() *inboundmodel.OAuthClient {
	return &inboundmodel.OAuthClient{
		ClientID:   "test-client-id",
		GrantTypes: []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_MissingGrantType() {
	svc := suite.newService()
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: "",
	}

	_, errResp := svc.ProcessTokenRequest(context.Background(), req, suite.defaultApp())

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidRequest, errResp.Error)
	assert.Equal(suite.T(), "Missing grant_type parameter", errResp.ErrorDescription)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_InvalidGrantType() {
	svc := suite.newService()
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: "invalid_grant",
	}

	_, errResp := svc.ProcessTokenRequest(context.Background(), req, suite.defaultApp())

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorUnsupportedGrantType, errResp.Error)
	assert.Equal(suite.T(), "Invalid grant_type parameter", errResp.ErrorDescription)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_UnsupportedGrantTypeError() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(nil, constants.UnSupportedGrantTypeError)

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, suite.defaultApp())

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorUnsupportedGrantType, errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_GrantHandlerProviderError() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(nil, errors.New("internal error"))

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, suite.defaultApp())

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_UnauthorizedClient() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeClientCredentials),
	}
	// App only allows authorization_code — client_credentials is not permitted.
	app := &inboundmodel.OAuthClient{
		ClientID:   "test-client-id",
		GrantTypes: []constants.GrantType{constants.GrantTypeAuthorizationCode},
	}

	mockCCHandler := granthandlersmock.NewGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeClientCredentials).
		Return(mockCCHandler, nil)

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorUnauthorizedClient, errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_ValidateGrantError() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
		Code:      "test-code",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	suite.mockGrantHandler.
		On("ValidateGrant", mock.Anything, mock.Anything, app).
		Return(&model.ErrorResponse{
			Error:            "invalid_grant",
			ErrorDescription: "Invalid authorization code",
		})

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), "invalid_grant", errResp.Error)
	assert.Equal(suite.T(), "Invalid authorization code", errResp.ErrorDescription)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_ScopeValidationError() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "invalid_scope",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)

	suite.mockScopeValidator.
		On("ValidateScopes", mock.Anything, "invalid_scope", "test-client-id").
		Return("", &scope.ScopeError{
			Error:            "invalid_scope",
			ErrorDescription: "Invalid scope requested",
		})

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), "invalid_scope", errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_HandleGrantError() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)
	suite.mockGrantHandler.
		On("HandleGrant", mock.Anything, mock.Anything, app).
		Return(nil, &model.ErrorResponse{
			Error:            "invalid_grant",
			ErrorDescription: "Authorization code expired",
		})

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), "invalid_grant", errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_HandleGrantServerError_NormalizesDescription() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)
	suite.mockGrantHandler.
		On("HandleGrant", mock.Anything, mock.Anything, app).
		Return(nil, &model.ErrorResponse{
			Error:            constants.ErrorServerError,
			ErrorDescription: "Failed to generate token",
		})

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
	assert.Equal(suite.T(), "Failed to process token request", errResp.ErrorDescription)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_Success() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid profile",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid profile", "test-client-id").
		Return("openid profile", nil)

	tokenRespDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			Token:     "access-token-123",
			TokenType: "Bearer",
			ExpiresIn: 3600,
			Scopes:    []string{"openid", "profile"},
		},
		RefreshToken: model.TokenDTO{Token: ""},
		IDToken:      model.TokenDTO{Token: ""},
	}
	suite.mockGrantHandler.On("HandleGrant", mock.Anything, mock.Anything, app).Return(tokenRespDTO, nil)

	svc := suite.newService()
	tokenResp, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), tokenResp)
	assert.Equal(suite.T(), "access-token-123", tokenResp.AccessToken)
	assert.Equal(suite.T(), "Bearer", tokenResp.TokenType)
	assert.Equal(suite.T(), int64(3600), tokenResp.ExpiresIn)
	assert.Equal(suite.T(), "openid profile", tokenResp.Scope)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_WithRefreshToken() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	// App allows both authorization_code and refresh_token.
	app := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []constants.GrantType{
			constants.GrantTypeAuthorizationCode,
			constants.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	mockRefreshHandler := granthandlersmock.NewRefreshTokenGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeRefreshToken).
		Return(mockRefreshHandler, nil)

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)

	tokenRespDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			Token:     "access-token-123",
			TokenType: "Bearer",
			ExpiresIn: 3600,
			Scopes:    []string{"openid"},
			Subject:   "user123",
			Audiences: []string{"test-audience"},
		},
		RefreshToken: model.TokenDTO{Token: ""},
		IDToken:      model.TokenDTO{Token: ""},
	}
	suite.mockGrantHandler.On("HandleGrant", mock.Anything, mock.Anything, app).Return(tokenRespDTO, nil)

	mockRefreshHandler.
		On("IssueRefreshToken", mock.Anything, tokenRespDTO, app, "user123", []string{"test-audience"},
			"authorization_code", []string{"openid"}, (*model.ClaimsRequest)(nil), "", "").
		Return(nil)

	svc := suite.newService()
	tokenResp, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), tokenResp)
	assert.Equal(suite.T(), "access-token-123", tokenResp.AccessToken)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_RefreshTokenIssuanceError() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []constants.GrantType{
			constants.GrantTypeAuthorizationCode,
			constants.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	mockRefreshHandler := granthandlersmock.NewRefreshTokenGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeRefreshToken).
		Return(mockRefreshHandler, nil)

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)

	tokenRespDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			Token:     "access-token-123",
			TokenType: "Bearer",
			ExpiresIn: 3600,
			Scopes:    []string{"openid"},
			Subject:   "user123",
			Audiences: []string{"test-audience"},
		},
		RefreshToken: model.TokenDTO{Token: ""},
		IDToken:      model.TokenDTO{Token: ""},
	}
	suite.mockGrantHandler.On("HandleGrant", mock.Anything, mock.Anything, app).Return(tokenRespDTO, nil)

	mockRefreshHandler.
		On("IssueRefreshToken", mock.Anything, tokenRespDTO, app, "user123", []string{"test-audience"},
			"authorization_code", []string{"openid"}, (*model.ClaimsRequest)(nil), "", "").
		Return(&model.ErrorResponse{
			Error:            "server_error",
			ErrorDescription: "Failed to issue refresh token",
		})

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), "server_error", errResp.Error)
	assert.Equal(suite.T(), "Failed to process token request", errResp.ErrorDescription)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_RefreshTokenHandlerNotFound() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []constants.GrantType{
			constants.GrantTypeAuthorizationCode,
			constants.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeRefreshToken).
		Return(nil, errors.New("refresh handler not found"))

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)

	tokenRespDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			Token: "access-token-123", TokenType: "Bearer", ExpiresIn: 3600,
			Scopes: []string{"openid"}, Subject: "user123", Audiences: []string{"test-audience"},
		},
		RefreshToken: model.TokenDTO{Token: ""},
		IDToken:      model.TokenDTO{Token: ""},
	}
	suite.mockGrantHandler.On("HandleGrant", mock.Anything, mock.Anything, app).Return(tokenRespDTO, nil)

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_RefreshTokenHandlerCastFailure() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []constants.GrantType{
			constants.GrantTypeAuthorizationCode,
			constants.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)
	// Return a plain GrantHandlerInterfaceMock which does NOT implement RefreshTokenGrantHandlerInterface.
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeRefreshToken).
		Return(suite.mockGrantHandler, nil)

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)

	tokenRespDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			Token: "access-token-123", TokenType: "Bearer", ExpiresIn: 3600,
			Scopes: []string{"openid"}, Subject: "user123", Audiences: []string{"test-audience"},
		},
		RefreshToken: model.TokenDTO{Token: ""},
		IDToken:      model.TokenDTO{Token: ""},
	}
	suite.mockGrantHandler.On("HandleGrant", mock.Anything, mock.Anything, app).Return(tokenRespDTO, nil)

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_TokenExchange() {
	req := &model.TokenRequest{
		ClientID:           "test-client-id",
		GrantType:          string(constants.GrantTypeTokenExchange),
		SubjectToken:       "subject-token",
		RequestedTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}
	app := &inboundmodel.OAuthClient{
		ClientID:   "test-client-id",
		GrantTypes: []constants.GrantType{constants.GrantTypeTokenExchange},
	}

	mockTEHandler := granthandlersmock.NewGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeTokenExchange).
		Return(mockTEHandler, nil)

	mockTEHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "", "test-client-id").Return("", nil)

	tokenRespDTO := &model.TokenResponseDTO{
		AccessToken:  model.TokenDTO{Token: "exchanged-token", TokenType: "Bearer", ExpiresIn: 3600},
		RefreshToken: model.TokenDTO{Token: ""},
		IDToken:      model.TokenDTO{Token: ""},
	}
	mockTEHandler.On("HandleGrant", mock.Anything, mock.Anything, app).Return(tokenRespDTO, nil)

	svc := suite.newService()
	tokenResp, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), tokenResp)
	assert.Equal(suite.T(), "exchanged-token", tokenResp.AccessToken)
	assert.Equal(suite.T(), string(constants.TokenTypeIdentifierAccessToken), tokenResp.IssuedTokenType)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_TokenExchangeWithJWTTokenType() {
	req := &model.TokenRequest{
		ClientID:           "test-client-id",
		GrantType:          string(constants.GrantTypeTokenExchange),
		SubjectToken:       "subject-token",
		RequestedTokenType: string(constants.TokenTypeIdentifierJWT),
	}
	app := &inboundmodel.OAuthClient{
		ClientID:   "test-client-id",
		GrantTypes: []constants.GrantType{constants.GrantTypeTokenExchange},
	}

	mockTEHandler := granthandlersmock.NewGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeTokenExchange).
		Return(mockTEHandler, nil)

	mockTEHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "", "test-client-id").Return("", nil)

	tokenRespDTO := &model.TokenResponseDTO{
		AccessToken:  model.TokenDTO{Token: "exchanged-token", TokenType: "Bearer", ExpiresIn: 3600},
		RefreshToken: model.TokenDTO{Token: ""},
		IDToken:      model.TokenDTO{Token: ""},
	}
	mockTEHandler.On("HandleGrant", mock.Anything, mock.Anything, app).Return(tokenRespDTO, nil)

	svc := suite.newService()
	tokenResp, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), tokenResp)
	assert.Equal(suite.T(), string(constants.TokenTypeIdentifierJWT), tokenResp.IssuedTokenType)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_WithRefreshToken_UsesOriginalAudiences() {
	// When the access token carries OriginalAudiences (narrowing occurred), the refresh token
	// issuance must receive the original full set, not the narrowed Audiences (RFC 8707 §5).
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(constants.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := &inboundmodel.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []constants.GrantType{
			constants.GrantTypeAuthorizationCode,
			constants.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	mockRefreshHandler := granthandlersmock.NewRefreshTokenGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.
		On("GetGrantHandler", constants.GrantTypeRefreshToken).
		Return(mockRefreshHandler, nil)

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)

	tokenRespDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			Token:             "access-token-123",
			TokenType:         "Bearer",
			ExpiresIn:         3600,
			Scopes:            []string{"openid"},
			Subject:           "user123",
			Audiences:         []string{"narrowed-audience"},
			OriginalAudiences: []string{"original-audience-1", "original-audience-2"},
		},
		RefreshToken: model.TokenDTO{Token: ""},
		IDToken:      model.TokenDTO{Token: ""},
	}
	suite.mockGrantHandler.On("HandleGrant", mock.Anything, mock.Anything, app).Return(tokenRespDTO, nil)

	mockRefreshHandler.
		On("IssueRefreshToken", mock.Anything, tokenRespDTO, app, "user123",
			[]string{"original-audience-1", "original-audience-2"},
			"authorization_code", []string{"openid"}, (*model.ClaimsRequest)(nil), "", "").
		Return(nil)

	svc := suite.newService()
	tokenResp, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), tokenResp)
	assert.Equal(suite.T(), "access-token-123", tokenResp.AccessToken)
}
