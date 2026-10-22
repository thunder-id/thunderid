// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package granthandlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/deviceauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/deviceauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

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
		suite.deviceService, suite.tokenBuilder, suite.attributeCache, suite.resourceService)
	suite.oauthApp = &providers.OAuthClient{
		ClientID: "client-1",
		ScopeClaims: map[string][]string{
			constants.ScopeOpenID: {"sub"},
		},
	}
	suite.tokenRequest = &model.TokenRequest{
		GrantType:  string(providers.GrantTypeDeviceCode),
		ClientID:   "client-1",
		DeviceCode: "device-code-1",
	}
}

func (suite *DeviceCodeGrantHandlerTestSuite) request(
	state deviceauth.DeviceRequestState,
) *deviceauth.DeviceAuthRequest {
	return &deviceauth.DeviceAuthRequest{
		RequestID:        "device-request-1",
		ClientID:         "client-1",
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
			return ctx.Subject == testGrantSubject && ctx.ClientID == "client-1" &&
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
