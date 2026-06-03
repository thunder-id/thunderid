/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package ciba

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/scopemock"
)

const testUserID = "user-1"

type CIBAServiceTestSuite struct {
	suite.Suite
	mockStore         *CIBARequestStoreInterfaceMock
	mockFlowExec      *flowexecmock.FlowExecServiceInterfaceMock
	mockEntityProv    *entityprovidermock.EntityProviderInterfaceMock
	mockJWTService    *jwtmock.JWTServiceInterfaceMock
	mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock
	mockScopeVal      *scopemock.ScopeValidatorInterfaceMock
	service           CIBAServiceInterface
	oauthApp          *inboundmodel.OAuthClient
}

func TestCIBAServiceTestSuite(t *testing.T) {
	suite.Run(t, new(CIBAServiceTestSuite))
}

func (suite *CIBAServiceTestSuite) SetupTest() {
	testConfig := &config.Config{
		GateClient: config.GateClientConfig{
			Scheme:    "https",
			Hostname:  "localhost",
			Port:      9001,
			LoginPath: "/signin",
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockStore = NewCIBARequestStoreInterfaceMock(suite.T())
	suite.mockFlowExec = flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
	suite.mockEntityProv = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockScopeVal = scopemock.NewScopeValidatorInterfaceMock(suite.T())
	suite.service = newCIBAService(suite.mockStore, suite.mockFlowExec, suite.mockEntityProv,
		suite.mockJWTService, suite.mockInboundClient, suite.mockScopeVal)
	suite.oauthApp = &inboundmodel.OAuthClient{
		ID:         "app-1",
		ClientID:   "client-1",
		GrantTypes: []oauth2const.GrantType{oauth2const.GrantTypeCIBA},
	}
}

func (suite *CIBAServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// expectScopePassthrough configures the scope validator mock to return the requested scope unchanged,
// matching the current passthrough behavior of the production validator.
func (suite *CIBAServiceTestSuite) expectScopePassthrough() {
	suite.mockScopeVal.EXPECT().ValidateScopes(mock.Anything, mock.Anything, "client-1").
		RunAndReturn(func(_ context.Context, requestedScopes, _ string) (string, *scope.ScopeError) {
			return requestedScopes, nil
		})
}

func (suite *CIBAServiceTestSuite) TestInitiate_Success() {
	userID := testUserID
	suite.expectScopePassthrough()
	suite.mockEntityProv.EXPECT().IdentifyEntity(map[string]interface{}{"username": "alice"}).
		Return(&userID, nil)
	suite.mockFlowExec.EXPECT().InitiateFlow(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			return initCtx.ApplicationID == "app-1" &&
				initCtx.FlowType == "AUTHENTICATION" &&
				initCtx.RuntimeData["clientId"] == "client-1" &&
				initCtx.RuntimeData["requested_permissions"] == "" &&
				initCtx.RuntimeData["user_attributes_cache_ttl_seconds"] != "" &&
				initCtx.RuntimeData[flowcm.RuntimeKeyCIBAAuthReqID] != ""
		})).Return("exec-1", nil)
	suite.mockStore.EXPECT().Add(mock.Anything, mock.MatchedBy(func(r *CIBAAuthRequest) bool {
		return r.ClientID == "client-1" && r.UserID == testUserID &&
			r.State == CIBAStatePending && r.ExecutionID == "exec-1" && r.Scopes == "openid profile"
	})).Return(nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid profile",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
	suite.NotEmpty(resp.AuthReqID)
	suite.Equal(int64(oauth2const.CIBADefaultExpiresInSeconds), resp.ExpiresIn)
	suite.Equal(int64(oauth2const.CIBADefaultIntervalSeconds), resp.Interval)
	suite.Contains(resp.NotificationURL, "https://localhost:9001/signin")
	suite.Contains(resp.NotificationURL, "flowType=AUTHENTICATION")
	suite.Contains(resp.NotificationURL, "executionId=exec-1")
	suite.Contains(resp.NotificationURL, "auth_req_id="+resp.AuthReqID)
}

// TestInitiate_StripsStandardScopesFromRuntime asserts that standard OIDC scopes are excluded from
// the requested_permissions exposed to the flow (matching the authorization_code path via the shared
// SeparateOIDCAndNonOIDCScopes helper), while the record persists the full scope set.
func (suite *CIBAServiceTestSuite) TestInitiate_StripsStandardScopesFromRuntime() {
	userID := testUserID
	suite.expectScopePassthrough()
	suite.mockEntityProv.EXPECT().IdentifyEntity(mock.Anything).Return(&userID, nil)
	suite.mockFlowExec.EXPECT().InitiateFlow(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			perms := strings.Fields(initCtx.RuntimeData[flowcm.RuntimeKeyRequestedPermissions])
			// openid, profile, email are standard OIDC scopes and must be stripped; read and write remain.
			return len(perms) == 2 && slices.Contains(perms, "read") && slices.Contains(perms, "write") &&
				!slices.Contains(perms, "openid") && !slices.Contains(perms, "profile") &&
				!slices.Contains(perms, "email")
		})).Return("exec-1", nil)
	suite.mockStore.EXPECT().Add(mock.Anything, mock.MatchedBy(func(r *CIBAAuthRequest) bool {
		return r.Scopes == "openid profile email read write"
	})).Return(nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid profile email read write",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

// TestInitiate_ScopeValidationRejected asserts that a scope validation error is surfaced to the caller.
func (suite *CIBAServiceTestSuite) TestInitiate_ScopeValidationRejected() {
	suite.mockScopeVal.EXPECT().ValidateScopes(mock.Anything, mock.Anything, "client-1").
		Return("", &scope.ScopeError{Error: oauth2const.ErrorInvalidScope, ErrorDescription: "bad scope"})

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid profile",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidScope, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_InjectsRequiredAttributes() {
	userID := testUserID
	app := &inboundmodel.OAuthClient{
		ID:         "app-1",
		ClientID:   "client-1",
		GrantTypes: []oauth2const.GrantType{oauth2const.GrantTypeCIBA},
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"email", "given_name", "family_name", "name"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "given_name", "family_name", "name"},
		},
	}

	suite.expectScopePassthrough()
	suite.mockEntityProv.EXPECT().IdentifyEntity(mock.Anything).Return(&userID, nil)
	suite.mockFlowExec.EXPECT().InitiateFlow(mock.Anything, mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initCtx *flowexec.FlowInitContext) {
			suite.Empty(strings.Fields(initCtx.RuntimeData[flowcm.RuntimeKeyRequiredEssentialAttributes]))
			suite.ElementsMatch([]string{"email", "given_name", "family_name", "name"},
				strings.Fields(initCtx.RuntimeData[flowcm.RuntimeKeyRequiredOptionalAttributes]))
		}).Return("exec-1", nil)
	suite.mockStore.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid profile email",
	}, app)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

func (suite *CIBAServiceTestSuite) TestInitiate_BindingMessageIncludedInURL() {
	userID := testUserID
	suite.expectScopePassthrough()
	suite.mockEntityProv.EXPECT().IdentifyEntity(mock.Anything).Return(&userID, nil)
	suite.mockFlowExec.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("exec-1", nil)
	suite.mockStore.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint:      "alice",
		Scope:          "openid",
		BindingMessage: "W4SCT",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.Contains(resp.NotificationURL, "binding_message=W4SCT")
}

func (suite *CIBAServiceTestSuite) TestInitiate_RequestedExpiryClamped() {
	userID := testUserID
	suite.expectScopePassthrough()
	suite.mockEntityProv.EXPECT().IdentifyEntity(mock.Anything).Return(&userID, nil)
	suite.mockFlowExec.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("exec-1", nil)
	suite.mockStore.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint:       "alice",
		Scope:           "openid",
		RequestedExpiry: "100000",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.Equal(int64(oauth2const.CIBAMaxExpiresInSeconds), resp.ExpiresIn)
}

func (suite *CIBAServiceTestSuite) TestInitiate_UnauthorizedClient() {
	app := &inboundmodel.OAuthClient{
		ID:         "app-1",
		ClientID:   "client-1",
		GrantTypes: []oauth2const.GrantType{oauth2const.GrantTypeAuthorizationCode},
	}
	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, app)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorUnauthorizedClient, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_MissingLoginHint() {
	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		Scope: "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_MissingScope() {
	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_ScopeMissingOpenID() {
	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "profile",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidScope, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_BindingMessageTooLong() {
	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint:      "alice",
		Scope:          "openid",
		BindingMessage: strings.Repeat("a", cibaMaxBindingMessageLength+1),
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_BindingMessageNonPrintable() {
	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint:      "alice",
		Scope:          "openid",
		BindingMessage: "bad\x07message",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_UnknownUser() {
	suite.expectScopePassthrough()
	suite.mockEntityProv.EXPECT().IdentifyEntity(mock.Anything).Return(nil,
		&entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeEntityNotFound})

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "ghost",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorUnknownUserID, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_AmbiguousUser() {
	suite.expectScopePassthrough()
	suite.mockEntityProv.EXPECT().IdentifyEntity(mock.Anything).Return(nil,
		&entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeAmbiguousEntity})

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "common",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorUnknownUserID, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_EntityProviderSystemError() {
	suite.expectScopePassthrough()
	suite.mockEntityProv.EXPECT().IdentifyEntity(mock.Anything).Return(nil,
		&entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeSystemError})

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorServerError, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_FlowInitiationFails() {
	userID := testUserID
	suite.expectScopePassthrough()
	suite.mockEntityProv.EXPECT().IdentifyEntity(mock.Anything).Return(&userID, nil)
	suite.mockFlowExec.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("",
		&serviceerror.ServiceError{Code: "FLOW-1"})

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorServerError, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_StorePersistenceFails() {
	userID := testUserID
	suite.expectScopePassthrough()
	suite.mockEntityProv.EXPECT().IdentifyEntity(mock.Anything).Return(&userID, nil)
	suite.mockFlowExec.EXPECT().InitiateFlow(mock.Anything, mock.Anything).Return("exec-1", nil)
	suite.mockStore.EXPECT().Add(mock.Anything, mock.Anything).Return(errors.New("db error"))

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorServerError, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestGetRequiredOptionalAttributes_NilApp() {
	suite.Empty(getRequiredOptionalAttributes([]string{"openid", "profile"}, nil))
}

func (suite *CIBAServiceTestSuite) TestGetRequiredOptionalAttributes_AccessTokenAttributesOnly() {
	app := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id", "role"},
			},
		},
	}

	suite.ElementsMatch([]string{"user_id", "role"}, strings.Fields(getRequiredOptionalAttributes([]string{}, app)))
}

func (suite *CIBAServiceTestSuite) TestGetRequiredOptionalAttributes_ScopeDerivedFilteredByUserInfo() {
	app := &inboundmodel.OAuthClient{
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken: &inboundmodel.AccessTokenConfig{
				UserAttributes: []string{"user_id"},
			},
		},
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "name"},
		},
	}

	optional := getRequiredOptionalAttributes([]string{"openid", "email", "profile"}, app)

	// user_id from access token config, email from the email scope (allowed by UserInfo), and
	// name from the profile scope (allowed by UserInfo). email_verified is dropped (not in UserInfo).
	suite.ElementsMatch([]string{"user_id", "email", "name"}, strings.Fields(optional))
}

func (suite *CIBAServiceTestSuite) TestGetRequiredOptionalAttributes_ScopeDerivedSkippedWithoutOpenID() {
	app := &inboundmodel.OAuthClient{
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"email", "name"},
		},
	}

	suite.Empty(getRequiredOptionalAttributes([]string{"profile", "email"}, app))
}

func (suite *CIBAServiceTestSuite) TestGetRequiredOptionalAttributes_UsesAppScopeClaimsMapping() {
	app := &inboundmodel.OAuthClient{
		UserInfo: &inboundmodel.UserInfoConfig{
			UserAttributes: []string{"custom_attr"},
		},
		ScopeClaims: map[string][]string{
			"profile": {"custom_attr"},
		},
	}

	suite.ElementsMatch([]string{"custom_attr"},
		strings.Fields(getRequiredOptionalAttributes([]string{"openid", "profile"}, app)))
}

// expectAudienceResolution configures the inbound client mock to resolve client-1 to the app entity
// ID app-1, which the flow uses as the assertion audience.
func (suite *CIBAServiceTestSuite) expectAudienceResolution() {
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-1").
		Return(&inboundmodel.OAuthClient{ID: "app-1", ClientID: "client-1"}, nil)
}

func (suite *CIBAServiceTestSuite) TestCallback_Success() {
	iat := time.Now().Unix()
	assertion := buildTestAssertion(map[string]interface{}{
		"sub":                  testUserID,
		"aci":                  "cache-1",
		"completed_auth_class": "urn:acr:pwd",
		"ciba_auth_req_id":     "auth-req-1",
		"iat":                  float64(iat),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		ClientID:   "client-1",
		UserID:     testUserID,
		State:      CIBAStatePending,
		ExpiryTime: time.Now().Add(2 * time.Minute),
	}, nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "app-1", "").Return(nil)
	// The auth_time persisted (and later surfaced as the id_token auth_time) must equal the
	// assertion iat instant, not be shifted by the server's timezone offset.
	suite.mockStore.EXPECT().MarkAuthenticated(mock.Anything, "auth-req-1", "cache-1", "urn:acr:pwd",
		mock.MatchedBy(func(authTime time.Time) bool { return authTime.Unix() == iat })).Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.Nil(cibaErr)
}

// TestCallback_BindingMismatch asserts the callback rejects an assertion whose ciba_auth_req_id
// claim does not match the request being authenticated (cross-request assertion replay).
func (suite *CIBAServiceTestSuite) TestCallback_BindingMismatch() {
	assertion := buildTestAssertion(map[string]interface{}{
		"sub":              testUserID,
		"aci":              "cache-1",
		"ciba_auth_req_id": "other-req",
		"iat":              float64(time.Now().Unix()),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		ClientID:   "client-1",
		UserID:     testUserID,
		State:      CIBAStatePending,
		ExpiryTime: time.Now().Add(2 * time.Minute),
	}, nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "app-1", "").Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorAccessDenied, cibaErr.Code)
}

// TestCallback_BindingClaimMissing asserts the callback rejects an assertion that lacks the
// ciba_auth_req_id binding claim entirely.
func (suite *CIBAServiceTestSuite) TestCallback_BindingClaimMissing() {
	assertion := buildTestAssertion(map[string]interface{}{
		"sub": testUserID,
		"aci": "cache-1",
		"iat": float64(time.Now().Unix()),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		ClientID:   "client-1",
		UserID:     testUserID,
		State:      CIBAStatePending,
		ExpiryTime: time.Now().Add(2 * time.Minute),
	}, nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "app-1", "").Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorAccessDenied, cibaErr.Code)
}

// TestCallback_AudienceResolutionFailureStillBinds asserts that when the client lookup fails the
// audience check is skipped (empty expectedAud) but the ciba_auth_req_id binding still authenticates
// a matching assertion.
func (suite *CIBAServiceTestSuite) TestCallback_AudienceResolutionFailureStillBinds() {
	iat := time.Now().Unix()
	assertion := buildTestAssertion(map[string]interface{}{
		"sub":              testUserID,
		"aci":              "cache-1",
		"ciba_auth_req_id": "auth-req-1",
		"iat":              float64(iat),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		ClientID:   "client-1",
		UserID:     testUserID,
		State:      CIBAStatePending,
		ExpiryTime: time.Now().Add(2 * time.Minute),
	}, nil)
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-1").
		Return(nil, errors.New("lookup failed"))
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "", "").Return(nil)
	suite.mockStore.EXPECT().MarkAuthenticated(mock.Anything, "auth-req-1", "cache-1", "",
		mock.AnythingOfType("time.Time")).Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.Nil(cibaErr)
}

func (suite *CIBAServiceTestSuite) TestCallback_MissingParams() {
	cibaErr := suite.service.HandleCallback(context.Background(), "", "assertion")
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_RequestNotFound() {
	suite.mockStore.EXPECT().GetByID(mock.Anything, "missing").Return(nil, ErrCIBARequestNotFound)

	cibaErr := suite.service.HandleCallback(context.Background(), "missing", "assertion")
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_NotPending() {
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		UserID:     testUserID,
		State:      CIBAStateConsumed,
		ExpiryTime: time.Now().Add(2 * time.Minute),
	}, nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", "assertion")
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_Expired() {
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		UserID:     testUserID,
		State:      CIBAStatePending,
		ExpiryTime: time.Now().Add(-1 * time.Minute),
	}, nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", "assertion")
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorExpiredToken, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_BadSignature() {
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		ClientID:   "client-1",
		UserID:     testUserID,
		State:      CIBAStatePending,
		ExpiryTime: time.Now().Add(2 * time.Minute),
	}, nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, "bad-assertion", "app-1", "").Return(
		&serviceerror.ServiceError{Code: "JWT-1"})

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", "bad-assertion")
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_SubMismatch() {
	assertion := buildTestAssertion(map[string]interface{}{
		"sub":              "attacker",
		"aci":              "cache-1",
		"ciba_auth_req_id": "auth-req-1",
		"iat":              float64(time.Now().Unix()),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		ClientID:   "client-1",
		UserID:     testUserID,
		State:      CIBAStatePending,
		ExpiryTime: time.Now().Add(2 * time.Minute),
	}, nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "app-1", "").Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorAccessDenied, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_DecodeError() {
	// VerifyJWT is mocked to succeed, but the assertion is not a valid JWT structure,
	// so decoding the claims fails and yields a server error.
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		ClientID:   "client-1",
		UserID:     testUserID,
		State:      CIBAStatePending,
		ExpiryTime: time.Now().Add(2 * time.Minute),
	}, nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, "not-a-jwt", "app-1", "").Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", "not-a-jwt")
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorServerError, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_StoreLookupError() {
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(nil, errors.New("db error"))

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", "assertion")
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorServerError, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_MarkAuthenticatedError() {
	assertion := buildTestAssertion(map[string]interface{}{
		"sub":              testUserID,
		"aci":              "cache-1",
		"ciba_auth_req_id": "auth-req-1",
		"iat":              float64(time.Now().Unix()),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		ClientID:   "client-1",
		UserID:     testUserID,
		State:      CIBAStatePending,
		ExpiryTime: time.Now().Add(2 * time.Minute),
	}, nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "app-1", "").Return(nil)
	suite.mockStore.EXPECT().MarkAuthenticated(mock.Anything, "auth-req-1", "cache-1", "",
		mock.AnythingOfType("time.Time")).Return(errors.New("db error"))

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorServerError, cibaErr.Code)
}

// buildTestAssertion builds a JWT-shaped string (header.payload.signature) for decode-path testing.
// Signature verification is mocked, so the signature segment is a placeholder.
func buildTestAssertion(claims map[string]interface{}) string {
	header, _ := json.Marshal(map[string]interface{}{"alg": "RS256", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	enc := base64.RawURLEncoding
	return enc.EncodeToString(header) + "." + enc.EncodeToString(payload) + "." + enc.EncodeToString([]byte("sig"))
}
