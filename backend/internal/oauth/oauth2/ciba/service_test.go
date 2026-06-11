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

	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

const testUserID = "user-1"

type CIBAServiceTestSuite struct {
	suite.Suite
	mockStore         *CIBARequestStoreInterfaceMock
	mockFlowExec      *flowexecmock.FlowExecServiceInterfaceMock
	mockJWTService    *jwtmock.JWTServiceInterfaceMock
	mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock
	mockResourceSvc   *resourcemock.ResourceServiceInterfaceMock
	service           CIBAServiceInterface
	oauthApp          *inboundmodel.OAuthClient
}

func TestCIBAServiceTestSuite(t *testing.T) {
	suite.Run(t, new(CIBAServiceTestSuite))
}

func (suite *CIBAServiceTestSuite) SetupTest() {
	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockStore = NewCIBARequestStoreInterfaceMock(suite.T())
	suite.mockFlowExec = flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockResourceSvc = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.service = newCIBAService(suite.mockStore, suite.mockFlowExec,
		suite.mockJWTService, suite.mockInboundClient, suite.mockResourceSvc)
	suite.oauthApp = &inboundmodel.OAuthClient{
		ID:         "app-1",
		ClientID:   "client-1",
		GrantTypes: []oauth2const.GrantType{oauth2const.GrantTypeCIBA},
	}
}

func (suite *CIBAServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *CIBAServiceTestSuite) expectFlowInitiateSuccess() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.Anything).
		Return(&flowexec.FlowStep{
			ExecutionID: "exec-1",
			Status:      flowcm.FlowStatusIncomplete,
		}, nil)
}

func (suite *CIBAServiceTestSuite) expectStoreAddSuccess() {
	suite.mockStore.EXPECT().Add(mock.Anything, mock.MatchedBy(func(r *CIBAAuthRequest) bool {
		return r.ClientID == "client-1" && r.State == CIBAStatePending && r.UserID == ""
	})).Return(nil)
}

// -------------------------------------------------------------------
// InitiateBackchannelAuth tests
// -------------------------------------------------------------------

func (suite *CIBAServiceTestSuite) TestInitiate_Success() {
	suite.expectFlowInitiateSuccess()
	suite.mockStore.EXPECT().Add(mock.Anything, mock.MatchedBy(func(r *CIBAAuthRequest) bool {
		return r.ClientID == "client-1" &&
			r.State == CIBAStatePending &&
			r.UserID == "" &&
			r.StandardScopes == "openid profile"
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
}

func (suite *CIBAServiceTestSuite) TestInitiate_ExpirySecondsMatchesResolvedExpiry() {
	// ExpirySeconds in FlowInitContext must equal the computed expiresIn so the
	// flow context and the CIBA auth_req_id expire at the same time.
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			return initCtx.ExpirySeconds == oauth2const.CIBADefaultExpiresInSeconds
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: flowcm.FlowStatusIncomplete}, nil)
	suite.expectStoreAddSuccess()

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.Equal(int64(oauth2const.CIBADefaultExpiresInSeconds), resp.ExpiresIn)
}

func (suite *CIBAServiceTestSuite) TestInitiate_CustomExpiryPassedToFlow() {
	const customExpiry = "300"
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			return initCtx.ExpirySeconds == 300
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: flowcm.FlowStatusIncomplete}, nil)
	suite.expectStoreAddSuccess()

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint:       "alice",
		Scope:           "openid",
		RequestedExpiry: customExpiry,
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.Equal(int64(300), resp.ExpiresIn)
}

func (suite *CIBAServiceTestSuite) TestInitiate_AuthReqIDInjectedIntoRuntimeData() {
	var capturedAuthReqID string
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			capturedAuthReqID = initCtx.RuntimeData[flowcm.RuntimeKeyCIBAAuthReqID]
			return capturedAuthReqID != ""
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: flowcm.FlowStatusIncomplete}, nil)
	suite.mockStore.EXPECT().Add(mock.Anything, mock.MatchedBy(func(r *CIBAAuthRequest) bool {
		return r.AuthReqID == capturedAuthReqID
	})).Return(nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.Equal(capturedAuthReqID, resp.AuthReqID)
}

func (suite *CIBAServiceTestSuite) TestInitiate_LoginHintInInitialInputs() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			return initCtx.InitialInputs[flowcm.UserInputKeyLoginHint] == "alice"
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: flowcm.FlowStatusIncomplete}, nil)
	suite.expectStoreAddSuccess()

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

func (suite *CIBAServiceTestSuite) TestInitiate_ACRValuesPassedToRuntime() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			return initCtx.RuntimeData[flowcm.RuntimeKeyRequestedAuthClasses] == "urn:acr:silver"
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: flowcm.FlowStatusIncomplete}, nil)
	suite.expectStoreAddSuccess()

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
		ACRValues: "urn:acr:silver",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

func (suite *CIBAServiceTestSuite) TestInitiate_ACRValuesOmittedFromRuntimeWhenEmpty() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			_, present := initCtx.RuntimeData[flowcm.RuntimeKeyRequestedAuthClasses]
			return !present
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: flowcm.FlowStatusIncomplete}, nil)
	suite.expectStoreAddSuccess()

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

func (suite *CIBAServiceTestSuite) TestInitiate_StripsStandardScopesFromRuntime() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			perms := strings.Fields(initCtx.RuntimeData[flowcm.RuntimeKeyRequestedPermissions])
			return len(perms) == 2 &&
				slices.Contains(perms, "read") && slices.Contains(perms, "write") &&
				!slices.Contains(perms, "openid") && !slices.Contains(perms, "profile") &&
				!slices.Contains(perms, "email")
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: flowcm.FlowStatusIncomplete}, nil)
	suite.mockStore.EXPECT().Add(mock.Anything, mock.MatchedBy(func(r *CIBAAuthRequest) bool {
		return r.StandardScopes == "openid profile email"
	})).Return(nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid profile email read write",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

func (suite *CIBAServiceTestSuite) TestInitiate_InjectsRequiredAttributes() {
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

	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initCtx *flowexec.FlowInitContext) {
			suite.Empty(strings.Fields(initCtx.RuntimeData[flowcm.RuntimeKeyRequiredEssentialAttributes]))
			suite.ElementsMatch([]string{"email", "given_name", "family_name", "name"},
				strings.Fields(initCtx.RuntimeData[flowcm.RuntimeKeyRequiredOptionalAttributes]))
		}).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: flowcm.FlowStatusIncomplete}, nil)
	suite.mockStore.EXPECT().Add(mock.Anything, mock.Anything).Return(nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid profile email",
	}, app)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

func (suite *CIBAServiceTestSuite) TestInitiate_RequestedExpiryClamped() {
	suite.expectFlowInitiateSuccess()
	suite.expectStoreAddSuccess()

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
	suite.Equal(oauth2const.ErrorInvalidBindingMessage, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_BindingMessageNonPrintable() {
	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint:      "alice",
		Scope:          "openid",
		BindingMessage: "bad\x07message",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidBindingMessage, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_FlowInitiationFails() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.Anything).Return(nil,
		&serviceerror.ServiceError{Code: "FLOW-1"})

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorServerError, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_FlowErrorMapsToUnknownUser() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.Anything).Return(
		&flowexec.FlowStep{Status: flowcm.FlowStatusError, Error: &serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{DefaultValue: "User not found"},
		}}, nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "ghost",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorUnknownUserID, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_FlowErrorAmbiguousUserMapsToUnknownUser() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.Anything).Return(
		&flowexec.FlowStep{Status: flowcm.FlowStatusError, Error: &serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{DefaultValue: "User identity is ambiguous"},
		}}, nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "common",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorUnknownUserID, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_FlowErrorGenericMapsToServerError() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.Anything).Return(
		&flowexec.FlowStep{Status: flowcm.FlowStatusError, Error: &serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{DefaultValue: "something else"},
		}}, nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorServerError, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_StorePersistenceFails() {
	suite.expectFlowInitiateSuccess()
	suite.mockStore.EXPECT().Add(mock.Anything, mock.Anything).Return(errors.New("db error"))

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorServerError, cibaErr.Code)
}

// -------------------------------------------------------------------
// resolveExpiresIn tests
// -------------------------------------------------------------------

func (suite *CIBAServiceTestSuite) TestResolveExpiresIn_Empty() {
	suite.Equal(int64(oauth2const.CIBADefaultExpiresInSeconds), resolveExpiresIn(""))
}

func (suite *CIBAServiceTestSuite) TestResolveExpiresIn_InvalidString() {
	suite.Equal(int64(oauth2const.CIBADefaultExpiresInSeconds), resolveExpiresIn("not-a-number"))
}

func (suite *CIBAServiceTestSuite) TestResolveExpiresIn_NegativeValue() {
	suite.Equal(int64(oauth2const.CIBADefaultExpiresInSeconds), resolveExpiresIn("-1"))
}

func (suite *CIBAServiceTestSuite) TestResolveExpiresIn_ZeroValue() {
	suite.Equal(int64(oauth2const.CIBADefaultExpiresInSeconds), resolveExpiresIn("0"))
}

func (suite *CIBAServiceTestSuite) TestResolveExpiresIn_ValidWithinBounds() {
	suite.Equal(int64(60), resolveExpiresIn("60"))
}

func (suite *CIBAServiceTestSuite) TestResolveExpiresIn_ExceedsMax() {
	suite.Equal(int64(oauth2const.CIBAMaxExpiresInSeconds), resolveExpiresIn("999999"))
}

func (suite *CIBAServiceTestSuite) TestResolveExpiresIn_WithWhitespace() {
	suite.Equal(int64(30), resolveExpiresIn("  30  "))
}

// -------------------------------------------------------------------
// resolveUserAttributesCacheTTL tests
// -------------------------------------------------------------------

func (suite *CIBAServiceTestSuite) TestResolveUserAttributesCacheTTL_RefreshTokenBranchTakesPrecedence() {
	app := &inboundmodel.OAuthClient{
		GrantTypes: []oauth2const.GrantType{oauth2const.GrantTypeCIBA, oauth2const.GrantTypeRefreshToken},
		Token: &inboundmodel.OAuthTokenConfig{
			AccessToken:  &inboundmodel.AccessTokenConfig{ValidityPeriod: 3600},
			RefreshToken: &inboundmodel.RefreshTokenConfig{ValidityPeriod: 86400},
		},
	}
	ttl := resolveUserAttributesCacheTTL(app)
	suite.Greater(ttl, int64(86400))
}

// -------------------------------------------------------------------
// resolveExpectedAudience tests
// -------------------------------------------------------------------

func (suite *CIBAServiceTestSuite) TestResolveExpectedAudience_NilApp() {
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-1").
		Return(nil, nil)

	assertion := buildTestAssertion(map[string]interface{}{
		"sub":              testUserID,
		"ciba_auth_req_id": "auth-req-1",
		"iat":              float64(time.Now().Unix()),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "", "").Return(nil)
	suite.mockStore.EXPECT().MarkAuthenticated(
		mock.Anything, "auth-req-1", testUserID,
		mock.AnythingOfType("string"), "", "", mock.AnythingOfType("time.Time")).Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.Nil(cibaErr)
}

// -------------------------------------------------------------------
// getRequiredOptionalAttributes tests
// -------------------------------------------------------------------

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

// -------------------------------------------------------------------
// HandleCallback tests
// -------------------------------------------------------------------

func (suite *CIBAServiceTestSuite) expectAudienceResolution() {
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-1").
		Return(&inboundmodel.OAuthClient{ID: "app-1", ClientID: "client-1"}, nil)
}

func (suite *CIBAServiceTestSuite) pendingRecord() *CIBAAuthRequest {
	return &CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		ClientID:   "client-1",
		State:      CIBAStatePending,
		ExpiryTime: time.Now().Add(2 * time.Minute),
	}
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
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "app-1", "").Return(nil)
	suite.mockStore.EXPECT().MarkAuthenticated(
		mock.Anything, "auth-req-1", testUserID, mock.AnythingOfType("string"),
		"cache-1", "urn:acr:pwd",
		mock.MatchedBy(func(authTime time.Time) bool { return authTime.Unix() == iat })).Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.Nil(cibaErr)
}

func (suite *CIBAServiceTestSuite) TestCallback_SubMissing() {
	assertion := buildTestAssertion(map[string]interface{}{
		"aci":              "cache-1",
		"ciba_auth_req_id": "auth-req-1",
		"iat":              float64(time.Now().Unix()),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "app-1", "").Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorAccessDenied, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_BindingMismatch() {
	assertion := buildTestAssertion(map[string]interface{}{
		"sub":              testUserID,
		"aci":              "cache-1",
		"ciba_auth_req_id": "other-req",
		"iat":              float64(time.Now().Unix()),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "app-1", "").Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorAccessDenied, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_BindingClaimMissing() {
	assertion := buildTestAssertion(map[string]interface{}{
		"sub": testUserID,
		"aci": "cache-1",
		"iat": float64(time.Now().Unix()),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "app-1", "").Return(nil)

	cibaErr := suite.service.HandleCallback(context.Background(), "auth-req-1", assertion)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorAccessDenied, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestCallback_AudienceResolutionFailureStillBinds() {
	iat := time.Now().Unix()
	assertion := buildTestAssertion(map[string]interface{}{
		"sub":              testUserID,
		"aci":              "cache-1",
		"ciba_auth_req_id": "auth-req-1",
		"iat":              float64(iat),
	})
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-1").
		Return(nil, errors.New("lookup failed"))
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "", "").Return(nil)
	suite.mockStore.EXPECT().MarkAuthenticated(
		mock.Anything, "auth-req-1", testUserID, mock.AnythingOfType("string"),
		"cache-1", "", mock.AnythingOfType("time.Time")).Return(nil)

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

func (suite *CIBAServiceTestSuite) TestCallback_DecodeError() {
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(&CIBAAuthRequest{
		AuthReqID:  "auth-req-1",
		ClientID:   "client-1",
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
	suite.mockStore.EXPECT().GetByID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)
	suite.expectAudienceResolution()
	suite.mockJWTService.EXPECT().VerifyJWT(mock.Anything, assertion, "app-1", "").Return(nil)
	suite.mockStore.EXPECT().MarkAuthenticated(
		mock.Anything, "auth-req-1", testUserID, mock.AnythingOfType("string"),
		"cache-1", "", mock.AnythingOfType("time.Time")).Return(errors.New("db error"))

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
