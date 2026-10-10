// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// issuedCookieInbound returns the inbound handle the transport reads from a request carrying the
// per-flow SSO cookie it issued for flowID, as a browser would send it back.
func issuedCookieInbound(flowID, handle string) session.InboundHandleInterface {
	transport := session.NewHandleTransport(session.TransportConfig{})
	issued := httptest.NewRecorder()
	transport.Write(&session.Exchange{Response: issued}, flowID, handle, time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", nil)
	for _, ck := range issued.Result().Cookies() {
		req.AddCookie(ck)
	}
	return transport.Read(&session.Exchange{Request: req})
}

func (s *ServiceSSOTestSuite) newTestGraph() core.GraphInterface {
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	return flowFactory.CreateGraph(testFlowID, providers.FlowTypeAuthentication, 1)
}

func (s *ServiceSSOTestSuite) TestApplyInboundSSO_SelectsHandleForFlow() {
	engineCtx := &EngineContext{Graph: s.newTestGraph()}

	ctx := session.WithInbound(context.Background(), issuedCookieInbound(testFlowID, "handle-1"))

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
	ctx := session.WithInbound(context.Background(), issuedCookieInbound(testFlowID, "handle-1"))

	applyInboundSSO(engineCtx, ctx)

	s.Empty(engineCtx.SSOHandleIn)
}
