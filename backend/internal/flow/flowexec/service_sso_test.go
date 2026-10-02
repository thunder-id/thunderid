// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

	ctx := session.WithInbound(context.Background(), session.NewStaticInbound(testFlowID, "handle-1"))

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
	ctx := session.WithInbound(context.Background(), session.NewStaticInbound(testFlowID, "handle-1"))

	applyInboundSSO(engineCtx, ctx)

	s.Empty(engineCtx.SSOHandleIn)
}
