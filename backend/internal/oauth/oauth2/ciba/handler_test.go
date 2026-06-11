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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
)

type CIBAHandlerTestSuite struct {
	suite.Suite
	mockService *CIBAServiceInterfaceMock
	handler     CIBAHandlerInterface
}

func TestCIBAHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(CIBAHandlerTestSuite))
}

func (suite *CIBAHandlerTestSuite) SetupTest() {
	suite.mockService = NewCIBAServiceInterfaceMock(suite.T())
	suite.handler = newCIBAHandler(suite.mockService)
}

func (suite *CIBAHandlerTestSuite) newAuthRequest(body string, client *clientauth.OAuthClientInfo) *http.Request {
	req := httptest.NewRequest(http.MethodPost, oauth2const.OAuth2BackchannelAuthEndpoint,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if client != nil {
		req = req.WithContext(context.WithValue(req.Context(), clientauth.OAuthClientKey, client))
	}
	return req
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_Success() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &inboundmodel.OAuthClient{ClientID: "client-1"},
	}
	suite.mockService.EXPECT().InitiateBackchannelAuth(mock.Anything, mock.MatchedBy(
		func(r *BackchannelAuthRequest) bool {
			return r.LoginHint == "alice" && r.Scope == "openid"
		}), client.OAuthApp).Return(&BackchannelAuthResponse{
		AuthReqID: "auth-req-1",
		ExpiresIn: 120,
		Interval:  5,
	}, nil)

	req := suite.newAuthRequest("login_hint=alice&scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
	var resp BackchannelAuthResponse
	suite.NoError(json.NewDecoder(w.Body).Decode(&resp))
	suite.Equal("auth-req-1", resp.AuthReqID)
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_NoClientInContext() {
	req := suite.newAuthRequest("login_hint=alice&scope=openid", nil)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_ServiceError() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &inboundmodel.OAuthClient{ClientID: "client-1"},
	}
	suite.mockService.EXPECT().InitiateBackchannelAuth(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &CIBAError{Code: oauth2const.ErrorUnknownUserID, Message: "unknown user"})

	req := suite.newAuthRequest("login_hint=ghost&scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorUnknownUserID, body["error"])
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_UnauthorizedClientMapsTo400() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &inboundmodel.OAuthClient{ClientID: "client-1"},
	}
	suite.mockService.EXPECT().InitiateBackchannelAuth(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &CIBAError{Code: oauth2const.ErrorUnauthorizedClient, Message: "not allowed"})

	req := suite.newAuthRequest("login_hint=alice&scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorUnauthorizedClient, body["error"])
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_ZeroHints() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &inboundmodel.OAuthClient{ClientID: "client-1"},
	}
	req := suite.newAuthRequest("scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_MultipleHints() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &inboundmodel.OAuthClient{ClientID: "client-1"},
	}
	req := suite.newAuthRequest("login_hint=alice&id_token_hint=eyJhbGci&scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_IDTokenHintUnsupported() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &inboundmodel.OAuthClient{ClientID: "client-1"},
	}
	req := suite.newAuthRequest("id_token_hint=eyJhbGci&scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_LoginHintTokenUnsupported() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &inboundmodel.OAuthClient{ClientID: "client-1"},
	}
	req := suite.newAuthRequest("login_hint_token=eyJhbGci&scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_ServerErrorMapsTo500() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &inboundmodel.OAuthClient{ClientID: "client-1"},
	}
	suite.mockService.EXPECT().InitiateBackchannelAuth(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &CIBAError{Code: oauth2const.ErrorServerError, Message: "internal error"})

	req := suite.newAuthRequest("login_hint=alice&scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}
