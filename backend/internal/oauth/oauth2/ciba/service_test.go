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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

const testUserID = "user-1"

type CIBAServiceTestSuite struct {
	suite.Suite
	mockStore          *CIBARequestStoreInterfaceMock
	mockFlowExec       *flowexecmock.FlowExecServiceInterfaceMock
	mockJWTService     *jwtmock.JWTServiceInterfaceMock
	mockInboundClient  *inboundclientmock.InboundClientServiceInterfaceMock
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	mockResourceSvc    *resourcemock.ResourceServiceInterfaceMock
	service            CIBAServiceInterface
	oauthApp           *providers.OAuthClient
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
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockResourceSvc = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	actorProv := actorprovider.Initialize(suite.mockInboundClient, suite.mockEntityProvider, noopAuthnMgr())
	suite.service = newCIBAService(suite.mockStore, suite.mockFlowExec,
		suite.mockJWTService, actorProv, suite.mockResourceSvc, testhelpers.OAuthConfig())
	suite.oauthApp = &providers.OAuthClient{
		ID:         "app-1",
		ClientID:   "client-1",
		GrantTypes: []providers.GrantType{providers.GrantTypeCIBA},
	}
}

func (suite *CIBAServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *CIBAServiceTestSuite) expectFlowInitiateSuccess() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.Anything).
		Return(&flowexec.FlowStep{
			ExecutionID: "exec-1",
			Status:      providers.FlowStatusIncomplete,
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
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
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
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
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
			capturedAuthReqID = initCtx.RuntimeData[flowcm.RuntimeKeyAuthorizationRequestID]
			return capturedAuthReqID != ""
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
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
			return initCtx.InitialInputs[oauth2const.RequestParamLoginHint] == "alice"
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
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
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
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
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
	suite.expectStoreAddSuccess()

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		LoginHint: "alice",
		Scope:     "openid",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

func (suite *CIBAServiceTestSuite) TestInitiate_ForceConsentRepromptAlwaysSet() {
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			return initCtx.RuntimeData[flowcm.RuntimeKeyForceConsentReprompt] == "true"
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
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
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
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
	app := &providers.OAuthClient{
		ID:         "app-1",
		ClientID:   "client-1",
		GrantTypes: []providers.GrantType{providers.GrantTypeCIBA},
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{
					Attributes: []string{"email", "given_name", "family_name", "name"},
				},
			},
		},
		UserInfo: &providers.UserInfoConfig{
			UserAttributes: []string{"email", "given_name", "family_name", "name"},
		},
	}

	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.AnythingOfType("*flowexec.FlowInitContext")).
		Run(func(_ context.Context, initCtx *flowexec.FlowInitContext) {
			suite.Empty(strings.Fields(initCtx.RuntimeData[flowcm.RuntimeKeyRequiredEssentialAttributes]))
			suite.ElementsMatch([]string{"email", "given_name", "family_name", "name"},
				strings.Fields(initCtx.RuntimeData[flowcm.RuntimeKeyRequiredOptionalAttributes]))
		}).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
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
	app := &providers.OAuthClient{
		ID:         "app-1",
		ClientID:   "client-1",
		GrantTypes: []providers.GrantType{providers.GrantTypeAuthorizationCode},
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
		&tidcommon.ServiceError{Code: "FLOW-1"})

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
		&flowexec.FlowStep{Status: providers.FlowStatusError, Error: &tidcommon.ServiceError{
			Error: tidcommon.I18nMessage{DefaultValue: "User not found"},
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
		&flowexec.FlowStep{Status: providers.FlowStatusError, Error: &tidcommon.ServiceError{
			Error: tidcommon.I18nMessage{DefaultValue: "User identity is ambiguous"},
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
		&flowexec.FlowStep{Status: providers.FlowStatusError, Error: &tidcommon.ServiceError{
			Error: tidcommon.I18nMessage{DefaultValue: "something else"},
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
	app := &providers.OAuthClient{
		GrantTypes: []providers.GrantType{providers.GrantTypeCIBA, providers.GrantTypeRefreshToken},
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{ValidityPeriod: 3600},
			},
			RefreshToken: &providers.RefreshTokenConfig{ValidityPeriod: 86400},
		},
	}
	ttl := (&cibaService{cfg: testhelpers.OAuthConfig()}).resolveUserAttributesCacheTTL(app)
	suite.Greater(ttl, int64(86400))
}

// -------------------------------------------------------------------
// resolveExpectedAudience tests
// -------------------------------------------------------------------

func (suite *CIBAServiceTestSuite) TestResolveExpectedAudience_NilApp() {
	suite.mockInboundClient.EXPECT().GetOAuthClientByClientID(mock.Anything, "client-1").
		Return(nil, nil)

	assertion := buildTestAssertion(map[string]interface{}{
		"sub":                      testUserID,
		"authorization_request_id": "auth-req-1",
		"iat":                      float64(time.Now().Unix()),
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
	app := &providers.OAuthClient{
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{Attributes: []string{"user_id", "role"}},
			},
		},
	}
	suite.ElementsMatch([]string{"user_id", "role"}, strings.Fields(getRequiredOptionalAttributes([]string{}, app)))
}

func (suite *CIBAServiceTestSuite) TestGetRequiredOptionalAttributes_ScopeDerivedFilteredByUserInfo() {
	app := &providers.OAuthClient{
		Token: &providers.OAuthTokenConfig{
			AccessToken: &providers.AccessTokenConfig{
				UserConfig: &providers.AccessTokenSubConfig{Attributes: []string{"user_id"}},
			},
		},
		UserInfo: &providers.UserInfoConfig{
			UserAttributes: []string{"email", "name"},
		},
	}
	optional := getRequiredOptionalAttributes([]string{"openid", "email", "profile"}, app)
	suite.ElementsMatch([]string{"user_id", "email", "name"}, strings.Fields(optional))
}

func (suite *CIBAServiceTestSuite) TestGetRequiredOptionalAttributes_ScopeDerivedSkippedWithoutOpenID() {
	app := &providers.OAuthClient{
		UserInfo: &providers.UserInfoConfig{
			UserAttributes: []string{"email", "name"},
		},
	}
	suite.Empty(getRequiredOptionalAttributes([]string{"profile", "email"}, app))
}

func (suite *CIBAServiceTestSuite) TestGetRequiredOptionalAttributes_UsesAppScopeClaimsMapping() {
	app := &providers.OAuthClient{
		UserInfo: &providers.UserInfoConfig{
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
		Return(&providers.OAuthClient{ID: "app-1", ClientID: "client-1"}, nil)
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
		"sub":                      testUserID,
		"aci":                      "cache-1",
		"completed_auth_class":     "urn:acr:pwd",
		"authorization_request_id": "auth-req-1",
		"iat":                      float64(iat),
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
		"aci":                      "cache-1",
		"authorization_request_id": "auth-req-1",
		"iat":                      float64(time.Now().Unix()),
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
		"sub":                      testUserID,
		"aci":                      "cache-1",
		"authorization_request_id": "other-req",
		"iat":                      float64(time.Now().Unix()),
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
		"sub":                      testUserID,
		"aci":                      "cache-1",
		"authorization_request_id": "auth-req-1",
		"iat":                      float64(iat),
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
		&tidcommon.ServiceError{Code: "JWT-1"})

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
		"sub":                      testUserID,
		"aci":                      "cache-1",
		"authorization_request_id": "auth-req-1",
		"iat":                      float64(time.Now().Unix()),
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

const testIssuer = "https://thunder.example.com"
const testEntityID = "entity-abc-123"

func (suite *CIBAServiceTestSuite) withIssuer() {
	cfg := testhelpers.OAuthConfig()
	cfg.JWT.Issuer = testIssuer
	actorProv := actorprovider.Initialize(suite.mockInboundClient, suite.mockEntityProvider, noopAuthnMgr())
	suite.service = newCIBAService(suite.mockStore, suite.mockFlowExec,
		suite.mockJWTService, actorProv, suite.mockResourceSvc, cfg)
}

func (suite *CIBAServiceTestSuite) validIDTokenHint() string {
	return buildTestAssertion(map[string]interface{}{
		"iss": testIssuer,
		"aud": "client-1",
		"sub": testEntityID,
		"exp": float64(time.Now().Add(10 * time.Minute).Unix()),
	})
}

func (suite *CIBAServiceTestSuite) TestInitiate_WithIDTokenHint_Success() {
	suite.withIssuer()
	hint := suite.validIDTokenHint()
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, hint).Return(nil)
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.MatchedBy(
		func(initCtx *flowexec.FlowInitContext) bool {
			return initCtx.InitialInputs[oauth2const.RequestParamLoginHint] == testEntityID
		})).Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
	suite.expectStoreAddSuccess()

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		IDTokenHint: hint,
		Scope:       "openid",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

func (suite *CIBAServiceTestSuite) TestInitiate_WithIDTokenHint_InvalidJWT() {
	suite.withIssuer()

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		IDTokenHint: "not.a.valid.jwt.at.all",
		Scope:       "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_WithIDTokenHint_WrongIssuer() {
	suite.withIssuer()
	hint := buildTestAssertion(map[string]interface{}{
		"iss": "https://foreign.example.com",
		"aud": "client-1",
		"sub": testEntityID,
		"exp": float64(time.Now().Add(10 * time.Minute).Unix()),
	})

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		IDTokenHint: hint,
		Scope:       "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_WithIDTokenHint_AudNotChecked() {
	suite.withIssuer()
	hint := buildTestAssertion(map[string]interface{}{
		"iss": testIssuer,
		"aud": "some-other-client",
		"sub": testEntityID,
		"exp": float64(time.Now().Add(10 * time.Minute).Unix()),
	})
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, hint).Return(nil)
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.Anything).
		Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
	suite.expectStoreAddSuccess()

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		IDTokenHint: hint,
		Scope:       "openid",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

func (suite *CIBAServiceTestSuite) TestInitiate_WithIDTokenHint_SubMissing() {
	suite.withIssuer()
	hint := buildTestAssertion(map[string]interface{}{
		"iss": testIssuer,
		"aud": "client-1",
		"exp": float64(time.Now().Add(10 * time.Minute).Unix()),
	})

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		IDTokenHint: hint,
		Scope:       "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_WithIDTokenHint_ExpMissing() {
	suite.withIssuer()
	hint := buildTestAssertion(map[string]interface{}{
		"iss": testIssuer,
		"aud": "client-1",
		"sub": testEntityID,
	})
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, hint).Return(nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		IDTokenHint: hint,
		Scope:       "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_WithIDTokenHint_SignatureFailed() {
	suite.withIssuer()
	hint := suite.validIDTokenHint()
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, hint).
		Return(&tidcommon.ServiceError{Code: "JWT-1004"})

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		IDTokenHint: hint,
		Scope:       "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_WithIDTokenHint_KeyNotFound() {
	suite.withIssuer()
	hint := suite.validIDTokenHint()
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, hint).
		Return(&tidcommon.ServiceError{Code: "JWT-1006"})

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		IDTokenHint: hint,
		Scope:       "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_WithIDTokenHint_ExpiredBeyondThreshold() {
	suite.withIssuer()
	pastExp := time.Now().Unix() - cibaIDTokenHintDefaultMaxAgeDays*24*60*60 - 1
	hint := buildTestAssertion(map[string]interface{}{
		"iss": testIssuer,
		"aud": "client-1",
		"sub": testEntityID,
		"exp": float64(pastExp),
	})
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, hint).Return(nil)

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		IDTokenHint: hint,
		Scope:       "openid",
	}, suite.oauthApp)

	suite.Nil(resp)
	suite.NotNil(cibaErr)
	suite.Equal(oauth2const.ErrorInvalidRequest, cibaErr.Code)
}

func (suite *CIBAServiceTestSuite) TestInitiate_WithIDTokenHint_ExpiredWithinThreshold() {
	suite.withIssuer()
	recentPastExp := time.Now().Unix() - (cibaIDTokenHintDefaultMaxAgeDays * 24 * 60 * 60 / 2)
	hint := buildTestAssertion(map[string]interface{}{
		"iss": testIssuer,
		"aud": "client-1",
		"sub": testEntityID,
		"exp": float64(recentPastExp),
	})
	suite.mockJWTService.EXPECT().VerifyJWTSignature(mock.Anything, hint).Return(nil)
	suite.mockFlowExec.EXPECT().InitiateAndExecute(mock.Anything, mock.Anything).
		Return(&flowexec.FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete}, nil)
	suite.expectStoreAddSuccess()

	resp, cibaErr := suite.service.InitiateBackchannelAuth(context.Background(), &BackchannelAuthRequest{
		IDTokenHint: hint,
		Scope:       "openid",
	}, suite.oauthApp)

	suite.Nil(cibaErr)
	suite.NotNil(resp)
}

// noopAuthnMgr returns an authentication-provider mock with no expectations, for tests that
// build a real actor provider but never exercise actor authentication.
func noopAuthnMgr() *managermock.AuthnProviderManagerMock {
	return &managermock.AuthnProviderManagerMock{}
}
