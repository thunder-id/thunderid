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

package granthandlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	rbacauthzmock "github.com/thunder-id/thunderid/tests/mocks/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

type GrantHandlerProviderTestSuite struct {
	suite.Suite
	provider             GrantHandlerProviderInterface
	mockJWTService       *jwtmock.JWTServiceInterfaceMock
	authzService         *authzmock.AuthorizeServiceInterfaceMock
	mockTokenBuilder     *tokenservicemock.TokenBuilderInterfaceMock
	mockTokenValidator   *tokenservicemock.TokenValidatorInterfaceMock
	mockAttrCacheService *attributecachemock.AttributeCacheServiceInterfaceMock
	mockOUService        *oumock.OrganizationUnitServiceInterfaceMock
	mockRBACAuthzService *rbacauthzmock.AuthorizationServiceInterfaceMock
	mockEntityProvider   *entityprovidermock.EntityProviderInterfaceMock
	mockResourceService  *resourcemock.ResourceServiceInterfaceMock
}

func TestGrantHandlerProviderSuite(t *testing.T) {
	suite.Run(t, new(GrantHandlerProviderTestSuite))
}

func (suite *GrantHandlerProviderTestSuite) SetupTest() {
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.authzService = authzmock.NewAuthorizeServiceInterfaceMock(suite.T())
	suite.mockTokenBuilder = tokenservicemock.NewTokenBuilderInterfaceMock(suite.T())
	suite.mockTokenValidator = tokenservicemock.NewTokenValidatorInterfaceMock(suite.T())
	suite.mockAttrCacheService = attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.mockRBACAuthzService = rbacauthzmock.NewAuthorizationServiceInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.provider = newGrantHandlerProvider(
		suite.mockJWTService,
		suite.authzService,
		suite.mockTokenBuilder,
		suite.mockTokenValidator,
		suite.mockAttrCacheService,
		suite.mockOUService,
		suite.mockRBACAuthzService,
		suite.mockEntityProvider,
		suite.mockResourceService,
	)
}

func (suite *GrantHandlerProviderTestSuite) TestNewGrantHandlerProvider() {
	provider := newGrantHandlerProvider(
		suite.mockJWTService,
		suite.authzService,
		suite.mockTokenBuilder,
		suite.mockTokenValidator,
		suite.mockAttrCacheService,
		suite.mockOUService,
		suite.mockRBACAuthzService,
		suite.mockEntityProvider,
		suite.mockResourceService,
	)
	assert.NotNil(suite.T(), provider)
	assert.Implements(suite.T(), (*GrantHandlerProviderInterface)(nil), provider)
}

func (suite *GrantHandlerProviderTestSuite) TestGetGrantHandler_ClientCredentials() {
	handler, err := suite.provider.GetGrantHandler(constants.GrantTypeClientCredentials)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), handler)
	assert.Implements(suite.T(), (*GrantHandlerInterface)(nil), handler)
}

func (suite *GrantHandlerProviderTestSuite) TestGetGrantHandler_AuthorizationCode() {
	handler, err := suite.provider.GetGrantHandler(constants.GrantTypeAuthorizationCode)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), handler)
	assert.Implements(suite.T(), (*GrantHandlerInterface)(nil), handler)
}

func (suite *GrantHandlerProviderTestSuite) TestGetGrantHandler_RefreshToken() {
	handler, err := suite.provider.GetGrantHandler(constants.GrantTypeRefreshToken)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), handler)
	assert.Implements(suite.T(), (*GrantHandlerInterface)(nil), handler)
	assert.Implements(suite.T(), (*RefreshTokenGrantHandlerInterface)(nil), handler)
}

func (suite *GrantHandlerProviderTestSuite) TestGetGrantHandler_UnsupportedGrantType() {
	unsupportedGrantTypes := []struct {
		name      string
		grantType constants.GrantType
	}{
		{"InvalidType", constants.GrantType("invalid_type")},
		{"EmptyType", constants.GrantType("")},
	}

	for _, tc := range unsupportedGrantTypes {
		suite.T().Run(tc.name, func(t *testing.T) {
			handler, err := suite.provider.GetGrantHandler(tc.grantType)

			assert.Error(t, err)
			assert.Nil(t, handler)
			assert.Equal(t, constants.UnSupportedGrantTypeError, err)
		})
	}
}

func (suite *GrantHandlerProviderTestSuite) TestGetGrantHandler_AllSupportedTypes() {
	supportedTypes := []constants.GrantType{
		constants.GrantTypeClientCredentials,
		constants.GrantTypeAuthorizationCode,
		constants.GrantTypeRefreshToken,
	}

	for _, grantType := range supportedTypes {
		suite.T().Run(string(grantType), func(t *testing.T) {
			handler, err := suite.provider.GetGrantHandler(grantType)

			assert.NoError(t, err)
			assert.NotNil(t, handler)
			assert.Implements(t, (*GrantHandlerInterface)(nil), handler)
		})
	}
}
