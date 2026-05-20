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

package authz

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

// stubTransactioner is a no-op Transactioner for use in service tests.
type stubTransactioner struct{}

func (s *stubTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	return txFunc(ctx)
}

// JWT constants used in service tests.
const (
	// Header: {"alg":"none","typ":"JWT"}   Payload: {"sub":"test-user","iat":1701421200}
	svcJWTWithIat = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ0ZXN0LXVzZXIiLCJpYXQiOjE3MDE0MjEyMDB9."
	// Header: {"alg":"none","typ":"JWT"}   Payload: {"sub":"test-user"}
	svcJWTMinimal = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ0ZXN0LXVzZXIifQ."
)

type AuthorizeServiceTestSuite struct {
	suite.Suite
	mockInboundClient   *inboundclientmock.InboundClientServiceInterfaceMock
	mockJWTService      *jwtmock.JWTServiceInterfaceMock
	mockAuthzCodeStore  *AuthorizationCodeStoreInterfaceMock
	mockAuthReqStore    *authorizationRequestStoreInterfaceMock
	mockFlowExecService *flowexecmock.FlowExecServiceInterfaceMock
	mockValidator       *AuthorizationValidatorInterfaceMock
}

func TestAuthorizeServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizeServiceTestSuite))
}

func (suite *AuthorizeServiceTestSuite) BeforeTest(suiteName, testName string) {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		GateClient: config.GateClientConfig{
			Scheme:    "https",
			Hostname:  "localhost",
			Port:      3000,
			LoginPath: "/login",
			ErrorPath: "/error",
		},
		Database: config.DatabaseConfig{
			Config:  config.DataSource{Type: "sqlite", SQLite: config.SQLiteDataSource{Path: ":memory:"}},
			Runtime: config.DataSource{Type: "sqlite", SQLite: config.SQLiteDataSource{Path: ":memory:"}},
		},
		JWT: config.JWTConfig{
			Issuer: "https://localhost:8090",
		},
		OAuth: config.OAuthConfig{
			AuthorizationCode: config.AuthorizationCodeConfig{ValidityPeriod: 600},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)
}

func (suite *AuthorizeServiceTestSuite) SetupTest() {
	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockAuthzCodeStore = NewAuthorizationCodeStoreInterfaceMock(suite.T())
	suite.mockAuthReqStore = newAuthorizationRequestStoreInterfaceMock(suite.T())
	suite.mockFlowExecService = flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
	suite.mockValidator = NewAuthorizationValidatorInterfaceMock(suite.T())
}

// newService builds an authorizeService with all mocked dependencies.
func (suite *AuthorizeServiceTestSuite) newService() *authorizeService {
	return &authorizeService{
		inboundClient:   suite.mockInboundClient,
		authZValidator:  suite.mockValidator,
		authCodeStore:   suite.mockAuthzCodeStore,
		authReqStore:    suite.mockAuthReqStore,
		jwtService:      suite.mockJWTService,
		flowExecService: suite.mockFlowExecService,
		transactioner:   &stubTransactioner{},
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizeServiceTest")),
	}
}

// testApp returns a minimal OAuthClient for use in tests.
func (suite *AuthorizeServiceTestSuite) testApp() *inboundmodel.OAuthClient {
	return &inboundmodel.OAuthClient{
		ID:           "test-app-id",
		ClientID:     "test-client-id",
		RedirectURIs: []string{"https://client.example.com/callback"},
		GrantTypes:   []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		PKCERequired: false,
	}
}

// testMsg returns a minimal OAuthMessage for initial authorization requests.
func (suite *AuthorizeServiceTestSuite) testMsg() *OAuthMessage {
	return &OAuthMessage{
		RequestType: oauth2const.TypeInitialAuthorizationRequest,
		RequestQueryParams: map[string]string{
			"client_id":     "test-client-id",
			"redirect_uri":  "https://client.example.com/callback",
			"response_type": "code",
			"scope":         "read write",
			"state":         "test-state",
		},
	}
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_MissingClientID() {
	msg := &OAuthMessage{
		RequestType: oauth2const.TypeInitialAuthorizationRequest,
		RequestQueryParams: map[string]string{
			"redirect_uri":  "https://client.example.com/callback",
			"response_type": "code",
		},
	}

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorInvalidRequest, authErr.Code)
	assert.Contains(suite.T(), authErr.Message, "Missing client_id")
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_InvalidClient() {
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "invalid-client").Return(nil, nil)

	msg := &OAuthMessage{
		RequestType: oauth2const.TypeInitialAuthorizationRequest,
		RequestQueryParams: map[string]string{
			"client_id":     "invalid-client",
			"redirect_uri":  "https://client.example.com/callback",
			"response_type": "code",
		},
	}

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorInvalidRequest, authErr.Code)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_InvalidClaimsParameter() {
	app := suite.testApp()
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)

	msg := &OAuthMessage{
		RequestType: oauth2const.TypeInitialAuthorizationRequest,
		RequestQueryParams: map[string]string{
			"client_id":    "test-client-id",
			"redirect_uri": "https://client.example.com/callback",
			"claims":       "{invalid json}",
		},
	}

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorInvalidRequest, authErr.Code)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_ValidationError_NoClientRedirect() {
	app := suite.testApp()
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)

	// Validator rejects; sendErrorToApp=false → error goes to error page, not client.
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, oauth2const.ErrorInvalidRequest, "Missing required parameter")

	msg := suite.testMsg()
	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorInvalidRequest, authErr.Code)
	assert.False(suite.T(), authErr.SendErrorToClient)
	assert.Equal(suite.T(), "test-state", authErr.State)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_ValidationError_SendToClient() {
	app := suite.testApp()
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)

	// sendErrorToApp=true + redirect_uri present → error forwarded to client.
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(true, oauth2const.ErrorUnsupportedResponseType, "Unsupported response_type value")

	msg := suite.testMsg()
	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorUnsupportedResponseType, authErr.Code)
	assert.True(suite.T(), authErr.SendErrorToClient)
	assert.Equal(suite.T(), "https://client.example.com/callback", authErr.ClientRedirectURI)
	assert.Equal(suite.T(), "test-state", authErr.State)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_FlowInitError() {
	app := suite.testApp()
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything, mock.Anything).
		Return("", &serviceerror.InternalServerError)

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), suite.testMsg())

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorServerError, authErr.Code)
	assert.True(suite.T(), authErr.SendErrorToClient)
	assert.Equal(suite.T(), "https://client.example.com/callback", authErr.ClientRedirectURI)
	assert.Equal(suite.T(), "test-state", authErr.State)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_Success() {
	app := suite.testApp()
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), suite.testMsg())

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testAuthID, result.QueryParams[oauth2const.AuthID])
	assert.Equal(suite.T(), "test-app-id", result.QueryParams[oauth2const.AppID])
	assert.Equal(suite.T(), "test-flow-id", result.QueryParams[oauth2const.ExecutionID])
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_InsecureRedirectURI() {
	app := suite.testApp()
	app.RedirectURIs = []string{"http://client.example.com/callback"}
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	msg := &OAuthMessage{
		RequestType: oauth2const.TypeInitialAuthorizationRequest,
		RequestQueryParams: map[string]string{
			"client_id":     "test-client-id",
			"redirect_uri":  "http://client.example.com/callback",
			"response_type": "code",
			"scope":         "read write",
		},
	}

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "true", result.QueryParams[oauth2const.ShowInsecureWarning])
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_EmptyRedirectURIUsesAppDefault() {
	app := suite.testApp() // RedirectURIs: ["https://client.example.com/callback"]
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	msg := &OAuthMessage{
		RequestType: oauth2const.TypeInitialAuthorizationRequest,
		RequestQueryParams: map[string]string{
			"client_id":     "test-client-id",
			"response_type": "code",
			"scope":         "read write",
			// No redirect_uri — service should use app.RedirectURIs[0].
		},
	}

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_WithClaimsLocales() {
	app := suite.testApp()
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	msg := &OAuthMessage{
		RequestType: oauth2const.TypeInitialAuthorizationRequest,
		RequestQueryParams: map[string]string{
			"client_id":      "test-client-id",
			"redirect_uri":   "https://client.example.com/callback",
			"response_type":  "code",
			"scope":          "openid read write",
			"claims_locales": "en-US fr-CA",
		},
	}

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_SetsRuntimeRequiredAttrs() {
	app := suite.testApp()
	app.Token = &inboundmodel.OAuthTokenConfig{
		AccessToken: &inboundmodel.AccessTokenConfig{
			UserAttributes: []string{"user_id"},
		},
		IDToken: &inboundmodel.IDTokenConfig{
			UserAttributes: []string{"email"},
		},
	}
	app.UserInfo = &inboundmodel.UserInfoConfig{
		UserAttributes: []string{"phone_number"},
	}

	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything,
		mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initContext *flowexec.FlowInitContext) {
			assert.Equal(suite.T(), "test-app-id", initContext.ApplicationID)
			assert.Equal(suite.T(), string(flowcm.FlowTypeAuthentication), initContext.FlowType)
			assert.Equal(suite.T(), "test-client-id", initContext.RuntimeData[flowcm.RuntimeKeyClientID])
			assert.ElementsMatch(suite.T(), []string{"email"},
				strings.Fields(initContext.RuntimeData[flowcm.RuntimeKeyRequiredEssentialAttributes]))
			assert.ElementsMatch(suite.T(), []string{"user_id", "phone_number"},
				strings.Fields(initContext.RuntimeData[flowcm.RuntimeKeyRequiredOptionalAttributes]))
		}).
		Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	msg := &OAuthMessage{
		RequestType: oauth2const.TypeInitialAuthorizationRequest,
		RequestQueryParams: map[string]string{
			"client_id":     "test-client-id",
			"redirect_uri":  "https://client.example.com/callback",
			"response_type": "code",
			"scope":         "openid",
			"claims":        `{"id_token":{"email":{"essential":true}},"userinfo":{"phone_number":{}}}`,
		},
	}

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "test-flow-id", result.QueryParams[oauth2const.ExecutionID])
}

func (suite *AuthorizeServiceTestSuite) TestHandleAuthorizationCallback_InvalidAuthID() {
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, "invalid-key").Return(false, authRequestContext{}, nil)

	svc := suite.newService()
	redirectURI, authErr := svc.HandleAuthorizationCallback(context.Background(), "invalid-key", "test-assertion")

	assert.Empty(suite.T(), redirectURI)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorInvalidRequest, authErr.Code)
}

func (suite *AuthorizeServiceTestSuite) TestHandleAuthorizationCallback_StoreError() {
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, "db-fail-key").
		Return(false, authRequestContext{}, errors.New("db connection error"))

	svc := suite.newService()
	redirectURI, authErr := svc.HandleAuthorizationCallback(context.Background(), "db-fail-key", "test-assertion")

	assert.Empty(suite.T(), redirectURI)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorServerError, authErr.Code)
}

func (suite *AuthorizeServiceTestSuite) TestHandleAuthorizationCallback_MissingAssertion() {
	authCtx := authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "test-client",
			RedirectURI: "https://client.example.com/callback",
			State:       "test-state",
		},
	}
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, testAuthID).Return(true, authCtx, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)

	svc := suite.newService()
	redirectURI, authErr := svc.HandleAuthorizationCallback(context.Background(), testAuthID, "")

	assert.Empty(suite.T(), redirectURI)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorInvalidRequest, authErr.Code)
	assert.Equal(suite.T(), "test-state", authErr.State)
	assert.True(suite.T(), authErr.SendErrorToClient)
	assert.Equal(suite.T(), "https://client.example.com/callback", authErr.ClientRedirectURI)
}

func (suite *AuthorizeServiceTestSuite) TestHandleAuthorizationCallback_InvalidAssertionSignature() {
	authCtx := authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "test-client",
			RedirectURI: "https://client.example.com/callback",
			State:       "test-state",
		},
	}
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, testAuthID).Return(true, authCtx, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)
	suite.mockJWTService.EXPECT().VerifyJWT("invalid-assertion", "", "").Return(&jwt.ErrorInvalidTokenSignature)

	svc := suite.newService()
	redirectURI, authErr := svc.HandleAuthorizationCallback(context.Background(), testAuthID, "invalid-assertion")

	assert.Empty(suite.T(), redirectURI)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorInvalidRequest, authErr.Code)
	assert.Equal(suite.T(), "test-state", authErr.State)
	assert.True(suite.T(), authErr.SendErrorToClient)
	assert.Equal(suite.T(), "https://client.example.com/callback", authErr.ClientRedirectURI)
}

func (suite *AuthorizeServiceTestSuite) TestHandleAuthorizationCallback_FailedToDecodeAssertion() {
	authCtx := authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "test-client",
			RedirectURI: "https://client.example.com/callback",
			State:       "test-state",
		},
	}
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, testAuthID).Return(true, authCtx, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)
	// VerifyJWT succeeds but "not.valid.jwt" cannot be decoded as a valid JWT payload.
	suite.mockJWTService.EXPECT().VerifyJWT("not.valid.jwt", "", "").Return(nil)

	svc := suite.newService()
	redirectURI, authErr := svc.HandleAuthorizationCallback(context.Background(), testAuthID, "not.valid.jwt")

	assert.Empty(suite.T(), redirectURI)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorServerError, authErr.Code)
	assert.Equal(suite.T(), "Failed to process authorization request", authErr.Message)
	assert.Equal(suite.T(), "test-state", authErr.State)
	assert.True(suite.T(), authErr.SendErrorToClient)
	assert.Equal(suite.T(), "https://client.example.com/callback", authErr.ClientRedirectURI)
}

func (suite *AuthorizeServiceTestSuite) TestHandleAuthorizationCallback_PersistAuthCodeError() {
	authCtx := authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "test-client",
			RedirectURI: "https://client.example.com/callback",
			State:       "test-state",
		},
	}
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, testAuthID).Return(true, authCtx, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)
	suite.mockJWTService.EXPECT().VerifyJWT(svcJWTWithIat, "", "").Return(nil)
	suite.mockAuthzCodeStore.EXPECT().
		InsertAuthorizationCode(mock.Anything, mock.Anything).
		Return(errors.New("db error"))

	svc := suite.newService()
	redirectURI, authErr := svc.HandleAuthorizationCallback(context.Background(), testAuthID, svcJWTWithIat)

	assert.Empty(suite.T(), redirectURI)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorServerError, authErr.Code)
	assert.Equal(suite.T(), "test-state", authErr.State)
	assert.True(suite.T(), authErr.SendErrorToClient)
	assert.Equal(suite.T(), "https://client.example.com/callback", authErr.ClientRedirectURI)
}

func (suite *AuthorizeServiceTestSuite) TestHandleAuthorizationCallback_Success() {
	authCtx := authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "test-client",
			RedirectURI: "https://client.example.com/callback",
		},
	}
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, testAuthID).Return(true, authCtx, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)
	suite.mockJWTService.EXPECT().VerifyJWT(svcJWTWithIat, "", "").Return(nil)
	suite.mockAuthzCodeStore.EXPECT().InsertAuthorizationCode(mock.Anything, mock.Anything).Return(nil)

	svc := suite.newService()
	redirectURI, authErr := svc.HandleAuthorizationCallback(context.Background(), testAuthID, svcJWTWithIat)

	assert.Nil(suite.T(), authErr)
	assert.Contains(suite.T(), redirectURI, "https://client.example.com/callback")
	assert.Contains(suite.T(), redirectURI, "code=")
	assert.Contains(suite.T(), redirectURI, "iss=https%3A%2F%2Flocalhost%3A8090")
	assert.NotContains(suite.T(), redirectURI, "state=")
}

func (suite *AuthorizeServiceTestSuite) TestHandleAuthorizationCallback_WithState() {
	authCtx := authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "test-client",
			RedirectURI: "https://client.example.com/callback",
			State:       "test-state-123",
		},
	}
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, testAuthID).Return(true, authCtx, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)
	suite.mockJWTService.EXPECT().VerifyJWT(svcJWTWithIat, "", "").Return(nil)
	suite.mockAuthzCodeStore.EXPECT().InsertAuthorizationCode(mock.Anything, mock.Anything).Return(nil)

	svc := suite.newService()
	redirectURI, authErr := svc.HandleAuthorizationCallback(context.Background(), testAuthID, svcJWTWithIat)

	assert.Nil(suite.T(), authErr)
	assert.Contains(suite.T(), redirectURI, "code=")
	assert.Contains(suite.T(), redirectURI, "state=test-state-123")
	assert.Contains(suite.T(), redirectURI, "iss=https%3A%2F%2Flocalhost%3A8090")
}

func (suite *AuthorizeServiceTestSuite) TestHandleAuthorizationCallback_EmptyAuthorizedPermissions() {
	// svcJWTWithIat has only "sub" and "iat" — no authorized_permissions.
	// Permission scopes in the auth context should be cleared.
	authCtx := authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:         "test-client",
			RedirectURI:      "https://client.example.com/callback",
			PermissionScopes: []string{"read", "write"},
		},
	}
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, testAuthID).Return(true, authCtx, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)
	suite.mockJWTService.EXPECT().VerifyJWT(svcJWTWithIat, "", "").Return(nil)
	suite.mockAuthzCodeStore.EXPECT().InsertAuthorizationCode(mock.Anything, mock.Anything).Return(nil)

	svc := suite.newService()
	redirectURI, authErr := svc.HandleAuthorizationCallback(context.Background(), testAuthID, svcJWTWithIat)

	assert.Nil(suite.T(), authErr)
	assert.NotEmpty(suite.T(), redirectURI)
}

func (suite *AuthorizeServiceTestSuite) TestHandleAuthorizationCallback_CreateAuthCodeError() {
	// Empty ClientID in auth context → createAuthorizationCode will fail.
	authCtx := authRequestContext{
		OAuthParameters: oauth2model.OAuthParameters{
			ClientID:    "",
			RedirectURI: "https://client.example.com/callback",
		},
	}
	suite.mockAuthReqStore.EXPECT().GetRequest(mock.Anything, testAuthID).Return(true, authCtx, nil)
	suite.mockAuthReqStore.EXPECT().ClearRequest(mock.Anything, testAuthID).Return(nil)
	suite.mockJWTService.EXPECT().VerifyJWT(svcJWTMinimal, "", "").Return(nil)

	svc := suite.newService()
	redirectURI, authErr := svc.HandleAuthorizationCallback(context.Background(), testAuthID, svcJWTMinimal)

	assert.Empty(suite.T(), redirectURI)
	assert.NotNil(suite.T(), authErr)
	assert.Equal(suite.T(), oauth2const.ErrorServerError, authErr.Code)
}

func (suite *AuthorizeServiceTestSuite) TestGetAuthorizationCodeDetails_GetError() {
	suite.mockAuthzCodeStore.EXPECT().GetAuthorizationCode(mock.Anything, "code").
		Return(nil, errors.New("database error"))

	svc := suite.newService()
	result, err := svc.GetAuthorizationCodeDetails(context.Background(), "client-id", "code")

	assert.Nil(suite.T(), result)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "database error")
}

func (suite *AuthorizeServiceTestSuite) TestGetAuthorizationCodeDetails_NotFound() {
	suite.mockAuthzCodeStore.EXPECT().GetAuthorizationCode(mock.Anything, "invalid-code").
		Return(nil, errAuthorizationCodeNotFound)

	svc := suite.newService()
	result, err := svc.GetAuthorizationCodeDetails(context.Background(), "client-id", "invalid-code")

	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, errAuthorizationCodeNotFound)
}

func (suite *AuthorizeServiceTestSuite) TestGetAuthorizationCodeDetails_ClientIDMismatch() {
	authCode := &AuthorizationCode{
		CodeID:   "code-id-123",
		Code:     "valid-code",
		ClientID: "other-client-id",
		State:    AuthCodeStateActive,
	}
	suite.mockAuthzCodeStore.EXPECT().GetAuthorizationCode(mock.Anything, "valid-code").
		Return(authCode, nil)

	svc := suite.newService()
	result, err := svc.GetAuthorizationCodeDetails(context.Background(), "client-id", "valid-code")

	assert.Nil(suite.T(), result)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "client ID mismatch")
}

func (suite *AuthorizeServiceTestSuite) TestGetAuthorizationCodeDetails_ConsumeError() {
	record := &AuthorizationCode{
		CodeID:   "code-id-123",
		Code:     "code",
		ClientID: "client-id",
		State:    AuthCodeStateActive,
	}
	suite.mockAuthzCodeStore.EXPECT().GetAuthorizationCode(mock.Anything, "code").
		Return(record, nil)
	suite.mockAuthzCodeStore.EXPECT().ConsumeAuthorizationCode(mock.Anything, "code").
		Return(false, errors.New("database error"))

	svc := suite.newService()
	result, err := svc.GetAuthorizationCodeDetails(context.Background(), "client-id", "code")

	assert.Nil(suite.T(), result)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "database error")
}

func (suite *AuthorizeServiceTestSuite) TestGetAuthorizationCodeDetails_AlreadyConsumed() {
	record := &AuthorizationCode{
		CodeID:   "code-id-123",
		Code:     "code",
		ClientID: "client-id",
		State:    AuthCodeStateInactive,
	}
	suite.mockAuthzCodeStore.EXPECT().GetAuthorizationCode(mock.Anything, "code").
		Return(record, nil)
	suite.mockAuthzCodeStore.EXPECT().ConsumeAuthorizationCode(mock.Anything, "code").
		Return(false, nil)

	svc := suite.newService()
	result, err := svc.GetAuthorizationCodeDetails(context.Background(), "client-id", "code")

	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, errAuthorizationCodeAlreadyConsumed)
}

func (suite *AuthorizeServiceTestSuite) TestGetAuthorizationCodeDetails_Success() {
	record := &AuthorizationCode{
		CodeID:           "code-id-123",
		Code:             "valid-code",
		ClientID:         "client-id",
		AuthorizedUserID: "user-123",
		State:            AuthCodeStateActive,
	}
	suite.mockAuthzCodeStore.EXPECT().GetAuthorizationCode(mock.Anything, "valid-code").
		Return(record, nil)
	suite.mockAuthzCodeStore.EXPECT().ConsumeAuthorizationCode(mock.Anything, "valid-code").
		Return(true, nil)

	svc := suite.newService()
	result, err := svc.GetAuthorizationCodeDetails(context.Background(), "client-id", "valid-code")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "valid-code", result.Code)
	assert.Equal(suite.T(), "user-123", result.AuthorizedUserID)
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_NilApp() {
	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid", "profile"},
		nil,
		string(oauth2const.ResponseTypeCode),
		nil,
	)

	assert.Empty(suite.T(), accessTokenClaims)
	assert.Empty(suite.T(), idTokenClaims)
	assert.Empty(suite.T(), userInfoClaims)
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_NilTokenConfig() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token:    nil,
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid", "profile"},
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), accessTokenClaims)
	assert.Empty(suite.T(), idTokenClaims)
	assert.Empty(suite.T(), userInfoClaims)
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_AccessTokenClaimsOnly() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id", "org_id", "role"},
			},
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{}, // No OIDC scopes
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Len(suite.T(), accessTokenClaims, 3)
	assert.True(suite.T(), accessTokenClaims["user_id"])
	assert.True(suite.T(), accessTokenClaims["org_id"])
	assert.True(suite.T(), accessTokenClaims["role"])
	assert.Empty(suite.T(), idTokenClaims)
	assert.Empty(suite.T(), userInfoClaims)
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_NoOpenIDScope() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id"},
			},
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "name"},
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"profile"}, // OIDC scope but no openid
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	// Only access token claims should be returned
	assert.Len(suite.T(), accessTokenClaims, 1)
	assert.True(suite.T(), accessTokenClaims["user_id"])
	assert.Empty(suite.T(), idTokenClaims)
	assert.Empty(suite.T(), userInfoClaims)
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_StandardOIDCScopes_CodeFlow() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "email_verified", "name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "email_verified", "name", "picture"},
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid", "email"},
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), accessTokenClaims)
	assert.Empty(suite.T(), idTokenClaims)
	// In code flow, claims from scopes go to userinfo
	assert.Len(suite.T(), userInfoClaims, 2)
	assert.True(suite.T(), userInfoClaims["email"])
	assert.True(suite.T(), userInfoClaims["email_verified"])
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_StandardOIDCScopes_ImplicitFlow() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "email_verified", "name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "email_verified", "name", "picture"},
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid", "email"},
		nil,
		string(oauth2const.ResponseTypeIDToken), // Implicit flow - no access token
		app,
	)

	assert.Empty(suite.T(), accessTokenClaims)
	// In implicit flow (id_token only), claims from scopes go to id_token
	assert.Len(suite.T(), idTokenClaims, 2)
	assert.True(suite.T(), idTokenClaims["email"])
	assert.True(suite.T(), idTokenClaims["email_verified"])
	assert.Empty(suite.T(), userInfoClaims)
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_ClaimsParameter_IDToken() {
	claimsRequest := &oauth2model.ClaimsRequest{
		IDToken: map[string]*oauth2model.IndividualClaimRequest{
			"email": {},
			"name":  {},
		},
	}

	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "name", "picture"},
			},
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid"},
		claimsRequest,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), accessTokenClaims)
	assert.Len(suite.T(), idTokenClaims, 2)
	assert.True(suite.T(), idTokenClaims["email"])
	assert.True(suite.T(), idTokenClaims["name"])
	assert.Empty(suite.T(), userInfoClaims)
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_ClaimsParameter_UserInfo() {
	claimsRequest := &oauth2model.ClaimsRequest{
		UserInfo: map[string]*oauth2model.IndividualClaimRequest{
			"email":   {},
			"picture": {},
		},
	}

	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token:    &inboundmodel.OAuthTokenConfig{}, // Need Token config for the method to process claims
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "name", "picture"},
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid"},
		claimsRequest,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), accessTokenClaims)
	assert.Empty(suite.T(), idTokenClaims)
	assert.Len(suite.T(), userInfoClaims, 2)
	assert.True(suite.T(), userInfoClaims["email"])
	assert.True(suite.T(), userInfoClaims["picture"])
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_ClaimsParameter_FilteredByAllowedSet() {
	claimsRequest := &oauth2model.ClaimsRequest{
		IDToken: map[string]*oauth2model.IndividualClaimRequest{
			"email":     {},
			"name":      {},
			"not_found": {}, // Not in allowed set
		},
	}

	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "name"}, // not_found is not allowed
			},
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid"},
		claimsRequest,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), accessTokenClaims)
	// not_found should be filtered out
	assert.Len(suite.T(), idTokenClaims, 2)
	assert.True(suite.T(), idTokenClaims["email"])
	assert.True(suite.T(), idTokenClaims["name"])
	assert.False(suite.T(), idTokenClaims["not_found"])
	assert.Empty(suite.T(), userInfoClaims)
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_CustomScopeMapping() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"org_id", "org_name", "department"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"org_id", "org_name", "department"},
		},
		ScopeClaims: map[string][]string{
			"organization": {"org_id", "org_name"},
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid", "organization"},
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), accessTokenClaims)
	assert.Empty(suite.T(), idTokenClaims)
	// Custom scope claims go to userinfo in code flow
	assert.Len(suite.T(), userInfoClaims, 2)
	assert.True(suite.T(), userInfoClaims["org_id"])
	assert.True(suite.T(), userInfoClaims["org_name"])
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_CustomScopeOverridesStandardScope() {
	// If app defines custom mapping for a standard scope, it should override
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"custom_email", "email"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"custom_email", "email"},
		},
		ScopeClaims: map[string][]string{
			"email": {"custom_email"}, // Override standard email scope
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid", "email"},
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), accessTokenClaims)
	assert.Empty(suite.T(), idTokenClaims)
	// Should use custom mapping, not standard
	assert.Len(suite.T(), userInfoClaims, 1)
	assert.True(suite.T(), userInfoClaims["custom_email"])
	assert.False(suite.T(), userInfoClaims["email"])
	assert.False(suite.T(), userInfoClaims["email_verified"])
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_MultipleScopesCodeFlow() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id"},
			},
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "email_verified", "name", "picture"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "email_verified", "name", "picture", "phone_number"},
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid", "email", "profile"},
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	// Access token claims
	assert.Len(suite.T(), accessTokenClaims, 1)
	assert.True(suite.T(), accessTokenClaims["user_id"])

	// ID token claims should be empty (code flow)
	assert.Empty(suite.T(), idTokenClaims)

	// UserInfo claims from email and profile scopes
	assert.True(suite.T(), userInfoClaims["email"])
	assert.True(suite.T(), userInfoClaims["email_verified"])
	assert.True(suite.T(), userInfoClaims["name"])
	assert.True(suite.T(), userInfoClaims["picture"])
	// phone_number not requested via scope
	assert.False(suite.T(), userInfoClaims["phone_number"])
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_CompleteScenario() {
	claimsRequest := &oauth2model.ClaimsRequest{
		IDToken: map[string]*oauth2model.IndividualClaimRequest{
			"email": {},
		},
		UserInfo: map[string]*oauth2model.IndividualClaimRequest{
			"phone_number": {},
		},
	}

	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id", "role"},
			},
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "email_verified", "name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "email_verified", "name", "picture", "phone_number"},
		},
		ScopeClaims: map[string][]string{
			"custom": {"name"},
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid", "custom"},
		claimsRequest,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	// Access token claims from config
	assert.Len(suite.T(), accessTokenClaims, 2)
	assert.True(suite.T(), accessTokenClaims["user_id"])
	assert.True(suite.T(), accessTokenClaims["role"])

	// ID token claims from claims parameter
	assert.Len(suite.T(), idTokenClaims, 1)
	assert.True(suite.T(), idTokenClaims["email"])

	// UserInfo claims from custom scope + claims parameter
	assert.Len(suite.T(), userInfoClaims, 2)
	assert.True(suite.T(), userInfoClaims["name"])         // from custom scope
	assert.True(suite.T(), userInfoClaims["phone_number"]) // from claims parameter
}

func (suite *AuthorizeServiceTestSuite) TestDetermineClaimsForTokens_EmptyAllowedSets() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{}, // Empty allowed set
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{}, // Empty allowed set
		},
	}

	accessTokenClaims, idTokenClaims, userInfoClaims := determineClaimsForTokens(
		[]string{"openid", "email", "profile"},
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), accessTokenClaims)
	assert.Empty(suite.T(), idTokenClaims)
	assert.Empty(suite.T(), userInfoClaims)
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_NilApp() {
	essential, optional := getRequiredAttributes(
		[]string{"openid", "profile"},
		nil,
		string(oauth2const.ResponseTypeCode),
		nil,
	)

	assert.Empty(suite.T(), essential)
	assert.Empty(suite.T(), optional)
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_NilTokenConfig() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token:    nil,
	}

	essential, optional := getRequiredAttributes(
		[]string{"openid", "profile"},
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), essential)
	assert.Empty(suite.T(), optional)
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_AccessTokenOnly() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id", "role"},
			},
		},
	}

	essential, optional := getRequiredAttributes(
		[]string{},
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), essential)
	assert.NotEmpty(suite.T(), optional)
	assert.Contains(suite.T(), optional, "user_id")
	assert.Contains(suite.T(), optional, "role")
	// Should have exactly 2 space-separated values
	parts := strings.Fields(optional)
	assert.Len(suite.T(), parts, 2)
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_CodeFlowWithScopes() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id"},
			},
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "email_verified", "name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "email_verified", "name", "picture"},
		},
	}

	essential, optional := getRequiredAttributes(
		[]string{"openid", "email"},
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), essential)
	// Should include access token claim + email scope claims from userinfo
	assert.NotEmpty(suite.T(), optional)
	assert.Contains(suite.T(), optional, "user_id")
	assert.Contains(suite.T(), optional, "email")
	assert.Contains(suite.T(), optional, "email_verified")

	parts := strings.Fields(optional)
	assert.Len(suite.T(), parts, 3)
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_ImplicitFlowWithScopes() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "email_verified", "name"},
			},
		},
	}

	essential, optional := getRequiredAttributes(
		[]string{"openid", "email"},
		nil,
		string(oauth2const.ResponseTypeIDToken),
		app,
	)

	assert.Empty(suite.T(), essential)
	// In implicit flow, email scope claims go to id_token
	assert.NotEmpty(suite.T(), optional)
	assert.Contains(suite.T(), optional, "email")
	assert.Contains(suite.T(), optional, "email_verified")

	parts := strings.Fields(optional)
	assert.Len(suite.T(), parts, 2)
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_WithClaimsParameter() {
	claimsRequest := &oauth2model.ClaimsRequest{
		IDToken: map[string]*oauth2model.IndividualClaimRequest{
			"email": {},
		},
		UserInfo: map[string]*oauth2model.IndividualClaimRequest{
			"phone_number": {},
		},
	}

	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id"},
			},
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "phone_number"},
		},
	}

	essential, optional := getRequiredAttributes(
		[]string{"openid"},
		claimsRequest,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), essential)
	// Should include access token + id_token claim + userinfo claim
	assert.NotEmpty(suite.T(), optional)
	assert.Contains(suite.T(), optional, "user_id")
	assert.Contains(suite.T(), optional, "email")
	assert.Contains(suite.T(), optional, "phone_number")

	parts := strings.Fields(optional)
	assert.Len(suite.T(), parts, 3)
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_ClaimsParameterEssentialPrecedence() {
	claimsRequest := &oauth2model.ClaimsRequest{
		IDToken: map[string]*oauth2model.IndividualClaimRequest{
			"email": {Essential: true},
		},
		UserInfo: map[string]*oauth2model.IndividualClaimRequest{
			"email":        {},
			"phone_number": {Essential: true},
		},
	}

	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"email", "role"},
			},
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "phone_number"},
		},
	}

	essential, optional := getRequiredAttributes(
		[]string{"openid"},
		claimsRequest,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Contains(suite.T(), essential, "email")
	assert.Contains(suite.T(), essential, "phone_number")
	assert.Contains(suite.T(), optional, "role")
	assert.NotContains(suite.T(), optional, "email")
	assert.NotContains(suite.T(), optional, "phone_number")
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_DeduplicatesClaims() {
	claimsRequest := &oauth2model.ClaimsRequest{
		IDToken: map[string]*oauth2model.IndividualClaimRequest{
			"email": {},
		},
		UserInfo: map[string]*oauth2model.IndividualClaimRequest{
			"email": {}, // Same claim in both
		},
	}

	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"email"}, // Same claim in access token too
			},
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email"},
		},
	}

	essential, optional := getRequiredAttributes(
		[]string{"openid"},
		claimsRequest,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), essential)
	// Email should only appear once despite being in all three token types
	assert.Equal(suite.T(), "email", optional)
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_CustomScopeMapping() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"org_id", "org_name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"org_id", "org_name"},
		},
		ScopeClaims: map[string][]string{
			"organization": {"org_id", "org_name"},
		},
	}

	essential, optional := getRequiredAttributes(
		[]string{"openid", "organization"},
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), essential)
	assert.NotEmpty(suite.T(), optional)
	assert.Contains(suite.T(), optional, "org_id")
	assert.Contains(suite.T(), optional, "org_name")

	parts := strings.Fields(optional)
	assert.Len(suite.T(), parts, 2)
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_ComplexScenario() {
	claimsRequest := &oauth2model.ClaimsRequest{
		IDToken: map[string]*oauth2model.IndividualClaimRequest{
			"email": {},
		},
		UserInfo: map[string]*oauth2model.IndividualClaimRequest{
			"phone_number": {},
		},
	}

	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id", "role"},
			},
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "email_verified", "name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "email_verified", "name", "picture", "phone_number"},
		},
		ScopeClaims: map[string][]string{
			"custom": {"name"},
		},
	}

	essential, optional := getRequiredAttributes(
		[]string{"openid", "custom"},
		claimsRequest,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	assert.Empty(suite.T(), essential)
	// Should include:
	// - Access token: user_id, role
	// - ID token from claims param: email
	// - UserInfo from claims param: phone_number
	// - UserInfo from custom scope: name
	assert.NotEmpty(suite.T(), optional)
	assert.Contains(suite.T(), optional, "user_id")
	assert.Contains(suite.T(), optional, "role")
	assert.Contains(suite.T(), optional, "email")
	assert.Contains(suite.T(), optional, "phone_number")
	assert.Contains(suite.T(), optional, "name")

	parts := strings.Fields(optional)
	assert.Len(suite.T(), parts, 5)
}

func (suite *AuthorizeServiceTestSuite) TestGetRequiredAttributes_NoOpenIDScope() {
	app := &inboundmodel.OAuthClient{
		ID:       "test-app",
		ClientID: "test-client",
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id"},
			},
			IDToken: &inboundmodel.IDTokenConfig{
				UserAttributes: []string{"email", "name"},
			},
		},
	}

	essential, optional := getRequiredAttributes(
		[]string{"profile"}, // OIDC scope but no openid
		nil,
		string(oauth2const.ResponseTypeCode),
		app,
	)

	// Without openid scope, only access token claims should be included
	assert.Empty(suite.T(), essential)
	assert.Equal(suite.T(), "user_id", optional)
}

func (suite *AuthorizeServiceTestSuite) TestResolveScopeAttributes_UnknownScope() {
	result := resolveScopeAttributes("unknown_scope", map[string][]string{
		"custom": {"email"},
	})

	assert.Nil(suite.T(), result)
}

func (suite *AuthorizeServiceTestSuite) TestResolveAttrCacheTTL_RefreshAllowed_UsesMaxOfRefreshAndAccessValidity() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", &config.Config{
		JWT: config.JWTConfig{ValidityPeriod: 900},
		OAuth: config.OAuthConfig{
			RefreshToken:      config.RefreshTokenConfig{ValidityPeriod: 7200},
			AuthorizationCode: config.AuthorizationCodeConfig{ValidityPeriod: 600},
		},
	})

	app := &inboundmodel.OAuthClient{
		GrantTypes: []oauth2const.GrantType{
			oauth2const.GrantTypeAuthorizationCode,
			oauth2const.GrantTypeRefreshToken,
		},
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{ValidityPeriod: 3600},
		},
	}

	// Refresh token validity (7200) > access token validity (3600) → max(7200) + authCode(600) + buffer(60) = 7860.
	assert.Equal(suite.T(), int64(7860), resolveUserAttributesCacheTTL(app))
}

func (suite *AuthorizeServiceTestSuite) TestResolveAttrCacheTTL_RefreshTokenAllowed_UsesAccessTokenWhenLonger() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", &config.Config{
		JWT: config.JWTConfig{ValidityPeriod: 900},
		OAuth: config.OAuthConfig{
			RefreshToken:      config.RefreshTokenConfig{ValidityPeriod: 1800},
			AuthorizationCode: config.AuthorizationCodeConfig{ValidityPeriod: 600},
		},
	})

	app := &inboundmodel.OAuthClient{
		GrantTypes: []oauth2const.GrantType{
			oauth2const.GrantTypeAuthorizationCode,
			oauth2const.GrantTypeRefreshToken,
		},
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{ValidityPeriod: 7200},
		},
	}

	// Access token validity (7200) > refresh token validity (1800) → max(7200) + authCode(600) + buffer(60) = 7860.
	assert.Equal(suite.T(), int64(7860), resolveUserAttributesCacheTTL(app))
}

func (suite *AuthorizeServiceTestSuite) TestResolveUserAttributesCacheTTL_RefreshTokenAllowed_FallsBackToGlobalJWT() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", &config.Config{
		JWT: config.JWTConfig{ValidityPeriod: 900},
		OAuth: config.OAuthConfig{
			// RefreshToken.ValidityPeriod is 0 → ResolveTokenConfig falls back to global JWT validity.
			RefreshToken:      config.RefreshTokenConfig{ValidityPeriod: 0},
			AuthorizationCode: config.AuthorizationCodeConfig{ValidityPeriod: 600},
		},
	})

	app := &inboundmodel.OAuthClient{
		GrantTypes: []oauth2const.GrantType{
			oauth2const.GrantTypeAuthorizationCode,
			oauth2const.GrantTypeRefreshToken,
		},
	}

	// JWT fallback (900) + authCode(600) + buffer(60) = 1560.
	assert.Equal(suite.T(), int64(1560), resolveUserAttributesCacheTTL(app))
}

func (suite *AuthorizeServiceTestSuite) TestResolveAttrCacheTTL_RefreshTokenNotAllowed_UsesAccessTokenValidity() {
	app := &inboundmodel.OAuthClient{
		GrantTypes: []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{ValidityPeriod: 3600},
		},
	}

	// Access token validity (3600) + authCode(600) + buffer(60) = 4260.
	assert.Equal(suite.T(), int64(4260), resolveUserAttributesCacheTTL(app))
}

func (suite *AuthorizeServiceTestSuite) TestResolveAttrCacheTTL_NoRefreshToken_ZeroAccessTTL_FallsBackToGlobalJWT() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", &config.Config{
		JWT: config.JWTConfig{ValidityPeriod: 900},
		OAuth: config.OAuthConfig{
			AuthorizationCode: config.AuthorizationCodeConfig{ValidityPeriod: 600},
		},
	})

	app := &inboundmodel.OAuthClient{
		GrantTypes: []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		Token: &inboundmodel.OAuthTokenConfig{
			// ValidityPeriod 0 is treated as unset by ResolveTokenConfig → falls back to global JWT validity.
			AccessToken: &inboundmodel.AccessTokenConfig{ValidityPeriod: 0},
		},
	}

	// JWT fallback (900) + authCode(600) + buffer(60) = 1560.
	assert.Equal(suite.T(), int64(1560), resolveUserAttributesCacheTTL(app))
}

func (suite *AuthorizeServiceTestSuite) TestResolveAttrCacheTTL_NoRefreshToken_NilToken_FallsBackToGlobalJWT() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", &config.Config{
		JWT: config.JWTConfig{ValidityPeriod: 900},
		OAuth: config.OAuthConfig{
			AuthorizationCode: config.AuthorizationCodeConfig{ValidityPeriod: 600},
		},
	})

	app := &inboundmodel.OAuthClient{
		GrantTypes: []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		Token:      nil,
	}

	// JWT fallback (900) + authCode(600) + buffer(60) = 1560.
	assert.Equal(suite.T(), int64(1560), resolveUserAttributesCacheTTL(app))
}

func (suite *AuthorizeServiceTestSuite) TestResolveAttrCacheTTL_NoRefreshToken_NilAccessToken_FallsBackToGlobalJWT() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test", &config.Config{
		JWT: config.JWTConfig{ValidityPeriod: 900},
		OAuth: config.OAuthConfig{
			AuthorizationCode: config.AuthorizationCodeConfig{ValidityPeriod: 600},
		},
	})

	app := &inboundmodel.OAuthClient{
		GrantTypes: []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
		Token:      &inboundmodel.OAuthTokenConfig{AccessToken: nil},
	}

	// JWT fallback (900) + authCode(600) + buffer(60) = 1560.
	assert.Equal(suite.T(), int64(1560), resolveUserAttributesCacheTTL(app))
}

// determineClaimsForTokens is a test helper retained to keep existing token-claim tests readable.
// It mirrors the token-specific split (access_token / id_token / userinfo) on top of current helper functions.
func determineClaimsForTokens(oidcScopes []string, claimsRequest *oauth2model.ClaimsRequest,
	responseType string, app *inboundmodel.OAuthClient) (
	map[string]bool, map[string]bool, map[string]bool) {
	accessTokenClaims := make(map[string]bool)
	idTokenClaims := make(map[string]bool)
	userInfoClaims := make(map[string]bool)

	if app == nil || app.Token == nil {
		return accessTokenClaims, idTokenClaims, userInfoClaims
	}

	if app.Token.AccessToken != nil {
		for _, claim := range app.Token.AccessToken.UserAttributes {
			accessTokenClaims[claim] = true
		}
	}

	hasOpenID := false
	for _, scope := range oidcScopes {
		if scope == oauth2const.ScopeOpenID {
			hasOpenID = true
			break
		}
	}
	if !hasOpenID {
		return accessTokenClaims, idTokenClaims, userInfoClaims
	}

	idTokenAllowedSet := buildIDTokenAllowedSet(app.Token.IDToken)
	userInfoAllowedSet := buildUserInfoAllowedSet(app.UserInfo)

	if claimsRequest != nil {
		if claimsRequest.IDToken != nil && idTokenAllowedSet != nil {
			for claim := range claimsRequest.IDToken {
				if idTokenAllowedSet[claim] {
					idTokenClaims[claim] = true
				}
			}
		}
		if claimsRequest.UserInfo != nil && userInfoAllowedSet != nil {
			for claim := range claimsRequest.UserInfo {
				if userInfoAllowedSet[claim] {
					userInfoClaims[claim] = true
				}
			}
		}
	}

	for _, scope := range oidcScopes {
		scopeAttributes := resolveScopeAttributes(scope, app.ScopeClaims)
		for _, attribute := range scopeAttributes {
			if responseType == string(oauth2const.ResponseTypeIDToken) {
				if idTokenAllowedSet != nil && idTokenAllowedSet[attribute] {
					idTokenClaims[attribute] = true
				}
			} else {
				if userInfoAllowedSet != nil && userInfoAllowedSet[attribute] {
					userInfoClaims[attribute] = true
				}
			}
		}
	}

	return accessTokenClaims, idTokenClaims, userInfoClaims
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_NoAcrValues_NoDefaults() {
	app := suite.testApp()
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything,
		mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initContext *flowexec.FlowInitContext) {
			assert.NotContains(suite.T(), initContext.RuntimeData, flowcm.RuntimeKeyRequestedAuthClasses)
		}).
		Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), suite.testMsg())

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_AcrValues_NoDefaults() {
	app := suite.testApp()
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything,
		mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initContext *flowexec.FlowInitContext) {
			assert.NotContains(suite.T(), initContext.RuntimeData, flowcm.RuntimeKeyRequestedAuthClasses)
		}).
		Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	msg := suite.testMsg()
	msg.RequestQueryParams[oauth2const.RequestParamAcrValues] =
		"urn:thunder:acr:password urn:thunder:acr:generated-code"

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_AcrValues_AllInDefaults() {
	app := suite.testApp()
	app.AcrValues = []string{
		"urn:thunder:acr:password",
		"urn:thunder:acr:generated-code",
		"urn:thunder:acr:biometrics",
	}

	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything,
		mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initContext *flowexec.FlowInitContext) {
			assert.Equal(suite.T(),
				[]string{"urn:thunder:acr:generated-code", "urn:thunder:acr:password"},
				strings.Fields(initContext.RuntimeData[flowcm.RuntimeKeyRequestedAuthClasses]))
		}).
		Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	msg := suite.testMsg()
	msg.RequestQueryParams[oauth2const.RequestParamAcrValues] =
		"urn:thunder:acr:generated-code urn:thunder:acr:password"

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_AcrValues_SomeNotInDefaults() {
	app := suite.testApp()
	app.AcrValues = []string{
		"urn:thunder:acr:password",
		"urn:thunder:acr:generated-code",
	}

	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything,
		mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initContext *flowexec.FlowInitContext) {
			effective := initContext.RuntimeData[flowcm.RuntimeKeyRequestedAuthClasses]
			assert.Equal(suite.T(), []string{"urn:thunder:acr:password"}, strings.Fields(effective))
			assert.NotContains(suite.T(), effective, "urn:thunder:acr:biometrics")
		}).
		Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	msg := suite.testMsg()
	msg.RequestQueryParams[oauth2const.RequestParamAcrValues] = "urn:thunder:acr:password urn:thunder:acr:biometrics"

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_AcrValues_NoneInDefaults() {
	defaults := []string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"}
	app := suite.testApp()
	app.AcrValues = defaults

	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything,
		mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initContext *flowexec.FlowInitContext) {
			assert.ElementsMatch(suite.T(), defaults,
				strings.Fields(initContext.RuntimeData[flowcm.RuntimeKeyRequestedAuthClasses]))
		}).
		Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	msg := suite.testMsg()
	msg.RequestQueryParams[oauth2const.RequestParamAcrValues] =
		"urn:thunder:acr:biometrics urn:thunder:acr:linked-wallet"

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_AcrValues_DuplicatesDeduped() {
	app := suite.testApp()
	app.AcrValues = []string{
		"urn:thunder:acr:password",
		"urn:thunder:acr:generated-code",
	}

	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything,
		mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initContext *flowexec.FlowInitContext) {
			assert.Equal(suite.T(),
				[]string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"},
				strings.Fields(initContext.RuntimeData[flowcm.RuntimeKeyRequestedAuthClasses]))
		}).
		Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	msg := suite.testMsg()
	msg.RequestQueryParams[oauth2const.RequestParamAcrValues] =
		"urn:thunder:acr:password urn:thunder:acr:password urn:thunder:acr:generated-code"

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
}

func (suite *AuthorizeServiceTestSuite) TestHandleInitialAuthorizationRequest_AcrValues_SingleDefault() {
	app := suite.testApp()
	app.AcrValues = []string{"urn:thunder:acr:password"}

	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "test-client-id").Return(app, nil)
	suite.mockValidator.On("validateInitialAuthorizationRequest", mock.Anything, app).
		Return(false, "", "")
	suite.mockFlowExecService.EXPECT().InitiateFlow(mock.Anything,
		mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initContext *flowexec.FlowInitContext) {
			assert.Equal(suite.T(),
				"urn:thunder:acr:password",
				initContext.RuntimeData[flowcm.RuntimeKeyRequestedAuthClasses])
		}).
		Return("test-flow-id", nil)
	suite.mockAuthReqStore.EXPECT().AddRequest(mock.Anything, mock.Anything).Return(testAuthID, nil)

	msg := suite.testMsg()
	msg.RequestQueryParams[oauth2const.RequestParamAcrValues] = "urn:thunder:acr:password"

	svc := suite.newService()
	result, authErr := svc.HandleInitialAuthorizationRequest(context.Background(), msg)

	assert.Nil(suite.T(), authErr)
	assert.NotNil(suite.T(), result)
}
