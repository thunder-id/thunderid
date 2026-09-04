// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/dpopmock"
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
	mockDPoPVerifier   *dpopmock.VerifierInterfaceMock
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
	suite.mockObsSvc.On("PublishEvent", mock.Anything, mock.Anything).Return().Maybe()

	suite.mockDPoPVerifier = dpopmock.NewVerifierInterfaceMock(suite.T())

	// Common grant handler lookup; individual tests may override this.
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil).Maybe()
}

// newService builds a fresh tokenService using the suite's mocks.
func (suite *TokenServiceTestSuite) newService() TokenServiceInterface {
	return newTokenService(suite.mockGrantProvider, suite.mockScopeValidator, suite.mockObsSvc,
		suite.mockDPoPVerifier, "https://example.test/oauth2/token", false)
}

// defaultApp returns an OAuthClient that allows the authorization_code grant.
func (suite *TokenServiceTestSuite) defaultApp() *providers.OAuthClient {
	return &providers.OAuthClient{
		ClientID:   "test-client-id",
		GrantTypes: []providers.GrantType{providers.GrantTypeAuthorizationCode},
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
		GrantType: string(providers.GrantTypeAuthorizationCode),
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(nil, constants.UnSupportedGrantTypeError)

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, suite.defaultApp())

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorUnsupportedGrantType, errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_GrantHandlerProviderError() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(providers.GrantTypeAuthorizationCode),
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(nil, errors.New("internal error"))

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, suite.defaultApp())

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorServerError, errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_UnauthorizedClient() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(providers.GrantTypeClientCredentials),
	}
	// App only allows authorization_code — client_credentials is not permitted.
	app := &providers.OAuthClient{
		ClientID:   "test-client-id",
		GrantTypes: []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}

	mockCCHandler := granthandlersmock.NewGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeClientCredentials).
		Return(mockCCHandler, nil)

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorUnauthorizedClient, errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_ValidateGrantError() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
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
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "invalid_scope",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
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
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
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
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
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
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid profile",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
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

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_DPoPProof_Verified_PropagatesJktToHandler() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := suite.defaultApp()

	const testJkt = "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I"
	const testProof = "eyJ.dpop.proof"

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)
	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)

	suite.mockDPoPVerifier.
		On("Verify", mock.Anything, mock.MatchedBy(func(p dpop.VerifyParams) bool {
			return p.Proof == testProof && p.HTM == "POST" &&
				p.HTU == "https://example.test/oauth2/token"
		})).
		Return(&dpop.ProofResult{JKT: testJkt}, nil)

	suite.mockGrantHandler.
		On("HandleGrant",
			mock.MatchedBy(func(ctx context.Context) bool { return dpop.GetJkt(ctx) == testJkt }),
			mock.Anything, app).
		Return(&model.TokenResponseDTO{
			AccessToken: model.TokenDTO{Token: "at", TokenType: constants.TokenTypeDPoP, ExpiresIn: 3600},
		}, nil)

	svc := suite.newService()
	ctx := dpop.WithProof(context.Background(), testProof)
	resp, errResp := svc.ProcessTokenRequest(ctx, req, app)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), constants.TokenTypeDPoP, resp.TokenType)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_DPoPProof_VerifyFails_InvalidDPoPProof() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)
	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)

	suite.mockDPoPVerifier.
		On("Verify", mock.Anything, mock.Anything).
		Return(nil, dpop.ErrInvalidProof)

	svc := suite.newService()
	ctx := dpop.WithProof(context.Background(), "bad-proof")
	_, errResp := svc.ProcessTokenRequest(ctx, req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidDPoPProof, errResp.Error)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_NoDPoPProof_VerifierNotInvoked() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)
	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)
	suite.mockGrantHandler.
		On("HandleGrant", mock.Anything, mock.Anything, app).
		Return(&model.TokenResponseDTO{
			AccessToken: model.TokenDTO{Token: "at", TokenType: constants.TokenTypeBearer, ExpiresIn: 3600},
		}, nil)

	svc := suite.newService()
	resp, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.Nil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.TokenTypeBearer, resp.TokenType)
	suite.mockDPoPVerifier.AssertNotCalled(suite.T(), "Verify", mock.Anything, mock.Anything)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_NoDPoPProof_PerClientFlag_Rejected() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := &providers.OAuthClient{
		ClientID:              "test-client-id",
		GrantTypes:            []providers.GrantType{providers.GrantTypeAuthorizationCode},
		DPoPBoundAccessTokens: true,
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)
	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)

	svc := suite.newService()
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidDPoPProof, errResp.Error)
	suite.mockDPoPVerifier.AssertNotCalled(suite.T(), "Verify", mock.Anything, mock.Anything)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_NoDPoPProof_GlobalRequired_Rejected() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := suite.defaultApp()

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)
	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)

	svc := newTokenService(suite.mockGrantProvider, suite.mockScopeValidator, suite.mockObsSvc,
		suite.mockDPoPVerifier, "https://example.test/oauth2/token", true)
	_, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.NotNil(suite.T(), errResp)
	assert.Equal(suite.T(), constants.ErrorInvalidDPoPProof, errResp.Error)
	suite.mockDPoPVerifier.AssertNotCalled(suite.T(), "Verify", mock.Anything, mock.Anything)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_WithRefreshToken() {
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	// App allows both authorization_code and refresh_token.
	app := &providers.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []providers.GrantType{
			providers.GrantTypeAuthorizationCode,
			providers.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	mockRefreshHandler := granthandlersmock.NewRefreshTokenGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeRefreshToken).
		Return(mockRefreshHandler, nil)

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "openid", "test-client-id").Return("openid", nil)

	tokenRespDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			Token:         "access-token-123",
			TokenType:     "Bearer",
			ExpiresIn:     3600,
			Scopes:        []string{"openid"},
			Subject:       "user123",
			Audiences:     []string{"test-audience"},
			TokenFamilyID: "tfid-access-123",
		},
		RefreshToken: model.TokenDTO{Token: ""},
		IDToken:      model.TokenDTO{Token: ""},
	}
	suite.mockGrantHandler.On("HandleGrant", mock.Anything, mock.Anything, app).Return(tokenRespDTO, nil)

	// The access token's tfid must be forwarded to refresh-token issuance so both tokens share the family.
	mockRefreshHandler.
		On("IssueRefreshToken", mock.Anything, tokenRespDTO, app, "user123", []string{"test-audience"},
			"authorization_code", []string{"openid"}, (*model.ClaimsRequest)(nil), "", "", "tfid-access-123", int64(0)).
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
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := &providers.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []providers.GrantType{
			providers.GrantTypeAuthorizationCode,
			providers.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	mockRefreshHandler := granthandlersmock.NewRefreshTokenGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeRefreshToken).
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
			"authorization_code", []string{"openid"}, (*model.ClaimsRequest)(nil), "", "", "", int64(0)).
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
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := &providers.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []providers.GrantType{
			providers.GrantTypeAuthorizationCode,
			providers.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeRefreshToken).
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
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := &providers.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []providers.GrantType{
			providers.GrantTypeAuthorizationCode,
			providers.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)
	// Return a plain GrantHandlerInterfaceMock which does NOT implement RefreshTokenGrantHandlerInterface.
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeRefreshToken).
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
		GrantType:          string(providers.GrantTypeTokenExchange),
		SubjectToken:       "subject-token",
		RequestedTokenType: string(constants.TokenTypeIdentifierAccessToken),
	}
	app := &providers.OAuthClient{
		ClientID:   "test-client-id",
		GrantTypes: []providers.GrantType{providers.GrantTypeTokenExchange},
	}

	mockTEHandler := granthandlersmock.NewGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeTokenExchange).
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
		GrantType:          string(providers.GrantTypeTokenExchange),
		SubjectToken:       "subject-token",
		RequestedTokenType: string(constants.TokenTypeIdentifierJWT),
	}
	app := &providers.OAuthClient{
		ClientID:   "test-client-id",
		GrantTypes: []providers.GrantType{providers.GrantTypeTokenExchange},
	}

	mockTEHandler := granthandlersmock.NewGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeTokenExchange).
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
		GrantType: string(providers.GrantTypeAuthorizationCode),
		Code:      "test-code",
		Scope:     "openid",
	}
	app := &providers.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []providers.GrantType{
			providers.GrantTypeAuthorizationCode,
			providers.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeAuthorizationCode).
		Return(suite.mockGrantHandler, nil)

	mockRefreshHandler := granthandlersmock.NewRefreshTokenGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeRefreshToken).
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
			"authorization_code", []string{"openid"}, (*model.ClaimsRequest)(nil), "", "", "", int64(0)).
		Return(nil)

	svc := suite.newService()
	tokenResp, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), tokenResp)
	assert.Equal(suite.T(), "access-token-123", tokenResp.AccessToken)
}

func (suite *TokenServiceTestSuite) TestProcessTokenRequest_CIBA_RefreshTokenUsesResourceAudience() {
	// A resource-bound CIBA access token carries its RS identifier as OriginalAudiences; the refresh
	// token issued for the CIBA grant must inherit that single RS audience for continuity (RFC 8707).
	req := &model.TokenRequest{
		ClientID:  "test-client-id",
		GrantType: string(providers.GrantTypeCIBA),
		AuthReqID: "auth-req-1",
	}
	app := &providers.OAuthClient{
		ClientID: "test-client-id",
		GrantTypes: []providers.GrantType{
			providers.GrantTypeCIBA,
			providers.GrantTypeRefreshToken,
		},
	}

	suite.mockGrantProvider.ExpectedCalls = nil
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeCIBA).
		Return(suite.mockGrantHandler, nil)

	mockRefreshHandler := granthandlersmock.NewRefreshTokenGrantHandlerInterfaceMock(suite.T())
	suite.mockGrantProvider.
		On("GetGrantHandler", providers.GrantTypeRefreshToken).
		Return(mockRefreshHandler, nil)

	suite.mockGrantHandler.On("ValidateGrant", mock.Anything, mock.Anything, app).Return(nil)
	suite.mockScopeValidator.On("ValidateScopes", mock.Anything, "", "test-client-id").Return("", nil)

	tokenRespDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			Token:             "access-token-123",
			TokenType:         "Bearer",
			ExpiresIn:         3600,
			Scopes:            []string{"openid", "read"},
			Subject:           "user-1",
			Audiences:         []string{"https://api.example.com"},
			OriginalAudiences: []string{"https://api.example.com"},
		},
		RefreshToken: model.TokenDTO{Token: ""},
		IDToken:      model.TokenDTO{Token: ""},
	}
	suite.mockGrantHandler.On("HandleGrant", mock.Anything, mock.Anything, app).Return(tokenRespDTO, nil)

	mockRefreshHandler.
		On("IssueRefreshToken", mock.Anything, tokenRespDTO, app, "user-1",
			[]string{"https://api.example.com"},
			string(providers.GrantTypeCIBA), []string{"openid", "read"},
			(*model.ClaimsRequest)(nil), "", "", "", int64(0)).
		Return(nil)

	svc := suite.newService()
	tokenResp, errResp := svc.ProcessTokenRequest(context.Background(), req, app)

	assert.Nil(suite.T(), errResp)
	assert.NotNil(suite.T(), tokenResp)
	assert.Equal(suite.T(), "access-token-123", tokenResp.AccessToken)
}

const (
	testAgentEntityID = "agent-entity-1"
	testAppEntityID   = "app-entity-1"
	testTraceID       = "trace-1"
)

type TokenEventsTestSuite struct {
	suite.Suite
	mockObsSvc *observabilitymock.ObservabilityServiceInterfaceMock
	published  []*providers.Event
}

func TestTokenEventsSuite(t *testing.T) {
	suite.Run(t, new(TokenEventsTestSuite))
}

func (suite *TokenEventsTestSuite) SetupTest() {
	suite.published = nil
	suite.mockObsSvc = observabilitymock.NewObservabilityServiceInterfaceMock(suite.T())
	suite.mockObsSvc.On("IsEnabled").Return(true).Maybe()
	suite.mockObsSvc.
		On("PublishEvent", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			suite.published = append(suite.published, args.Get(1).(*providers.Event))
		}).
		Return().Maybe()
}

func (suite *TokenEventsTestSuite) newService() *tokenService {
	return &tokenService{observabilitySvc: suite.mockObsSvc}
}

func (suite *TokenEventsTestSuite) ctx() context.Context {
	return sysContext.WithTraceID(context.Background(), testTraceID)
}

// lastEvent returns the single event published by the call under test.
func (suite *TokenEventsTestSuite) lastEvent() *providers.Event {
	suite.Require().Len(suite.published, 1)
	return suite.published[0]
}

func agentClient() *providers.OAuthClient {
	return &providers.OAuthClient{
		ID:             testAgentEntityID,
		ClientID:       "agent-client-id",
		EntityCategory: providers.EntityCategoryAgent,
	}
}

func appClient() *providers.OAuthClient {
	return &providers.OAuthClient{
		ID:             testAppEntityID,
		ClientID:       "app-client-id",
		EntityCategory: providers.EntityCategoryApp,
	}
}

func (suite *TokenEventsTestSuite) TestIssuanceStarted_AgentReportsActorTypeAndEntityID() {
	suite.newService().publishTokenIssuanceStartedEvent(
		suite.ctx(), agentClient(), "agent-client-id", "client_credentials", "")

	evt := suite.lastEvent()
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.ActorType])
	assert.Equal(suite.T(), testAgentEntityID, evt.Data[event.DataKey.EntityID])
	assert.Equal(suite.T(), testTraceID, evt.Data[event.DataKey.CorrelationID])
}

// The entity vocabulary spells an application "app" while the reported principal type spells it
// "application", matching the token's sub_type claim.
func (suite *TokenEventsTestSuite) TestIssuanceStarted_ApplicationReportsApplicationActorType() {
	suite.newService().publishTokenIssuanceStartedEvent(
		suite.ctx(), appClient(), "app-client-id", "authorization_code", "")

	assert.Equal(suite.T(), event.PrincipalTypeApplication, suite.lastEvent().Data[event.DataKey.ActorType])
}

func (suite *TokenEventsTestSuite) TestIssuanceStarted_UnresolvedClientOmitsPrincipalFields() {
	suite.newService().publishTokenIssuanceStartedEvent(suite.ctx(), nil, "", "", "")

	evt := suite.lastEvent()
	assert.NotContains(suite.T(), evt.Data, event.DataKey.ActorType)
	assert.NotContains(suite.T(), evt.Data, event.DataKey.EntityID)
}

func (suite *TokenEventsTestSuite) TestIssued_M2MReportsAgentSubjectAndNoDelegation() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			SubjectID:       testAgentEntityID,
			SubjectCategory: string(providers.EntityCategoryAgent),
		},
	}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "client_credentials", "", 0)

	evt := suite.lastEvent()
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.ActorType])
	assert.Equal(suite.T(), testAgentEntityID, evt.Data[event.DataKey.Subject])
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.SubjectType])
	assert.Equal(suite.T(), false, evt.Data[event.DataKey.IsDelegated])
	assert.NotContains(suite.T(), evt.Data, event.DataKey.ActorSub)
}

func (suite *TokenEventsTestSuite) TestIssued_OBOReportsUserSubjectWithAgentActor() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			SubjectID:       "user-1",
			SubjectCategory: string(providers.EntityCategoryUser),
			ActorID:         testAgentEntityID,
		},
	}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "authorization_code", "", 0)

	evt := suite.lastEvent()
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.ActorType])
	assert.Equal(suite.T(), "user-1", evt.Data[event.DataKey.Subject])
	assert.Equal(suite.T(), event.PrincipalTypeUser, evt.Data[event.DataKey.SubjectType])
	assert.Equal(suite.T(), testAgentEntityID, evt.Data[event.DataKey.ActorSub])
	assert.Equal(suite.T(), true, evt.Data[event.DataKey.IsDelegated])
}

func (suite *TokenEventsTestSuite) TestIssued_GrantCorrelationIDWinsOverTraceID() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{
		AccessToken:   model.TokenDTO{SubjectID: "user-1"},
		CorrelationID: "flow-execution-1",
	}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "authorization_code", "", 0)

	assert.Equal(suite.T(), "flow-execution-1", suite.lastEvent().Data[event.DataKey.CorrelationID])
}

func (suite *TokenEventsTestSuite) TestIssued_FallsBackToTraceIDWhenGrantHasNoCorrelationID() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{AccessToken: model.TokenDTO{SubjectID: testAgentEntityID}}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "client_credentials", "", 0)

	assert.Equal(suite.T(), testTraceID, suite.lastEvent().Data[event.DataKey.CorrelationID])
}

func (suite *TokenEventsTestSuite) TestIssuanceFailed_ReportsActorTypeWhenClientIsKnown() {
	publishTokenIssuanceFailedEvent(suite.mockObsSvc, suite.ctx(), agentClient(),
		"agent-client-id", "client_credentials", "", 400, "invalid_scope", 0)

	evt := suite.lastEvent()
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.ActorType])
	assert.Equal(suite.T(), testAgentEntityID, evt.Data[event.DataKey.EntityID])
	assert.Equal(suite.T(), testTraceID, evt.Data[event.DataKey.CorrelationID])
}

// The subject's category is resolved while the token is built, so the event reports whatever the
// token carries rather than inferring it from the client.
func (suite *TokenEventsTestSuite) TestIssued_SubjectTypeIsReadFromTheToken() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{
		AccessToken: model.TokenDTO{
			SubjectID:       "agent-b",
			SubjectCategory: string(providers.EntityCategoryAgent),
		},
	}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "urn:ietf:params:oauth:grant-type:token-exchange", "", 0)

	evt := suite.lastEvent()
	assert.Equal(suite.T(), "agent-b", evt.Data[event.DataKey.Subject])
	assert.Equal(suite.T(), event.PrincipalTypeAgent, evt.Data[event.DataKey.SubjectType])
}

func (suite *TokenEventsTestSuite) TestIssued_SubjectTypeOmittedWhenTokenHasNoCategory() {
	app := agentClient()
	respDTO := &model.TokenResponseDTO{AccessToken: model.TokenDTO{SubjectID: "user-1"}}

	suite.newService().publishTokenIssuedEvent(
		suite.ctx(), app, respDTO, app.ClientID, "authorization_code", "", 0)

	evt := suite.lastEvent()
	assert.Equal(suite.T(), "user-1", evt.Data[event.DataKey.Subject])
	assert.NotContains(suite.T(), evt.Data, event.DataKey.SubjectType)
}
