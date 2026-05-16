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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/dpopmock"
)

const testResponseTypeCodeBody = "response_type=code"

type HandlerTestSuite struct {
	suite.Suite
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) SetupTest() {
	testConfig := &config.Config{}
	_ = config.InitializeServerRuntime("", testConfig)
}

func (s *HandlerTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *HandlerTestSuite) TestHandlePAR_Success() {
	svc := NewPARServiceInterfaceMock(s.T())
	svc.EXPECT().HandlePushedAuthorizationRequest(mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).
		Return(&parResponse{
			RequestURI: requestURIPrefix + "test",
			ExpiresIn:  60,
		}, "", "")
	handler := newPARHandler(svc, nil, "https://example.test/oauth2/par")

	body := "response_type=code&redirect_uri=https%3A%2F%2Fexample.com%2Fcallback&scope=openid"
	req := httptest.NewRequest(http.MethodPost, "/oauth2/par", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Set authenticated client in context.
	app := &inboundmodel.OAuthClient{
		ClientID: "test-client",
	}
	clientInfo := &clientauth.OAuthClientInfo{
		ClientID: "test-client",
		OAuthApp: app,
	}
	ctx := context.WithValue(req.Context(), clientauth.OAuthClientKey, clientInfo)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.HandlePARRequest(rec, req)

	assert.Equal(s.T(), http.StatusCreated, rec.Code)

	var resp parResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	assert.NoError(s.T(), err)
	assert.True(s.T(), strings.HasPrefix(resp.RequestURI, requestURIPrefix))
	assert.Equal(s.T(), int64(60), resp.ExpiresIn)
}

func (s *HandlerTestSuite) TestHandlePAR_NoClientAuth() {
	svc := NewPARServiceInterfaceMock(s.T())
	handler := newPARHandler(svc, nil, "https://example.test/oauth2/par")

	req := httptest.NewRequest(http.MethodPost, "/oauth2/par", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	handler.HandlePARRequest(rec, req)

	assert.Equal(s.T(), http.StatusInternalServerError, rec.Code)
}

func (s *HandlerTestSuite) TestHandlePAR_ValidationError() {
	svc := NewPARServiceInterfaceMock(s.T())
	svc.EXPECT().HandlePushedAuthorizationRequest(mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).
		Return(nil, oauth2const.ErrorInvalidRequest, "Missing response_type parameter")
	handler := newPARHandler(svc, nil, "https://example.test/oauth2/par")

	body := "scope=openid"
	req := httptest.NewRequest(http.MethodPost, "/oauth2/par", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	app := &inboundmodel.OAuthClient{ClientID: "test-client"}
	clientInfo := &clientauth.OAuthClientInfo{ClientID: "test-client", OAuthApp: app}
	ctx := context.WithValue(req.Context(), clientauth.OAuthClientKey, clientInfo)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.HandlePARRequest(rec, req)

	assert.Equal(s.T(), http.StatusBadRequest, rec.Code)

	var errResp map[string]string
	err := json.NewDecoder(rec.Body).Decode(&errResp)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), oauth2const.ErrorInvalidRequest, errResp["error"])
}

func (s *HandlerTestSuite) TestHandlePAR_DPoPHeaderForwardedAsJkt() {
	svc := NewPARServiceInterfaceMock(s.T())
	verifier := dpopmock.NewVerifierInterfaceMock(s.T())
	verifier.EXPECT().Verify(mock.Anything, mock.MatchedBy(func(p dpop.VerifyParams) bool {
		return p.Proof == "proof-jwt" && p.HTM == http.MethodPost &&
			p.HTU == "https://example.test/oauth2/par"
	})).Return(&dpop.ProofResult{JKT: "thumbprint-abc"}, nil)
	svc.EXPECT().HandlePushedAuthorizationRequest(
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, "thumbprint-abc",
	).Return(&parResponse{RequestURI: requestURIPrefix + "x", ExpiresIn: 60}, "", "")
	handler := newPARHandler(svc, verifier, "https://example.test/oauth2/par")

	body := "response_type=code&redirect_uri=https%3A%2F%2Fexample.com%2Fcallback&scope=openid"
	req := httptest.NewRequest(http.MethodPost, "/oauth2/par", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(oauth2const.HeaderDPoP, "proof-jwt")
	app := &inboundmodel.OAuthClient{ClientID: "test-client"}
	clientInfo := &clientauth.OAuthClientInfo{ClientID: "test-client", OAuthApp: app}
	req = req.WithContext(context.WithValue(req.Context(), clientauth.OAuthClientKey, clientInfo))

	rec := httptest.NewRecorder()
	handler.HandlePARRequest(rec, req)

	assert.Equal(s.T(), http.StatusCreated, rec.Code)
}

func (s *HandlerTestSuite) TestHandlePAR_MultipleDPoPHeaders_Rejected() {
	svc := NewPARServiceInterfaceMock(s.T())
	verifier := dpopmock.NewVerifierInterfaceMock(s.T())
	handler := newPARHandler(svc, verifier, "https://example.test/oauth2/par")

	body := testResponseTypeCodeBody
	req := httptest.NewRequest(http.MethodPost, "/oauth2/par", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add(oauth2const.HeaderDPoP, "proof-1")
	req.Header.Add(oauth2const.HeaderDPoP, "proof-2")
	app := &inboundmodel.OAuthClient{ClientID: "test-client"}
	clientInfo := &clientauth.OAuthClientInfo{ClientID: "test-client", OAuthApp: app}
	req = req.WithContext(context.WithValue(req.Context(), clientauth.OAuthClientKey, clientInfo))

	rec := httptest.NewRecorder()
	handler.HandlePARRequest(rec, req)

	assert.Equal(s.T(), http.StatusBadRequest, rec.Code)
	var errResp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&errResp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidDPoPProof, errResp["error"])
}

func (s *HandlerTestSuite) TestHandlePAR_InvalidDPoPProof_Rejected() {
	svc := NewPARServiceInterfaceMock(s.T())
	verifier := dpopmock.NewVerifierInterfaceMock(s.T())
	verifier.EXPECT().Verify(mock.Anything, mock.Anything).
		Return(nil, errors.New("bad proof"))
	handler := newPARHandler(svc, verifier, "https://example.test/oauth2/par")

	body := testResponseTypeCodeBody
	req := httptest.NewRequest(http.MethodPost, "/oauth2/par", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(oauth2const.HeaderDPoP, "proof-jwt")
	app := &inboundmodel.OAuthClient{ClientID: "test-client"}
	clientInfo := &clientauth.OAuthClientInfo{ClientID: "test-client", OAuthApp: app}
	req = req.WithContext(context.WithValue(req.Context(), clientauth.OAuthClientKey, clientInfo))

	rec := httptest.NewRecorder()
	handler.HandlePARRequest(rec, req)

	assert.Equal(s.T(), http.StatusBadRequest, rec.Code)
	var errResp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&errResp)
	assert.Equal(s.T(), oauth2const.ErrorInvalidDPoPProof, errResp["error"])
}

func (s *HandlerTestSuite) TestHandlePAR_ServerError() {
	svc := NewPARServiceInterfaceMock(s.T())
	svc.EXPECT().HandlePushedAuthorizationRequest(mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).
		Return(nil, oauth2const.ErrorServerError, "Internal error")
	handler := newPARHandler(svc, nil, "https://example.test/oauth2/par")

	body := testResponseTypeCodeBody
	req := httptest.NewRequest(http.MethodPost, "/oauth2/par", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	app := &inboundmodel.OAuthClient{ClientID: "test-client"}
	clientInfo := &clientauth.OAuthClientInfo{ClientID: "test-client", OAuthApp: app}
	ctx := context.WithValue(req.Context(), clientauth.OAuthClientKey, clientInfo)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.HandlePARRequest(rec, req)

	assert.Equal(s.T(), http.StatusInternalServerError, rec.Code)
}
