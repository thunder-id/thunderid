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

package executor

import (
	"testing"

	"github.com/stretchr/testify/suite"

	authnoidc "github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/tests/mocks/authn/googlemock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oidcmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

type GoogleAuthExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory       *coremock.FlowFactoryInterfaceMock
	mockIDPService        *idpmock.IDPServiceInterfaceMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	mockGoogleService     *googlemock.GoogleOIDCAuthnServiceInterfaceMock
	mockOIDCService       *oidcmock.OIDCAuthnCoreServiceInterfaceMock
	mockAuthnProvider     *managermock.AuthnProviderManagerInterfaceMock
}

func TestGoogleAuthExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(GoogleAuthExecutorTestSuite))
}

func (suite *GoogleAuthExecutorTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.mockGoogleService = googlemock.NewGoogleOIDCAuthnServiceInterfaceMock(suite.T())
	suite.mockOIDCService = oidcmock.NewOIDCAuthnCoreServiceInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
}

func (suite *GoogleAuthExecutorTestSuite) TestNewGoogleOIDCAuthExecutor_Success() {
	defaultInputs := []common.Input{
		{
			Identifier: "code",
			Type:       "string",
			Required:   true,
		},
		{
			Identifier: "nonce",
			Type:       "string",
			Required:   false,
		},
	}
	baseExec := coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameGoogleAuth,
		common.ExecutorTypeAuthentication, defaultInputs, []common.Input{}).
		Return(baseExec).Once()

	mockGoogleSvc := &mockGoogleServiceWithOIDC{
		GoogleOIDCAuthnServiceInterfaceMock: suite.mockGoogleService,
		oidcService:                         suite.mockOIDCService,
	}

	executor := newGoogleOIDCAuthExecutor(suite.mockFlowFactory, suite.mockIDPService,
		suite.mockEntityTypeService, mockGoogleSvc, suite.mockAuthnProvider)

	suite.NotNil(executor)
	googleExec, ok := executor.(*googleOIDCAuthExecutor)
	suite.True(ok)
	suite.NotNil(googleExec.oidcAuthExecutorInterface)
	suite.Equal(mockGoogleSvc, googleExec.googleAuthService)
}

type mockGoogleServiceWithOIDC struct {
	*googlemock.GoogleOIDCAuthnServiceInterfaceMock
	oidcService authnoidc.OIDCAuthnCoreServiceInterface
}
