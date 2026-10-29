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

	applyInboundSSO(engineCtx, ctx, nil)

	s.Equal("handle-1", engineCtx.SSOHandleIn)
}

func (s *ServiceSSOTestSuite) TestApplyInboundSSO_PrefersTheRequestBody() {
	cases := []struct {
		name        string
		sessionData []SessionData
		cookie      bool
		want        string
	}{
		{"a body handle for the flow wins over the cookie",
			[]SessionData{{FlowIdentifier: session.FlowIdentifier(testFlowID), SessionHandle: "body-handle"}},
			true, "body-handle"},
		{"a body handle without a cookie is used",
			[]SessionData{{FlowIdentifier: session.FlowIdentifier(testFlowID), SessionHandle: "body-handle"}},
			false, "body-handle"},
		{"a body handle sent only for another flow leaves the cookie",
			[]SessionData{{FlowIdentifier: session.FlowIdentifier("flow-2"), SessionHandle: "body-handle"}},
			true, "cookie-handle"},
		{"an empty body handle leaves the cookie",
			[]SessionData{{FlowIdentifier: session.FlowIdentifier(testFlowID)}},
			true, "cookie-handle"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			engineCtx := &EngineContext{Graph: s.newTestGraph()}
			ctx := context.Background()
			if tc.cookie {
				ctx = session.WithInbound(ctx, issuedCookieInbound(testFlowID, "cookie-handle"))
			}

			applyInboundSSO(engineCtx, ctx, tc.sessionData)

			s.Equal(tc.want, engineCtx.SSOHandleIn)
		})
	}
}

func (s *ServiceSSOTestSuite) TestApplyInboundSSO_SignOutSelectsBodyHandleOfLoginFlow() {
	engineCtx := &EngineContext{Graph: s.newTestGraph(), SessionFlowID: "login-flow"}

	applyInboundSSO(engineCtx, context.Background(), []SessionData{
		{FlowIdentifier: session.FlowIdentifier(testFlowID), SessionHandle: "signout-flow-handle"},
		{FlowIdentifier: session.FlowIdentifier("login-flow"), SessionHandle: "login-flow-handle"},
	})

	s.Equal("login-flow-handle", engineCtx.SSOHandleIn)
}

func (s *ServiceSSOTestSuite) TestApplyInboundSSO_NoInbound() {
	engineCtx := &EngineContext{Graph: s.newTestGraph()}

	applyInboundSSO(engineCtx, context.Background(), nil)

	s.Empty(engineCtx.SSOHandleIn)
}

func (s *ServiceSSOTestSuite) TestApplyOutboundSSO() {
	cases := []struct {
		name        string
		step        FlowStep
		wantData    []IssuedSessionData
		wantCleared []string
	}{
		{"a started session is returned keyed by flow identifier",
			FlowStep{SSOHandleOut: "minted-handle", SSOFlowID: testFlowID},
			[]IssuedSessionData{{
				SessionData: SessionData{
					FlowIdentifier: session.FlowIdentifier(testFlowID),
					SessionHandle:  "minted-handle",
				},
				ExpiresIn: int64(time.Hour.Seconds()),
			}}, nil},
		{"an ended session is listed as cleared",
			FlowStep{SSOClearFlowID: testFlowID},
			nil, []string{session.FlowIdentifier(testFlowID)}},
		{"a handle without its flow is not returned",
			FlowStep{SSOHandleOut: "minted-handle"}, nil, nil},
		{"a step that neither started nor ended a session returns none", FlowStep{}, nil, nil},
	}

	service := &flowExecService{ssoHandleTTL: time.Hour}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			step := tc.step

			service.applyOutboundSSO(&step)

			s.Equal(tc.wantData, step.SessionData)
			s.Equal(tc.wantCleared, step.ClearedSessions)
		})
	}
}

func (s *ServiceSSOTestSuite) TestApplyInboundSSO_NilGraph() {
	engineCtx := &EngineContext{}
	ctx := session.WithInbound(context.Background(), issuedCookieInbound(testFlowID, "handle-1"))

	applyInboundSSO(engineCtx, ctx, []SessionData{
		{FlowIdentifier: session.FlowIdentifier(testFlowID), SessionHandle: "body-handle"},
	})

	s.Empty(engineCtx.SSOHandleIn)
}
