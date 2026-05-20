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

package userinfo

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type InitTestSuite struct {
	suite.Suite
	mockJWTService            *jwtmock.JWTServiceInterfaceMock
	mockTokenValidator        *tokenservicemock.TokenValidatorInterfaceMock
	mockInboundClient         *inboundclientmock.InboundClientServiceInterfaceMock
	mockOUService             *oumock.OrganizationUnitServiceInterfaceMock
	mockAttributeCacheService *attributecachemock.AttributeCacheServiceInterfaceMock
	mockTransactioner         *MockTransactioner
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockTokenValidator = tokenservicemock.NewTokenValidatorInterfaceMock(suite.T())
	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.mockAttributeCacheService = attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
	suite.mockTransactioner = &MockTransactioner{}
}

func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()

	service := Initialize(mux, suite.mockJWTService, nil, nil,
		suite.mockTokenValidator, suite.mockInboundClient,
		suite.mockOUService, suite.mockAttributeCacheService, suite.mockTransactioner)

	assert.NotNil(suite.T(), service)
}

func (suite *InitTestSuite) TestInitialize_RegistersRoutes() {
	mux := http.NewServeMux()

	Initialize(mux, suite.mockJWTService, nil, nil,
		suite.mockTokenValidator, suite.mockInboundClient,
		suite.mockOUService, suite.mockAttributeCacheService, suite.mockTransactioner)

	// Verify that the routes are registered by attempting to get a handler for them.
	// The pattern includes the method because of CORS middleware wrapping.
	_, pattern := mux.Handler(&http.Request{Method: "GET", URL: &url.URL{Path: "/oauth2/userinfo"}})
	assert.Contains(suite.T(), pattern, "/oauth2/userinfo")

	_, pattern = mux.Handler(&http.Request{Method: "POST", URL: &url.URL{Path: "/oauth2/userinfo"}})
	assert.Contains(suite.T(), pattern, "/oauth2/userinfo")

	_, pattern = mux.Handler(&http.Request{Method: "OPTIONS", URL: &url.URL{Path: "/oauth2/userinfo"}})
	assert.Contains(suite.T(), pattern, "/oauth2/userinfo")
}
