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

package flowbuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/flow/executormock"
)

type InitTestSuite struct {
	suite.Suite
	flowFactory      core.FlowFactoryInterface
	executorRegistry *executormock.ExecutorRegistryInterfaceMock
	cacheManager     cache.CacheManagerInterface
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) SetupTest() {
	_ = config.InitializeServerRuntime("test", &config.Config{
		Server: config.ServerConfig{Identifier: "test-deployment"},
	})
	s.flowFactory = core.Initialize()
	s.executorRegistry = executormock.NewExecutorRegistryInterfaceMock(s.T())
	s.cacheManager = cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment")
}

func (s *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *InitTestSuite) TestInitialize_ReturnsGraphBuilder() {
	builder := Initialize(s.cacheManager, s.flowFactory, s.executorRegistry)

	s.NotNil(builder)
}

func (s *InitTestSuite) TestInitialize_InvalidateCacheDoesNotPanic() {
	builder := Initialize(s.cacheManager, s.flowFactory, s.executorRegistry)

	s.NotPanics(func() {
		builder.InvalidateCache(context.Background(), "")
		builder.InvalidateCache(context.Background(), "flow-1")
	})
}

func (s *InitTestSuite) TestInitialize_GetGraphRejectsEmptyFlow() {
	builder := Initialize(s.cacheManager, s.flowFactory, s.executorRegistry)

	graph, err := builder.GetGraph(context.Background(), &common.CompleteFlowDefinition{
		ID:       "flow-1",
		FlowType: common.FlowTypeAuthentication,
		Nodes:    []common.NodeDefinition{},
	})

	s.Nil(graph)
	s.NotNil(err)
	s.Equal(ErrorInvalidFlowData.Code, err.Code)
}
