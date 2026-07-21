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

package granthandlers

import (
	"context"
	"errors"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/cibamock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

type CIBAGrantHandlerTestSuite struct {
	suite.Suite
	handler              GrantHandlerInterface
	mockCIBAService      *cibamock.CIBAServiceInterfaceMock
	mockTokenBuilder     *tokenservicemock.TokenBuilderInterfaceMock
	mockAttrCacheService *attributecachemock.AttributeCacheServiceInterfaceMock
	mockResource         *resourcemock.ResourceServiceInterfaceMock
	oauthApp             *providers.OAuthClient
	tokenReq             *model.TokenRequest
}

func TestCIBAGrantHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(CIBAGrantHandlerTestSuite))
}

func (suite *CIBAGrantHandlerTestSuite) SetupTest() {
	suite.mockCIBAService = cibamock.NewCIBAServiceInterfaceMock(suite.T())
	suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	suite.mockAttrCacheService = attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
	suite.mockResource = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.handler = newCIBAGrantHandler(suite.mockCIBAService, suite.mockTokenBuilder,
		suite.mockAttrCacheService, suite.mockResource)
	suite.oauthApp = &providers.OAuthClient{ClientID: "client-1"}
	suite.tokenReq = &model.TokenRequest{
		GrantType: string(providers.GrantTypeCIBA),
		ClientID:  "client-1",
		AuthReqID: "auth-req-1",
	}
}

func (suite *CIBAGrantHandlerTestSuite) pendingRecord() *ciba.CIBAAuthRequest {
	return &ciba.CIBAAuthRequest{
		AuthReqID:        "auth-req-1",
		ClientID:         "client-1",
		UserID:           "user-1",
		AuthorizedScopes: "openid profile",
		State:            ciba.CIBAStatePending,
		ExpiryTime:       time.Now().Add(2 * time.Minute),
	}
}

const testCIBAResourceURL = "https://api.example.com"

// boundAuthenticatedRecord returns an authenticated record bound to testCIBAResourceURL.
func (suite *CIBAGrantHandlerTestSuite) boundAuthenticatedRecord(scopes string) *ciba.CIBAAuthRequest {
	record := suite.pendingRecord()
	record.State = ciba.CIBAStateAuthenticated
	record.AuthorizedScopes = scopes
	record.Resources = []string{testCIBAResourceURL}
	return record
}

// expectResourceServer stubs the stored-resource resolution for a bound record, with all permissions valid.
func (suite *CIBAGrantHandlerTestSuite) expectResourceServer() {
	suite.mockResource.EXPECT().GetResourceServerByIdentifier(mock.Anything, testCIBAResourceURL).
		Return(&providers.ResourceServer{ID: "rs-1", Identifier: testCIBAResourceURL}, nil)
	suite.mockResource.EXPECT().ValidatePermissions(mock.Anything, "rs-1", mock.Anything).
		Return([]string{}, nil)
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_Success() {
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)

	errResp := suite.handler.ValidateGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(errResp)
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_WrongGrantType() {
	req := &model.TokenRequest{GrantType: string(providers.GrantTypeRefreshToken), AuthReqID: "x"}
	errResp := suite.handler.ValidateGrant(context.Background(), req, suite.oauthApp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorUnsupportedGrantType, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_MissingAuthReqID() {
	req := &model.TokenRequest{GrantType: string(providers.GrantTypeCIBA)}
	errResp := suite.handler.ValidateGrant(context.Background(), req, suite.oauthApp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidRequest, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_RequestNotFound() {
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(nil, ciba.ErrCIBARequestNotFound)

	errResp := suite.handler.ValidateGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidGrant, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_ClientMismatch() {
	record := suite.pendingRecord()
	record.ClientID = "other-client"
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)

	errResp := suite.handler.ValidateGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidGrant, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_StoreError() {
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(nil, errors.New("db error"))

	errResp := suite.handler.ValidateGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorServerError, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Pending() {
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)
	suite.mockCIBAService.EXPECT().UpdateLastPolled(mock.Anything, "auth-req-1",
		mock.AnythingOfType("time.Time")).Return(nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorAuthorizationPending, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_IntervalElapsedReturnsPending() {
	record := suite.pendingRecord()
	record.LastPolledAt = time.Now().Add(
		-time.Duration(constants.CIBADefaultIntervalSeconds+1) * time.Second)
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.mockCIBAService.EXPECT().UpdateLastPolled(mock.Anything, "auth-req-1",
		mock.AnythingOfType("time.Time")).Return(nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorAuthorizationPending, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_SlowDown() {
	record := suite.pendingRecord()
	record.LastPolledAt = time.Now().Add(-1 * time.Second)
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.mockCIBAService.EXPECT().UpdateLastPolled(mock.Anything, "auth-req-1",
		mock.AnythingOfType("time.Time")).Return(nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorSlowDown, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Expired() {
	record := suite.pendingRecord()
	record.ExpiryTime = time.Now().Add(-1 * time.Minute)
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.mockCIBAService.EXPECT().UpdateState(mock.Anything, "auth-req-1", ciba.CIBAStateExpired).Return(nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorExpiredToken, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Denied() {
	record := suite.pendingRecord()
	record.State = ciba.CIBAStateDenied
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorAccessDenied, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Consumed() {
	record := suite.pendingRecord()
	record.State = ciba.CIBAStateConsumed
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidGrant, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Authenticated_IssuesTokens() {
	record := suite.pendingRecord()
	record.State = ciba.CIBAStateAuthenticated
	record.AttributeCacheID = "cache-1"
	record.CompletedACR = "urn:acr:pwd"
	record.AuthTime = time.Now()
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.mockAttrCacheService.EXPECT().GetAttributeCache(mock.Anything, "cache-1").Return(
		&attributecache.AttributeCache{ID: "cache-1", Attributes: map[string]interface{}{"email": "a@b.c"}},
		nil)
	suite.mockTokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			return ctx.Subject == "user-1" && ctx.ClientID == "client-1" &&
				ctx.GrantType == string(providers.GrantTypeCIBA)
		})).Return(&model.TokenDTO{Token: "access-token", TokenType: "Bearer", ExpiresIn: 3600}, nil)
	suite.mockTokenBuilder.EXPECT().BuildIDToken(mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.IDTokenBuildContext) bool {
			return ctx.Subject == "user-1" && ctx.CompletedACR == "urn:acr:pwd"
		})).Return(&model.TokenDTO{Token: "id-token"}, nil)
	suite.mockCIBAService.EXPECT().MarkConsumed(mock.Anything, "auth-req-1").Return(true, nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(errResp)
	suite.NotNil(resp)
	suite.Equal("access-token", resp.AccessToken.Token)
	suite.Equal("id-token", resp.IDToken.Token)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Authenticated_NoOpenIDSkipsIDToken() {
	record := suite.boundAuthenticatedRecord(testScopeRead)
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.expectResourceServer()
	suite.mockTokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.Anything).Return(
		&model.TokenDTO{Token: "access-token", TokenType: "Bearer"}, nil)
	suite.mockCIBAService.EXPECT().MarkConsumed(mock.Anything, "auth-req-1").Return(true, nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(errResp)
	suite.NotNil(resp)
	suite.Equal("access-token", resp.AccessToken.Token)
	suite.Empty(resp.IDToken.Token)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Authenticated_OneTimeUseRace() {
	record := suite.boundAuthenticatedRecord(testScopeRead)
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.expectResourceServer()
	suite.mockTokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.Anything).Return(
		&model.TokenDTO{Token: "access-token", TokenType: "Bearer"}, nil)
	suite.mockCIBAService.EXPECT().MarkConsumed(mock.Anything, "auth-req-1").Return(false, nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidGrant, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Authenticated_AttributeCacheError() {
	record := suite.pendingRecord()
	record.State = ciba.CIBAStateAuthenticated
	record.AttributeCacheID = "cache-1"
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.mockAttrCacheService.EXPECT().GetAttributeCache(mock.Anything, "cache-1").Return(nil,
		&tidcommon.ServiceError{Code: "AC-1"})

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorServerError, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Authenticated_AccessTokenError() {
	record := suite.boundAuthenticatedRecord(testScopeRead)
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.expectResourceServer()
	suite.mockTokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.Anything).Return(nil,
		errors.New("build error"))

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorServerError, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Authenticated_IDTokenError() {
	record := suite.pendingRecord()
	record.State = ciba.CIBAStateAuthenticated
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.mockTokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.Anything).Return(
		&model.TokenDTO{Token: "access-token"}, nil)
	suite.mockTokenBuilder.EXPECT().BuildIDToken(mock.Anything, mock.Anything).Return(nil,
		errors.New("id token error"))

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorServerError, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Authenticated_MarkConsumedError() {
	record := suite.boundAuthenticatedRecord(testScopeRead)
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.expectResourceServer()
	suite.mockTokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.Anything).Return(
		&model.TokenDTO{Token: "access-token"}, nil)
	suite.mockCIBAService.EXPECT().MarkConsumed(mock.Anything, "auth-req-1").Return(false,
		errors.New("db error"))

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorServerError, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_StoreError() {
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(nil, errors.New("db error"))

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorServerError, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_RequestNotFound() {
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(nil, ciba.ErrCIBARequestNotFound)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidGrant, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_Pending_UpdateLastPolledFails_StillReturnsPending() {
	// UpdateLastPolled failure is logged but must not affect the pending/slow_down response.
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)
	suite.mockCIBAService.EXPECT().UpdateLastPolled(mock.Anything, "auth-req-1",
		mock.AnythingOfType("time.Time")).Return(errors.New("redis unavailable"))

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorAuthorizationPending, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_BoundTokenHasResourceAudience() {
	record := suite.boundAuthenticatedRecord(testScopeRead)
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.expectResourceServer()
	var capturedCtx *tokenservice.AccessTokenBuildContext
	suite.mockTokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			capturedCtx = ctx
			return true
		})).Return(&model.TokenDTO{Token: "access-token", TokenType: "Bearer"}, nil)
	suite.mockCIBAService.EXPECT().MarkConsumed(mock.Anything, "auth-req-1").Return(true, nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(errResp)
	suite.NotNil(resp)
	suite.Equal([]string{testCIBAResourceURL}, capturedCtx.Audiences)
	suite.Equal([]string{"read"}, capturedCtx.Scopes)
	suite.Equal([]string{testCIBAResourceURL}, resp.AccessToken.OriginalAudiences)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_UnboundOIDCOnlyKeepsClientAudience() {
	record := suite.pendingRecord()
	record.State = ciba.CIBAStateAuthenticated
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	var capturedCtx *tokenservice.AccessTokenBuildContext
	suite.mockTokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			capturedCtx = ctx
			return true
		})).Return(&model.TokenDTO{Token: "access-token", TokenType: "Bearer"}, nil)
	suite.mockTokenBuilder.EXPECT().BuildIDToken(mock.Anything, mock.Anything).Return(
		&model.TokenDTO{Token: "id-token"}, nil)
	suite.mockCIBAService.EXPECT().MarkConsumed(mock.Anything, "auth-req-1").Return(true, nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(errResp)
	suite.NotNil(resp)
	suite.Equal([]string{"client-1"}, capturedCtx.Audiences)
	suite.Equal([]string{"client-1"}, resp.AccessToken.OriginalAudiences)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_UnboundWithPermissionScopesRejected() {
	record := suite.pendingRecord()
	record.State = ciba.CIBAStateAuthenticated
	record.AuthorizedScopes = "openid read"
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidGrant, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_PermissionScopesOutsideRSOmitted() {
	record := suite.boundAuthenticatedRecord("read write")
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.mockResource.EXPECT().GetResourceServerByIdentifier(mock.Anything, testCIBAResourceURL).
		Return(&providers.ResourceServer{ID: "rs-1", Identifier: testCIBAResourceURL}, nil)
	suite.mockResource.EXPECT().ValidatePermissions(mock.Anything, "rs-1", mock.Anything).
		Return([]string{"write"}, nil)
	var capturedCtx *tokenservice.AccessTokenBuildContext
	suite.mockTokenBuilder.EXPECT().BuildAccessToken(mock.Anything, mock.MatchedBy(
		func(ctx *tokenservice.AccessTokenBuildContext) bool {
			capturedCtx = ctx
			return true
		})).Return(&model.TokenDTO{Token: "access-token"}, nil)
	suite.mockCIBAService.EXPECT().MarkConsumed(mock.Anything, "auth-req-1").Return(true, nil)

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(errResp)
	suite.NotNil(resp)
	suite.Equal([]string{"read"}, capturedCtx.Scopes)
}

func (suite *CIBAGrantHandlerTestSuite) TestHandleGrant_StoredResourceLookupFails() {
	record := suite.boundAuthenticatedRecord(testScopeRead)
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	suite.mockResource.EXPECT().GetResourceServerByIdentifier(mock.Anything, testCIBAResourceURL).
		Return(nil, &tidcommon.ServiceError{Code: "RS-5000", Type: tidcommon.ServerErrorType})

	resp, errResp := suite.handler.HandleGrant(context.Background(), suite.tokenReq, suite.oauthApp)
	suite.Nil(resp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorServerError, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_MatchingPollingResource() {
	record := suite.pendingRecord()
	record.Resources = []string{testCIBAResourceURL}
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	req := &model.TokenRequest{GrantType: string(providers.GrantTypeCIBA), ClientID: "client-1",
		AuthReqID: "auth-req-1", Resources: []string{testCIBAResourceURL}}

	suite.Nil(suite.handler.ValidateGrant(context.Background(), req, suite.oauthApp))
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_MismatchedPollingResource() {
	record := suite.pendingRecord()
	record.Resources = []string{testCIBAResourceURL}
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	req := &model.TokenRequest{GrantType: string(providers.GrantTypeCIBA), ClientID: "client-1",
		AuthReqID: "auth-req-1", Resources: []string{"https://other.example.com"}}

	errResp := suite.handler.ValidateGrant(context.Background(), req, suite.oauthApp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidTarget, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_PollingResourceOnUnbound() {
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(suite.pendingRecord(), nil)
	req := &model.TokenRequest{GrantType: string(providers.GrantTypeCIBA), ClientID: "client-1",
		AuthReqID: "auth-req-1", Resources: []string{testCIBAResourceURL}}

	errResp := suite.handler.ValidateGrant(context.Background(), req, suite.oauthApp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidTarget, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_DuplicatePollingResource() {
	record := suite.pendingRecord()
	record.Resources = []string{testCIBAResourceURL}
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	req := &model.TokenRequest{GrantType: string(providers.GrantTypeCIBA), ClientID: "client-1",
		AuthReqID: "auth-req-1", Resources: []string{testCIBAResourceURL, testCIBAResourceURL}}

	errResp := suite.handler.ValidateGrant(context.Background(), req, suite.oauthApp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidTarget, errResp.Error)
}

func (suite *CIBAGrantHandlerTestSuite) TestValidateGrant_InvalidPollingResourceURI() {
	record := suite.pendingRecord()
	record.Resources = []string{testCIBAResourceURL}
	suite.mockCIBAService.EXPECT().GetByAuthReqID(mock.Anything, "auth-req-1").Return(record, nil)
	req := &model.TokenRequest{GrantType: string(providers.GrantTypeCIBA), ClientID: "client-1",
		AuthReqID: "auth-req-1", Resources: []string{"/relative-path"}}

	errResp := suite.handler.ValidateGrant(context.Background(), req, suite.oauthApp)
	suite.NotNil(errResp)
	suite.Equal(constants.ErrorInvalidTarget, errResp.Error)
}
