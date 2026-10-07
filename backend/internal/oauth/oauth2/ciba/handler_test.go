// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/discoverymock"
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
		OAuthApp: &providers.OAuthClient{ClientID: "client-1"},
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
		OAuthApp: &providers.OAuthClient{ClientID: "client-1"},
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
		OAuthApp: &providers.OAuthClient{ClientID: "client-1"},
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
		OAuthApp: &providers.OAuthClient{ClientID: "client-1"},
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
		OAuthApp: &providers.OAuthClient{ClientID: "client-1"},
	}
	req := suite.newAuthRequest("login_hint=alice&id_token_hint=eyJhbGci&scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusBadRequest, w.Code)
	var body map[string]string
	suite.NoError(json.NewDecoder(w.Body).Decode(&body))
	suite.Equal(oauth2const.ErrorInvalidRequest, body["error"])
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_IDTokenHintRoutedToService() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &providers.OAuthClient{ClientID: "client-1"},
	}
	suite.mockService.EXPECT().InitiateBackchannelAuth(mock.Anything, mock.MatchedBy(
		func(r *BackchannelAuthRequest) bool {
			return r.IDTokenHint == "eyJhbGci" && r.LoginHint == "" && r.Scope == "openid"
		}), client.OAuthApp).Return(&BackchannelAuthResponse{
		AuthReqID: "auth-req-1",
		ExpiresIn: 120,
		Interval:  5,
	}, nil)

	req := suite.newAuthRequest("id_token_hint=eyJhbGci&scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_LoginHintTokenUnsupported() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &providers.OAuthClient{ClientID: "client-1"},
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
		OAuthApp: &providers.OAuthClient{ClientID: "client-1"},
	}
	suite.mockService.EXPECT().InitiateBackchannelAuth(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &CIBAError{Code: oauth2const.ErrorServerError, Message: "internal error"})

	req := suite.newAuthRequest("login_hint=alice&scope=openid", client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusInternalServerError, w.Code)
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_SingleResourceParsed() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &providers.OAuthClient{ClientID: "client-1"},
	}
	suite.mockService.EXPECT().InitiateBackchannelAuth(mock.Anything, mock.MatchedBy(
		func(r *BackchannelAuthRequest) bool {
			return len(r.Resources) == 1 && r.Resources[0] == "https://api.example.com"
		}), client.OAuthApp).Return(&BackchannelAuthResponse{
		AuthReqID: "auth-req-1",
		ExpiresIn: 120,
		Interval:  5,
	}, nil)

	form := url.Values{}
	form.Set("login_hint", "alice")
	form.Set("scope", "openid read:things")
	form.Set("resource", "https://api.example.com")
	req := suite.newAuthRequest(form.Encode(), client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *CIBAHandlerTestSuite) TestBackchannelAuth_RepeatedResourcePreserved() {
	client := &clientauth.OAuthClientInfo{
		ClientID: "client-1",
		OAuthApp: &providers.OAuthClient{ClientID: "client-1"},
	}
	suite.mockService.EXPECT().InitiateBackchannelAuth(mock.Anything, mock.MatchedBy(
		func(r *BackchannelAuthRequest) bool {
			return len(r.Resources) == 2 &&
				r.Resources[0] == "https://rs1.example.com" &&
				r.Resources[1] == "https://rs2.example.com"
		}), client.OAuthApp).Return(&BackchannelAuthResponse{
		AuthReqID: "auth-req-1",
		ExpiresIn: 120,
		Interval:  5,
	}, nil)

	form := url.Values{}
	form.Set("login_hint", "alice")
	form.Set("scope", "openid read:things")
	form["resource"] = []string{"https://rs1.example.com", "https://rs2.example.com"}
	req := suite.newAuthRequest(form.Encode(), client)
	w := httptest.NewRecorder()

	suite.handler.HandleBackchannelAuthRequest(w, req)

	suite.Equal(http.StatusOK, w.Code)
}

func (suite *CIBAHandlerTestSuite) TestRegisterRoutes_SetsCorrelationID() {
	mockDiscovery := discoverymock.NewDiscoveryServiceInterfaceMock(suite.T())
	mockDiscovery.On("GetOAuth2AuthorizationServerMetadata", mock.Anything).
		Return(&discovery.OAuth2AuthorizationServerMetadata{Issuer: "https://example.com"})
	mux := http.NewServeMux()
	registerRoutes(mux, suite.handler, nil, nil, nil, mockDiscovery, nil, engineconfig.ClientAssertionConfig{}, 0)

	req := httptest.NewRequest(http.MethodPost, oauth2const.OAuth2BackchannelAuthEndpoint, nil)
	req.Header.Set(serverconst.CorrelationIDHeaderName, "trace-abcd")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	suite.Equal("trace-abcd", rec.Header().Get(serverconst.CorrelationIDHeaderName))
}
