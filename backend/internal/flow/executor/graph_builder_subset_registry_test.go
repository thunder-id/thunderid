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

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/authn/githubmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/googlemock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oidcmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
)

type GraphBuilderSubsetRegistryTestSuite struct {
	suite.Suite
}

func TestGraphBuilderSubsetRegistrySuite(t *testing.T) {
	suite.Run(t, new(GraphBuilderSubsetRegistryTestSuite))
}

func (s *GraphBuilderSubsetRegistryTestSuite) SetupSuite() {
	_ = config.InitializeServerRuntime("test", &config.Config{
		Server: config.ServerConfig{Identifier: "test-deployment"},
	})
}

func (s *GraphBuilderSubsetRegistryTestSuite) subsetExecutorRegistry() core.ExecutorRegistryInterface {
	reg, err := Initialize(ExecutorDependencies{
		EntityProvider: entityprovidermock.NewEntityProviderInterfaceMock(s.T()),
		OAuthSvc:       oauthmock.NewOAuthAuthnServiceInterfaceMock(s.T()),
		OIDCSvc:        oidcmock.NewOIDCAuthnServiceInterfaceMock(s.T()),
		GithubSvc:      githubmock.NewGithubOAuthAuthnServiceInterfaceMock(s.T()),
		GoogleSvc:      googlemock.NewGoogleOIDCAuthnServiceInterfaceMock(s.T()),
	}, config.FlowConfig{
		Executors: []string{ExecutorNameInviteExecutor},
	})
	require.NoError(s.T(), err)
	return reg
}

func (s *GraphBuilderSubsetRegistryTestSuite) TestBuildGraph_SubsetRegistryRejectsUnregisteredExecutor() {
	execRegistry := s.subsetExecutorRegistry()
	require.False(s.T(), execRegistry.IsRegistered(ExecutorNameBasicAuth))

	cacheManager := cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment")
	builder := core.InitializeGraphBuilder(cacheManager, execRegistry)

	flow := &common.CompleteFlowDefinition{
		ID:       "flow-1",
		Handle:   "test-handle",
		Name:     "Test Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []common.NodeDefinition{
			{ID: "start", Type: string(common.NodeTypeStart)},
			{
				ID:        "task",
				Type:      string(common.NodeTypeTaskExecution),
				Executor:  &common.ExecutorDefinition{Name: ExecutorNameBasicAuth},
				OnSuccess: "end",
			},
			{ID: "end", Type: string(common.NodeTypeEnd)},
		},
	}

	graph, err := builder.GetGraph(context.Background(), flow)

	s.Nil(graph)
	s.Require().NotNil(err)
	s.Equal(core.ErrorGraphBuildFailure.Code, err.Code)
}
