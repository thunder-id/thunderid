// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/deviceauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/deviceauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

const testDeviceGrantReadScope = "booking:read"

type DeviceCodeGrantHandlerTestSuite struct {
	suite.Suite
	handler         GrantHandlerInterface
	deviceService   *deviceauthmock.DeviceAuthServiceInterfaceMock
	tokenBuilder    *tokenservicemock.TokenBuilderInterfaceMock
	attributeCache  *attributecachemock.AttributeCacheServiceInterfaceMock
	resourceService *resourcemock.ResourceServiceInterfaceMock
	oauthApp        *providers.OAuthClient
	tokenRequest    *model.TokenRequest
}

func TestDeviceCodeGrantHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(DeviceCodeGrantHandlerTestSuite))
}

func (suite *DeviceCodeGrantHandlerTestSuite) SetupTest() {
	suite.deviceService = deviceauthmock.NewDeviceAuthServiceInterfaceMock(suite.T())
	suite.tokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	suite.attributeCache = attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
	suite.resourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.handler = newDeviceCodeGrantHandler(
		suite.deviceService, suite.tokenBuilder, suite.attributeCache, suite.resourceService, nil)
	suite.oauthApp = &providers.OAuthClient{
		ClientID: testGrantClientID,
		ScopeClaims: map[string][]string{
			constants.ScopeOpenID: {"sub"},
		},
	}
	suite.tokenRequest = &model.TokenRequest{
		GrantType:  string(providers.GrantTypeDeviceCode),
		ClientID:   testGrantClientID,
		DeviceCode: "device-code-1",
	}
}

func (suite *DeviceCodeGrantHandlerTestSuite) request(
	state deviceauth.DeviceRequestState,
) *deviceauth.DeviceAuthRequest {
	return &deviceauth.DeviceAuthRequest{
		RequestID:        "device-request-1",
		ClientID:         testGrantClientID,
		UserID:           testGrantSubject,
		AuthorizedScopes: constants.ScopeOpenID,
		State:            state,
		PollInterval:     constants.DeviceAuthorizationDefaultIntervalSeconds,
		ExpiryTime:       time.Now().Add(2 * time.Minute),
	}
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestValidateGrant_Success() {
	suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").
		Return(suite.request(deviceauth.DeviceStatePending), nil)

	errResp := suite.handler.ValidateGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Nil(errResp)
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestEnforceDevicePollingResourceAcceptsAuthorizedResource() {
	suite.Nil(enforceDevicePollingResource(nil, nil))
	suite.Nil(enforceDevicePollingResource(
		[]string{"https://api.example.com"}, []string{"https://api.example.com"}))
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrant_PendingStates() {
	for _, state := range []deviceauth.DeviceRequestState{
		deviceauth.DeviceStatePending,
		deviceauth.DeviceStateRedeeming,
	} {
		suite.Run(string(state), func() {
			suite.SetupTest()
			record := suite.request(state)
			suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").Return(record, nil)
			suite.deviceService.EXPECT().UpdatePolling(
				mock.Anything, "device-request-1", mock.AnythingOfType("time.Time"), int64(5)).Return(nil)

			response, errResp := suite.handler.HandleGrant(
				context.Background(), suite.tokenRequest, suite.oauthApp)
			suite.Nil(response)
			suite.Equal(constants.ErrorAuthorizationPending, errResp.Error)
		})
	}
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrant_SlowDownIncreasesInterval() {
	record := suite.request(deviceauth.DeviceStatePending)
	record.LastPolledAt = time.Now().Add(-time.Second)
	suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").Return(record, nil)
	suite.deviceService.EXPECT().UpdatePolling(
		mock.Anything, "device-request-1", mock.AnythingOfType("time.Time"), int64(10)).Return(nil)

	response, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Nil(response)
	suite.Equal(constants.ErrorSlowDown, errResp.Error)
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrant_FailedRequestReturnsAccessDenied() {
	record := suite.request(deviceauth.DeviceStateFailed)
	suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").Return(record, nil)

	response, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Nil(response)
	suite.Equal(constants.ErrorAccessDenied, errResp.Error)
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrant_Expired() {
	record := suite.request(deviceauth.DeviceStatePending)
	record.ExpiryTime = time.Now().Add(-time.Minute)
	suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").Return(record, nil)
	suite.deviceService.EXPECT().UpdateState(
		mock.Anything, "device-request-1", deviceauth.DeviceStateExpired).Return(nil)

	response, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Nil(response)
	suite.Equal(constants.ErrorExpiredToken, errResp.Error)
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrant_ApprovedIssuesTokensOnce() {
	record := suite.request(deviceauth.DeviceStateApproved)
	record.AuthTime = time.Now()
	record.CompletedACR = testGrantCompletedACR
	suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").Return(record, nil)
	suite.tokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == testGrantSubject && ctx.ClientID == testGrantClientID &&
				ctx.GrantType == string(providers.GrantTypeDeviceCode)
		})).Return(&model.TokenDTO{Token: "access-token", TokenType: "Bearer", ExpiresIn: 3600}, nil)
	suite.tokenBuilder.EXPECT().BuildIDToken(mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.IDTokenBuildContext) bool {
			return ctx.Subject == testGrantSubject && ctx.CompletedACR == testGrantCompletedACR
		})).Return(&model.TokenDTO{Token: "id-token"}, nil)
	suite.deviceService.EXPECT().MarkConsumed(mock.Anything, "device-request-1").Return(true, nil)

	response, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Nil(errResp)
	suite.Equal("access-token", response.AccessToken.Token)
	suite.Equal("id-token", response.IDToken.Token)
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestValidateGrantRejectsInvalidRequests() {
	suite.tokenRequest.GrantType = string(providers.GrantTypeAuthorizationCode)
	errResp := suite.handler.ValidateGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Equal(constants.ErrorUnsupportedGrantType, errResp.Error)

	suite.SetupTest()
	suite.tokenRequest.DeviceCode = ""
	errResp = suite.handler.ValidateGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Equal(constants.ErrorInvalidRequest, errResp.Error)
	assertNoMockCalls := suite.deviceService.AssertNotCalled(suite.T(), "GetByDeviceCode",
		mock.Anything, mock.Anything)
	suite.True(assertNoMockCalls)
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestValidateGrantMapsLookupAndBindingErrors() {
	tests := []struct {
		name          string
		record        *deviceauth.DeviceAuthRequest
		lookupErr     error
		resources     []string
		expectedError string
	}{
		{name: "unknown device code", lookupErr: deviceauth.ErrDeviceAuthRequestNotFound,
			expectedError: constants.ErrorInvalidGrant},
		{name: "store failure", lookupErr: errors.New("store unavailable"),
			expectedError: constants.ErrorServerError},
		{name: "different client", record: &deviceauth.DeviceAuthRequest{ClientID: "another-client"},
			expectedError: constants.ErrorInvalidGrant},
		{name: "invalid resource URI", record: &deviceauth.DeviceAuthRequest{ClientID: testGrantClientID},
			resources: []string{"not-an-absolute-uri"}, expectedError: constants.ErrorInvalidTarget},
		{name: "resource differs from authorization", record: &deviceauth.DeviceAuthRequest{
			ClientID: testGrantClientID, Resources: []string{"https://api.example.com/one"}},
			resources: []string{"https://api.example.com/two"}, expectedError: constants.ErrorInvalidTarget},
		{name: "multiple polling resources", record: &deviceauth.DeviceAuthRequest{
			ClientID: testGrantClientID, Resources: []string{"https://api.example.com/one"}},
			resources:     []string{"https://api.example.com/one", "https://api.example.com/two"},
			expectedError: constants.ErrorInvalidTarget},
		{name: "resource not authorized", record: &deviceauth.DeviceAuthRequest{ClientID: testGrantClientID},
			resources: []string{"https://api.example.com/one"}, expectedError: constants.ErrorInvalidTarget},
	}

	for _, test := range tests {
		suite.Run(test.name, func() {
			suite.SetupTest()
			suite.tokenRequest.Resources = test.resources
			suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").
				Return(test.record, test.lookupErr)

			errResp := suite.handler.ValidateGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
			suite.Equal(test.expectedError, errResp.Error)
		})
	}
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrantMapsLookupAndTerminalStates() {
	tests := []struct {
		name          string
		record        *deviceauth.DeviceAuthRequest
		lookupErr     error
		expectedError string
	}{
		{name: "unknown device code", lookupErr: deviceauth.ErrDeviceAuthRequestNotFound,
			expectedError: constants.ErrorInvalidGrant},
		{name: "store failure", lookupErr: errors.New("store unavailable"),
			expectedError: constants.ErrorServerError},
		{name: "denied", record: suite.request(deviceauth.DeviceStateDenied),
			expectedError: constants.ErrorAccessDenied},
		{name: "consumed", record: suite.request(deviceauth.DeviceStateConsumed),
			expectedError: constants.ErrorInvalidGrant},
		{name: "unknown state", record: suite.request(deviceauth.DeviceRequestState("unknown")),
			expectedError: constants.ErrorInvalidGrant},
	}

	for _, test := range tests {
		suite.Run(test.name, func() {
			suite.SetupTest()
			suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").
				Return(test.record, test.lookupErr)

			response, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
			suite.Nil(response)
			suite.Equal(test.expectedError, errResp.Error)
		})
	}
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrantExpiredStateUpdateFailureStillExpires() {
	record := suite.request(deviceauth.DeviceStatePending)
	record.ExpiryTime = time.Now().Add(-time.Minute)
	suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").Return(record, nil)
	suite.deviceService.EXPECT().UpdateState(
		mock.Anything, "device-request-1", deviceauth.DeviceStateExpired).
		Return(errors.New("store unavailable"))

	response, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Nil(response)
	suite.Equal(constants.ErrorExpiredToken, errResp.Error)
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrantPendingPollingUpdateFailureReturnsServerError() {
	record := suite.request(deviceauth.DeviceStatePending)
	record.PollInterval = 0
	suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").Return(record, nil)
	suite.deviceService.EXPECT().UpdatePolling(
		mock.Anything, "device-request-1", mock.AnythingOfType("time.Time"), int64(5)).
		Return(errors.New("store unavailable"))

	response, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Nil(response)
	suite.Equal(constants.ErrorServerError, errResp.Error)
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrantApprovalFailurePaths() {
	tests := []struct {
		name          string
		prepare       func(*deviceauth.DeviceAuthRequest)
		setupMocks    func()
		expectedError string
	}{
		{
			name: "attribute cache failure",
			prepare: func(record *deviceauth.DeviceAuthRequest) {
				record.AttributeCacheID = testGrantAttributeCacheID
			},
			setupMocks: func() {
				suite.attributeCache.EXPECT().GetAttributeCache(mock.Anything, testGrantAttributeCacheID).
					Return(nil, &tidcommon.InternalServerError)
			},
			expectedError: constants.ErrorServerError,
		},
		{
			name: "access token failure",
			setupMocks: func() {
				suite.tokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.Anything).
					Return(nil, errors.New("signer unavailable"))
			},
			expectedError: constants.ErrorServerError,
		},
		{
			name: "ID token failure",
			setupMocks: func() {
				suite.tokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.Anything).
					Return(&model.TokenDTO{Token: "access-token"}, nil)
				suite.tokenBuilder.EXPECT().BuildIDToken(mock.Anything, mock.Anything).
					Return(nil, errors.New("signer unavailable"))
			},
			expectedError: constants.ErrorServerError,
		},
		{
			name: "request consumption failure",
			setupMocks: func() {
				suite.tokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.Anything).
					Return(&model.TokenDTO{Token: "access-token"}, nil)
				suite.tokenBuilder.EXPECT().BuildIDToken(mock.Anything, mock.Anything).
					Return(&model.TokenDTO{Token: "id-token"}, nil)
				suite.deviceService.EXPECT().MarkConsumed(mock.Anything, "device-request-1").
					Return(false, nil)
			},
			expectedError: constants.ErrorInvalidGrant,
		},
		{
			name: "request consumption store failure",
			setupMocks: func() {
				suite.tokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.Anything).
					Return(&model.TokenDTO{Token: "access-token"}, nil)
				suite.tokenBuilder.EXPECT().BuildIDToken(mock.Anything, mock.Anything).
					Return(&model.TokenDTO{Token: "id-token"}, nil)
				suite.deviceService.EXPECT().MarkConsumed(mock.Anything, "device-request-1").
					Return(false, errors.New("store unavailable"))
			},
			expectedError: constants.ErrorServerError,
		},
		{
			name: "permission scopes require a resource",
			prepare: func(record *deviceauth.DeviceAuthRequest) {
				record.AuthorizedScopes = testDeviceGrantReadScope
			},
			setupMocks:    func() {},
			expectedError: constants.ErrorInvalidGrant,
		},
		{
			name: "resource lookup failure",
			prepare: func(record *deviceauth.DeviceAuthRequest) {
				record.Resources = []string{"https://api.example.com"}
			},
			setupMocks: func() {
				suite.resourceService.EXPECT().GetResourceServerByIdentifier(
					mock.Anything, "https://api.example.com").
					Return(nil, &tidcommon.InternalServerError)
			},
			expectedError: constants.ErrorServerError,
		},
		{
			name: "permission validation failure",
			prepare: func(record *deviceauth.DeviceAuthRequest) {
				record.AuthorizedScopes = testDeviceGrantReadScope
				record.Resources = []string{"https://api.example.com"}
			},
			setupMocks: func() {
				suite.resourceService.EXPECT().GetResourceServerByIdentifier(
					mock.Anything, "https://api.example.com").
					Return(&providers.ResourceServer{ID: "resource-1", Identifier: "https://api.example.com"}, nil)
				suite.resourceService.EXPECT().ValidatePermissions(
					mock.Anything, "resource-1", []string{testDeviceGrantReadScope}).
					Return(nil, &tidcommon.InternalServerError)
			},
			expectedError: constants.ErrorServerError,
		},
	}

	for _, test := range tests {
		suite.Run(test.name, func() {
			suite.SetupTest()
			record := suite.request(deviceauth.DeviceStateApproved)
			record.AuthTime = time.Now()
			if test.prepare != nil {
				test.prepare(record)
			}
			suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").Return(record, nil)
			test.setupMocks()

			response, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
			suite.Nil(response)
			suite.Equal(test.expectedError, errResp.Error)
		})
	}
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrantApprovedResourceIssuesBoundTokens() {
	record := suite.request(deviceauth.DeviceStateApproved)
	record.AuthTime = time.Now()
	record.AttributeCacheID = testGrantAttributeCacheID
	record.AuthorizedScopes = "openid booking:read"
	record.Resources = []string{"https://api.example.com"}
	suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").Return(record, nil)
	suite.attributeCache.EXPECT().GetAttributeCache(mock.Anything, testGrantAttributeCacheID).Return(
		&attributecache.AttributeCache{Attributes: map[string]interface{}{"email": "user@example.com"}}, nil)
	suite.resourceService.EXPECT().GetResourceServerByIdentifier(
		mock.Anything, "https://api.example.com").
		Return(&providers.ResourceServer{ID: "resource-1", Identifier: "https://api.example.com"}, nil)
	suite.resourceService.EXPECT().ValidatePermissions(
		mock.Anything, "resource-1", []string{testDeviceGrantReadScope}).Return(nil, nil)
	suite.tokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == "https://api.example.com" &&
				len(ctx.Scopes) == 2 && ctx.Scopes[0] == constants.ScopeOpenID &&
				ctx.Scopes[1] == testDeviceGrantReadScope
		})).Return(&model.TokenDTO{Token: "access-token"}, nil)
	suite.tokenBuilder.EXPECT().BuildIDToken(mock.Anything, mock.Anything).
		Return(&model.TokenDTO{Token: "id-token"}, nil)
	suite.deviceService.EXPECT().MarkConsumed(mock.Anything, "device-request-1").Return(true, nil)

	response, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Nil(errResp)
	suite.Equal("access-token", response.AccessToken.Token)
	suite.Equal("id-token", response.IDToken.Token)
}

func (suite *DeviceCodeGrantHandlerTestSuite) TestHandleGrantApprovedWithoutOpenIDIssuesAccessTokenOnly() {
	record := suite.request(deviceauth.DeviceStateApproved)
	record.AuthorizedScopes = "profile"
	suite.deviceService.EXPECT().GetByDeviceCode(mock.Anything, "device-code-1").Return(record, nil)
	suite.tokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return len(ctx.Audiences) == 1 && ctx.Audiences[0] == testGrantClientID &&
				len(ctx.Scopes) == 1 && ctx.Scopes[0] == "profile"
		})).Return(&model.TokenDTO{Token: "access-token"}, nil)
	suite.deviceService.EXPECT().MarkConsumed(mock.Anything, "device-request-1").Return(true, nil)

	response, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenRequest, suite.oauthApp)
	suite.Nil(errResp)
	suite.Equal("access-token", response.AccessToken.Token)
	suite.Empty(response.IDToken.Token)
}
