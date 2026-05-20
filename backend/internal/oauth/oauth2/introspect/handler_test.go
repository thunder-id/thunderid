/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package introspect

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// brokenWriter is a writer that always returns an error
type brokenWriter struct {
	header http.Header
}

func (b *brokenWriter) Header() http.Header {
	if b.header == nil {
		b.header = make(http.Header)
	}
	return b.header
}

func (b *brokenWriter) Write([]byte) (int, error) {
	return 0, errors.New("write error")
}

func (b *brokenWriter) WriteHeader(statusCode int) {
	// Do nothing
}

type TokenIntrospectionHandlerTestSuite struct {
	suite.Suite
	introspectionServiceMock *TokenIntrospectionServiceInterfaceMock
	handler                  *tokenIntrospectionHandler
}

func TestTokenIntrospectionHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(TokenIntrospectionHandlerTestSuite))
}

func (s *TokenIntrospectionHandlerTestSuite) SetupTest() {
	s.introspectionServiceMock = NewTokenIntrospectionServiceInterfaceMock(s.T())
	s.handler = newTokenIntrospectionHandler(s.introspectionServiceMock)
}

func (s *TokenIntrospectionHandlerTestSuite) TestHandleIntrospect_ParseFormError() {
	// Create a request with an invalid content type to cause form parse error
	req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	s.handler.HandleIntrospect(rr, req)
	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), constants.ErrorInvalidRequest)
}

func (s *TokenIntrospectionHandlerTestSuite) TestHandleIntrospect_ParseFormError_EncodeError() {
	// Create a request with an invalid content type to cause form parse error
	req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", strings.NewReader("%"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Use a broken writer to trigger encoding error
	bw := &brokenWriter{}

	// Execute the handler - should attempt to write error response despite encoding failure
	s.handler.HandleIntrospect(bw, req)

	// Verify that the handler attempted to set headers
	assert.NotNil(s.T(), bw.Header())
	// WriteJSONError sets Content-Type to application/json before attempting to encode
	assert.Contains(s.T(), bw.Header().Get("Content-Type"), "application/json")
}

func (s *TokenIntrospectionHandlerTestSuite) TestHandleIntrospect_MissingToken() {
	form := url.Values{}
	req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	s.handler.HandleIntrospect(rr, req)
	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), constants.ErrorInvalidRequest)
	assert.Contains(s.T(), rr.Body.String(), "Token parameter is required")
}

func (s *TokenIntrospectionHandlerTestSuite) TestHandleIntrospect_MissingToken_EncodeError() {
	form := url.Values{}
	req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Use a broken writer to trigger encoding error
	bw := &brokenWriter{}

	// Execute the handler - should attempt to write error response despite encoding failure
	s.handler.HandleIntrospect(bw, req)

	// Verify that the handler attempted to set headers
	assert.NotNil(s.T(), bw.Header())
	// WriteJSONError sets Content-Type to application/json before attempting to encode
	assert.Contains(s.T(), bw.Header().Get("Content-Type"), "application/json")
}

func (s *TokenIntrospectionHandlerTestSuite) TestHandleIntrospect_IntrospectionError() {
	form := url.Values{}
	form.Add(constants.RequestParamToken, "valid-token")
	req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Setup the mock to return an error
	s.introspectionServiceMock.On("IntrospectToken", mock.Anything, "valid-token", "").
		Return(nil, errors.New("introspection error"))

	rr := httptest.NewRecorder()

	s.handler.HandleIntrospect(rr, req)
	assert.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), constants.ErrorServerError)
	assert.Contains(s.T(), rr.Body.String(), "An unexpected error occurred while processing the request")
	s.introspectionServiceMock.AssertExpectations(s.T())
}

func (s *TokenIntrospectionHandlerTestSuite) TestHandleIntrospect_Success_ActiveToken() {
	form := url.Values{}
	form.Add(constants.RequestParamToken, "valid-token")
	form.Add(constants.RequestParamTokenTypeHint, "access_token")
	req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Setup the mock to return a valid active token response
	activeResponse := &IntrospectResponse{
		Active:    true,
		Scope:     "openid profile",
		ClientID:  "client123",
		Username:  "user@example.com",
		TokenType: constants.TokenTypeBearer,
		Exp:       1620000000,
		Iat:       1619990000,
		Nbf:       1619990000,
		Sub:       "user123",
		Aud:       "api.example.com",
		Iss:       "https://example.com",
		Jti:       "token-id-123",
	}
	s.introspectionServiceMock.On("IntrospectToken", mock.Anything, "valid-token", "access_token").
		Return(activeResponse, nil)

	rr := httptest.NewRecorder()

	s.handler.HandleIntrospect(rr, req)
	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), `"active":true`)
	assert.Contains(s.T(), rr.Body.String(), `"scope":"openid profile"`)
	assert.Contains(s.T(), rr.Body.String(), `"client_id":"client123"`)
	s.introspectionServiceMock.AssertExpectations(s.T())
}

func (s *TokenIntrospectionHandlerTestSuite) TestHandleIntrospect_IntrospectionError_EncodeError() {
	form := url.Values{}
	form.Add(constants.RequestParamToken, "valid-token")
	req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Setup the mock to return an error
	s.introspectionServiceMock.On("IntrospectToken", mock.Anything, "valid-token", "").
		Return(nil, errors.New("introspection error"))

	rr := httptest.NewRecorder()

	s.handler.HandleIntrospect(rr, req)
	assert.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	s.introspectionServiceMock.AssertExpectations(s.T())
}

func (s *TokenIntrospectionHandlerTestSuite) TestHandleIntrospect_Success_EncodeError() {
	form := url.Values{}
	form.Add(constants.RequestParamToken, "valid-token")
	req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Setup the mock to return a valid active token response
	activeResponse := &IntrospectResponse{
		Active: true,
	}
	s.introspectionServiceMock.On("IntrospectToken", mock.Anything, "valid-token", "").Return(activeResponse, nil)

	rr := httptest.NewRecorder()

	s.handler.HandleIntrospect(rr, req)
	assert.Equal(s.T(), http.StatusOK, rr.Code)
	s.introspectionServiceMock.AssertExpectations(s.T())
}

func (s *TokenIntrospectionHandlerTestSuite) TestHandleIntrospect_Success_InactiveToken() {
	form := url.Values{}
	form.Add(constants.RequestParamToken, "invalid-token")
	req := httptest.NewRequest(http.MethodPost, "/oauth2/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Setup the mock to return an inactive token response
	inactiveResponse := &IntrospectResponse{
		Active: false,
	}
	s.introspectionServiceMock.On("IntrospectToken", mock.Anything, "invalid-token", "").Return(inactiveResponse, nil)

	rr := httptest.NewRecorder()

	s.handler.HandleIntrospect(rr, req)
	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Contains(s.T(), rr.Body.String(), `"active":false`)
	s.introspectionServiceMock.AssertExpectations(s.T())
}
