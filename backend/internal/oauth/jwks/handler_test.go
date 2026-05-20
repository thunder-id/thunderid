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

package jwks

import (
	"github.com/thunder-id/thunderid/internal/system/i18n/core"

	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type JWKSHandlerTestSuite struct {
	suite.Suite
	mockService *JWKSServiceInterfaceMock
	handler     *jwksHandler
}

func TestJWKSHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(JWKSHandlerTestSuite))
}

func (s *JWKSHandlerTestSuite) SetupTest() {
	s.mockService = NewJWKSServiceInterfaceMock(s.T())
	s.handler = newJWKSHandler(s.mockService)
}

func (s *JWKSHandlerTestSuite) TestNewJWKSHandler() {
	handler := newJWKSHandler(s.mockService)
	assert.NotNil(s.T(), handler)
	assert.NotNil(s.T(), handler.jwksService)
}

func (s *JWKSHandlerTestSuite) TestHandleJWKSRequest_Success() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/jwks", nil)
	rr := httptest.NewRecorder()

	jwksResponse := &JWKSResponse{
		Keys: []JWKS{
			{
				Kid: "test-kid",
				Kty: "RSA",
				Use: "sig",
				Alg: "RS256",
				N:   "test-n",
				E:   "AQAB",
			},
		},
	}
	s.mockService.On("GetJWKS").Return(jwksResponse, nil)

	s.handler.HandleJWKSRequest(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Equal(s.T(), "application/json", rr.Header().Get("Content-Type"))

	var response JWKSResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), response.Keys, 1)
	assert.Equal(s.T(), "test-kid", response.Keys[0].Kid)
	s.mockService.AssertExpectations(s.T())
}

func (s *JWKSHandlerTestSuite) TestHandleJWKSRequest_ClientError() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/jwks", nil)
	rr := httptest.NewRecorder()

	svcErr := &serviceerror.ServiceError{
		Type:             serviceerror.ClientErrorType,
		Code:             "invalid_request",
		Error:            core.I18nMessage{Key: "error.test.invalid_request", DefaultValue: "invalid_request"},
		ErrorDescription: core.I18nMessage{Key: "error.test.invalid_request", DefaultValue: "Invalid request"},
	}
	s.mockService.On("GetJWKS").Return(nil, svcErr)

	s.handler.HandleJWKSRequest(rr, req)

	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	assert.Equal(s.T(), "application/json", rr.Header().Get("Content-Type"))
	s.mockService.AssertExpectations(s.T())
}

func (s *JWKSHandlerTestSuite) TestHandleJWKSRequest_ServiceError() {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/jwks", nil)
	rr := httptest.NewRecorder()

	svcErr := serviceerror.CustomServiceError(serviceerror.InternalServerError, core.I18nMessage{
		Key:          "error.test.failed_get_jwks",
		DefaultValue: "Failed to get JWKS",
	})
	s.mockService.On("GetJWKS").Return(nil, svcErr)

	s.handler.HandleJWKSRequest(rr, req)

	assert.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	assert.Equal(s.T(), "application/json", rr.Header().Get("Content-Type"))
	assert.Contains(s.T(), rr.Body.String(), svcErr.Code)
	s.mockService.AssertExpectations(s.T())
}
