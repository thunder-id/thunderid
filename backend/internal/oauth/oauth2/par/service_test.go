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

package par

import (
	"context"
	"errors"
	"strings"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

const (
	testJKT      = "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I"
	testOtherJKT = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
)

type ServiceTestSuite struct {
	suite.Suite
	ctx     context.Context
	testCfg oauthconfig.Config
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	testConfig := &config.Config{
		OAuth: engineconfig.OAuthConfig{
			PAR: engineconfig.PARConfig{
				ExpiresIn: 60,
			},
		},
	}
	_ = config.InitializeServerRuntime("", testConfig)
	s.ctx = context.Background()
	s.testCfg = testhelpers.OAuthConfig()
	s.testCfg.OAuth.PAR.ExpiresIn = 60
}

func (s *ServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *ServiceTestSuite) newTestApp() *providers.OAuthClient {
	return &providers.OAuthClient{
		ClientID:                "test-client",
		RedirectURIs:            []string{"https://example.com/callback"},
		GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
		ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
		Scopes:                  []string{"openid", "profile", "email"},
	}
}

// newPermissiveResourceMock returns a resource service mock that accepts any lookup, so tests
// not concerned with resource indicators don't need to script expectations.
func (s *ServiceTestSuite) newPermissiveResourceMock() *resourcemock.ResourceServiceInterfaceMock {
	m := resourcemock.NewResourceServiceInterfaceMock(s.T())
	m.On("GetResourceServerByIdentifier", mock.Anything, mock.Anything).
		Return(func(_ context.Context, identifier string) *providers.ResourceServer {
			return &providers.ResourceServer{ID: identifier, Identifier: identifier}
		}, func(_ context.Context, _ string) *tidcommon.ServiceError {
			return nil
		}).Maybe()
	m.On("ValidatePermissions", mock.Anything, mock.Anything, mock.Anything).
		Return([]string{}, (*tidcommon.ServiceError)(nil)).Maybe()
	return m
}

func (s *ServiceTestSuite) newValidParams() map[string]string {
	return map[string]string{
		oauth2const.RequestParamResponseType: "code",
		oauth2const.RequestParamRedirectURI:  "https://example.com/callback",
		oauth2const.RequestParamScope:        "openid",
		oauth2const.RequestParamState:        "test-state",
	}
}

func (s *ServiceTestSuite) TestHandlePAR_Success() {
	store := newParStoreInterfaceMock(s.T())
	store.EXPECT().Store(mock.Anything, mock.Anything, mock.Anything).Return("test-uri", nil)
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Empty(s.T(), errCode)
	assert.NotNil(s.T(), resp)
	assert.True(s.T(), strings.HasPrefix(resp.RequestURI, requestURIPrefix))
	assert.Equal(s.T(), int64(60), resp.ExpiresIn)
}

func (s *ServiceTestSuite) TestHandlePAR_RejectsRequestURIInBody() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamRequestURI] = "urn:ietf:params:oauth:request_uri:test"

	resp, errCode, errDesc := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidRequest, errCode)
	assert.Contains(s.T(), errDesc, "request_uri parameter must not be included")
}

func (s *ServiceTestSuite) TestHandlePAR_MissingResponseType() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	delete(params, oauth2const.RequestParamResponseType)

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidRequest, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_InvalidRedirectURI() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamRedirectURI] = "https://evil.com/callback"

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidRequest, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_UnauthorizedGrantType() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	app.GrantTypes = []providers.GrantType{providers.GrantTypeClientCredentials}
	params := s.newValidParams()

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorUnauthorizedClient, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_UnsupportedResponseType() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamResponseType] = "token"

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorUnsupportedResponseType, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_PKCERequired() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	app.PKCERequired = true
	params := s.newValidParams()
	// No code_challenge provided.

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidRequest, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_StoreError() {
	store := newParStoreInterfaceMock(s.T())
	store.EXPECT().Store(mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("store error"))
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorServerError, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_PromptNone_LoginRequired() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamPrompt] = "none"

	resp, errCode, errDesc := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorLoginRequired, errCode)
	assert.Equal(s.T(), "User authentication is required", errDesc)
}

func (s *ServiceTestSuite) TestHandlePAR_PromptInvalid() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamPrompt] = "invalid_value"

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidRequest, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_PromptLogin_Success() {
	store := newParStoreInterfaceMock(s.T())
	store.EXPECT().Store(mock.Anything, mock.Anything, mock.Anything).Return("test-uri", nil)
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamPrompt] = "login"

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Empty(s.T(), errCode)
	assert.NotNil(s.T(), resp)
}

func (s *ServiceTestSuite) TestHandlePAR_ResourceWithFragment() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	resources := []string{"https://api.example.com/resource#fragment"}

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, resources, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidTarget, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_ResourceMissingScheme() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	resources := []string{"api.example.com/resource"}

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, resources, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidTarget, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_ValidResource_Success() {
	store := newParStoreInterfaceMock(s.T())
	store.EXPECT().Store(mock.Anything, mock.Anything, mock.Anything).Return("test-uri", nil)
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	resources := []string{"https://api.example.com/resource"}

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, resources, app, "")

	assert.Empty(s.T(), errCode)
	assert.NotNil(s.T(), resp)
}

func (s *ServiceTestSuite) TestHandlePAR_UnregisteredResource_InvalidTarget() {
	store := newParStoreInterfaceMock(s.T())
	rsMock := resourcemock.NewResourceServiceInterfaceMock(s.T())
	rsMock.On("GetResourceServerByIdentifier", mock.Anything, "https://unknown.example.com").
		Return((*providers.ResourceServer)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			Code: "RES-1001",
		})
	svc := newPARService(store, rsMock, s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	resources := []string{"https://unknown.example.com"}

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, resources, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidTarget, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_ResourceResolutionServerError() {
	store := newParStoreInterfaceMock(s.T())
	rsMock := resourcemock.NewResourceServiceInterfaceMock(s.T())
	rsMock.On("GetResourceServerByIdentifier", mock.Anything, mock.Anything).
		Return((*providers.ResourceServer)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "RES-5000",
		})
	svc := newPARService(store, rsMock, s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	resources := []string{"https://api.example.com/resource"}

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, resources, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorServerError, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_ScopesDownscopedAgainstResourceServers() {
	store := newParStoreInterfaceMock(s.T())
	var captured pushedAuthorizationRequest
	store.EXPECT().Store(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, req pushedAuthorizationRequest, _ int64) {
			captured = req
		}).Return("test-uri", nil)

	rsMock := resourcemock.NewResourceServiceInterfaceMock(s.T())
	rsMock.On("GetResourceServerByIdentifier", mock.Anything, "https://api.example.com").
		Return(&providers.ResourceServer{ID: "rs-1", Identifier: "https://api.example.com"},
			(*tidcommon.ServiceError)(nil))
	// "write" is not a permission on rs-1, so the helper should drop it.
	rsMock.On("ValidatePermissions", mock.Anything, "rs-1",
		mock.MatchedBy(func(scopes []string) bool {
			return len(scopes) == 2 && scopes[0] == "read" && scopes[1] == "write"
		})).
		Return([]string{"write"}, (*tidcommon.ServiceError)(nil))

	svc := newPARService(store, rsMock, s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamScope] = "read write"
	resources := []string{"https://api.example.com"}

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, resources, app, "")

	assert.Empty(s.T(), errCode)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), []string{"read"}, captured.OAuthParameters.PermissionScopes)
}

func (s *ServiceTestSuite) TestHandlePAR_FiltersOIDCScopesByAppScopes() {
	store := newParStoreInterfaceMock(s.T())
	var captured pushedAuthorizationRequest
	store.EXPECT().Store(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, req pushedAuthorizationRequest, _ int64) {
			captured = req
		}).Return("test-uri", nil)

	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	app.Scopes = []string{"profile"}
	params := s.newValidParams()
	params[oauth2const.RequestParamScope] = "openid email profile"

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Empty(s.T(), errCode)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), []string{"profile"}, captured.OAuthParameters.StandardScopes)
}

func (s *ServiceTestSuite) TestHandlePAR_AcrValuesPropagated() {
	store := newParStoreInterfaceMock(s.T())
	var captured pushedAuthorizationRequest
	store.EXPECT().Store(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, req pushedAuthorizationRequest, _ int64) {
			captured = req
		}).Return("test-uri", nil)

	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamAcrValues] = "urn:thunder:acr:password urn:thunder:acr:generated-code"

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Empty(s.T(), errCode)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(),
		"urn:thunder:acr:password urn:thunder:acr:generated-code",
		captured.OAuthParameters.AcrValues)
}

func (s *ServiceTestSuite) TestHandlePAR_DPoPHeaderJkt_PersistedOnRequest() {
	var captured pushedAuthorizationRequest
	store := newParStoreInterfaceMock(s.T())
	store.EXPECT().Store(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, req pushedAuthorizationRequest, _ int64) {
			captured = req
		}).Return("test-uri", nil)
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, testJKT)

	assert.Empty(s.T(), errCode)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), testJKT, captured.OAuthParameters.DPoPJkt)
}

func (s *ServiceTestSuite) TestHandlePAR_DPoPJktParam_PersistedWhenNoHeader() {
	var captured pushedAuthorizationRequest
	store := newParStoreInterfaceMock(s.T())
	store.EXPECT().Store(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, req pushedAuthorizationRequest, _ int64) {
			captured = req
		}).Return("test-uri", nil)
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamDPoPJkt] = testJKT

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Empty(s.T(), errCode)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), testJKT, captured.OAuthParameters.DPoPJkt)
}

func (s *ServiceTestSuite) TestHandlePAR_DPoPJktParam_HeaderMismatch_Rejected() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamDPoPJkt] = testJKT

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, testOtherJKT)

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidDPoPProof, errCode)
}

func (s *ServiceTestSuite) TestHandlePAR_NonceTooLong() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	params[oauth2const.RequestParamNonce] = strings.Repeat("a", oauth2const.MaxNonceLength+1)

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, nil, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidRequest, errCode)
}

func (s *ServiceTestSuite) TestResolvePAR_Success() {
	storedRequest := pushedAuthorizationRequest{
		ClientID: "test-client",
		OAuthParameters: model.OAuthParameters{
			ClientID:     "test-client",
			RedirectURI:  "https://example.com/callback",
			ResponseType: "code",
		},
	}
	store := newParStoreInterfaceMock(s.T())
	store.EXPECT().Consume(mock.Anything, mock.Anything).Return(storedRequest, true, nil)
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)

	result, err := svc.ResolvePushedAuthorizationRequest(
		s.ctx, requestURIPrefix+"test-uri", "test-client")

	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), result)
	assert.Equal(s.T(), "test-client", result.ClientID)
	assert.Equal(s.T(), "https://example.com/callback", result.RedirectURI)
}

func (s *ServiceTestSuite) TestResolvePAR_InvalidURIFormat() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)

	result, err := svc.ResolvePushedAuthorizationRequest(s.ctx, "invalid-uri", "test-client")

	assert.Nil(s.T(), result)
	assert.ErrorIs(s.T(), err, errInvalidRequestURI)
}

func (s *ServiceTestSuite) TestResolvePAR_NotFound() {
	store := newParStoreInterfaceMock(s.T())
	store.EXPECT().Consume(mock.Anything, mock.Anything).Return(pushedAuthorizationRequest{}, false, nil)
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)

	result, err := svc.ResolvePushedAuthorizationRequest(
		s.ctx, requestURIPrefix+"nonexistent", "test-client")

	assert.Nil(s.T(), result)
	assert.ErrorIs(s.T(), err, errRequestURINotFound)
}

func (s *ServiceTestSuite) TestResolvePAR_ClientIDMismatch() {
	storedRequest := pushedAuthorizationRequest{
		ClientID: "client-a",
		OAuthParameters: model.OAuthParameters{
			ClientID: "client-a",
		},
	}
	store := newParStoreInterfaceMock(s.T())
	store.EXPECT().Consume(mock.Anything, mock.Anything).Return(storedRequest, true, nil)
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)

	result, err := svc.ResolvePushedAuthorizationRequest(
		s.ctx, requestURIPrefix+"test-uri", "client-b")

	assert.Nil(s.T(), result)
	assert.ErrorIs(s.T(), err, errClientIDMismatch)
}

func (s *ServiceTestSuite) TestResolvePAR_ConsumeError() {
	store := newParStoreInterfaceMock(s.T())
	store.EXPECT().Consume(mock.Anything, mock.Anything).
		Return(pushedAuthorizationRequest{}, false, errors.New("cache error"))
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)

	result, err := svc.ResolvePushedAuthorizationRequest(
		s.ctx, requestURIPrefix+"test-uri", "test-client")

	assert.Nil(s.T(), result)
	assert.ErrorIs(s.T(), err, ErrPARResolutionFailed)
}

func (s *ServiceTestSuite) TestHandlePAR_MultipleResources_InvalidTarget() {
	store := newParStoreInterfaceMock(s.T())
	svc := newPARService(store, s.newPermissiveResourceMock(), s.testCfg)
	app := s.newTestApp()
	params := s.newValidParams()
	resources := []string{"https://a.example.com", "https://b.example.com"}

	resp, errCode, _ := svc.HandlePushedAuthorizationRequest(s.ctx, params, resources, app, "")

	assert.Nil(s.T(), resp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidTarget, errCode)
}
