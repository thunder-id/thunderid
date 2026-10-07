// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/discoverymock"
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
	app := &providers.OAuthClient{
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

	app := &providers.OAuthClient{ClientID: "test-client"}
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
	app := &providers.OAuthClient{ClientID: "test-client"}
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
	app := &providers.OAuthClient{ClientID: "test-client"}
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
	app := &providers.OAuthClient{ClientID: "test-client"}
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

	app := &providers.OAuthClient{ClientID: "test-client"}
	clientInfo := &clientauth.OAuthClientInfo{ClientID: "test-client", OAuthApp: app}
	ctx := context.WithValue(req.Context(), clientauth.OAuthClientKey, clientInfo)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.HandlePARRequest(rec, req)

	assert.Equal(s.T(), http.StatusInternalServerError, rec.Code)
}

func (s *HandlerTestSuite) TestRegisterRoutes_SetsCorrelationID() {
	mockDiscovery := discoverymock.NewDiscoveryServiceInterfaceMock(s.T())
	mockDiscovery.On("GetOAuth2AuthorizationServerMetadata", mock.Anything).
		Return(&discovery.OAuth2AuthorizationServerMetadata{Issuer: "https://example.com"})
	mux := http.NewServeMux()
	registerRoutes(mux, newPARHandler(nil, nil, ""), nil, nil, nil, mockDiscovery, nil, engineconfig.ClientAssertionConfig{}, 0)

	req := httptest.NewRequest(http.MethodPost, "/oauth2/par", nil)
	req.Header.Set(serverconst.CorrelationIDHeaderName, "trace-abcd")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	s.Equal("trace-abcd", rec.Header().Get(serverconst.CorrelationIDHeaderName))
}
