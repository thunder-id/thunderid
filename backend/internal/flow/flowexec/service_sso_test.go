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

package flowexec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const testFlowID = "auth-graph-1"

type ServiceSSOTestSuite struct {
	suite.Suite
}

func TestServiceSSOTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceSSOTestSuite))
}

func (s *ServiceSSOTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
}

func (s *ServiceSSOTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *ServiceSSOTestSuite) newTestGraph() core.GraphInterface {
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	return flowFactory.CreateGraph(testFlowID, providers.FlowTypeAuthentication, 1)
}

func (s *ServiceSSOTestSuite) TestApplyInboundSSO_SelectsHandleForFlow() {
	engineCtx := &EngineContext{Graph: s.newTestGraph()}

	ih := session.InboundHandle{
		Cookies: map[string]string{session.CookieName(testFlowID): "handle-1"},
	}
	ctx := session.WithInbound(context.Background(), ih)

	applyInboundSSO(engineCtx, ctx)

	s.Equal("handle-1", engineCtx.SSOHandleIn)
}

func (s *ServiceSSOTestSuite) TestApplyInboundSSO_NoInbound() {
	engineCtx := &EngineContext{Graph: s.newTestGraph()}

	applyInboundSSO(engineCtx, context.Background())

	s.Empty(engineCtx.SSOHandleIn)
}

func (s *ServiceSSOTestSuite) TestApplyInboundSSO_NilGraph() {
	engineCtx := &EngineContext{}
	ctx := session.WithInbound(context.Background(),
		session.InboundHandle{Cookies: map[string]string{}})

	applyInboundSSO(engineCtx, ctx)

	s.Empty(engineCtx.SSOHandleIn)
}
