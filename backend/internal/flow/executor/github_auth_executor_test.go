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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	"github.com/thunder-id/thunderid/tests/mocks/authn/githubmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

type GithubAuthExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory   *coremock.FlowFactoryInterfaceMock
	mockIDPService    *idpmock.IDPServiceInterfaceMock
	mockGithubService *githubmock.GithubOAuthAuthnServiceInterfaceMock
	mockOAuthService  *oauthmock.OAuthAuthnCoreServiceInterfaceMock
	mockAuthnProvider *managermock.AuthnProviderManagerMock
}

func TestGithubAuthExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(GithubAuthExecutorTestSuite))
}

func (suite *GithubAuthExecutorTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockGithubService = githubmock.NewGithubOAuthAuthnServiceInterfaceMock(suite.T())
	suite.mockOAuthService = oauthmock.NewOAuthAuthnCoreServiceInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
}

func (suite *GithubAuthExecutorTestSuite) TestNewGithubOAuthExecutor_Success() {
	defaultInputs := []providers.Input{
		{
			Identifier: "code",
			Type:       "string",
			Required:   true,
		},
	}
	baseExec := coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameGitHubAuth,
		providers.ExecutorTypeAuthentication, defaultInputs, []providers.Input{}, mock.Anything).
		Return(baseExec).Once()

	mockGithubSvc := &mockGithubServiceWithOAuth{
		GithubOAuthAuthnServiceInterfaceMock: suite.mockGithubService,
		oauthService:                         suite.mockOAuthService,
	}

	executor := newGithubOAuthExecutor(suite.mockFlowFactory, suite.mockIDPService,
		mockGithubSvc, suite.mockAuthnProvider)

	suite.NotNil(executor)
	githubExec, ok := executor.(*githubOAuthExecutor)
	suite.True(ok)
	suite.NotNil(githubExec.oAuthExecutorInterface)
	suite.Equal(mockGithubSvc, githubExec.githubAuthService)
}

type mockGithubServiceWithOAuth struct {
	*githubmock.GithubOAuthAuthnServiceInterfaceMock
	oauthService authnoauth.OAuthAuthnCoreServiceInterface
}
