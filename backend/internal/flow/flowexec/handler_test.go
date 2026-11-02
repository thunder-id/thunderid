// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const testFlowExecRequestBody = `{"applicationId":"app-1","flowType":"AUTHENTICATION","action":"submit"}`

type HandlerTestSuite struct {
	suite.Suite
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) TestNewFlowExecutionHandler() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{SecureCookies: false}), 0)
	s.NotNil(h)
	s.Equal(mockSvc, h.flowExecService)
}

func (s *HandlerTestSuite) TestHandleFlowError_ClientError_Returns400() {
	w := httptest.NewRecorder()
	svcErr := &tidcommon.ServiceError{
		Code: "FES-4001",
		Type: tidcommon.ClientErrorType,
		Error: tidcommon.I18nMessage{
			Key:          "client.error",
			DefaultValue: "bad request",
		},
	}
	handleFlowError(context.Background(), w, svcErr, "")
	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowError_ForbiddenError_Returns403() {
	w := httptest.NewRecorder()
	handleFlowError(context.Background(), w, &ErrorDirectFlowInitiationNotPermitted, "")
	s.Equal(http.StatusForbidden, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowError_ApplicationNotUsable_Returns403() {
	w := httptest.NewRecorder()
	handleFlowError(context.Background(), w, &ErrorFlowApplicationNotUsable, "")
	s.Equal(http.StatusForbidden, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowError_AdministrationAuthenticationRequired_Returns401() {
	w := httptest.NewRecorder()
	handleFlowError(context.Background(), w, &ErrorAdministrationAuthenticationRequired, "")
	s.Equal(http.StatusUnauthorized, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowError_ServerError_Returns500() {
	w := httptest.NewRecorder()
	svcErr := &tidcommon.ServiceError{
		Code: "FES-5001",
		Type: tidcommon.ServerErrorType,
		Error: tidcommon.I18nMessage{
			Key:          "server.error",
			DefaultValue: "internal error",
		},
	}
	handleFlowError(context.Background(), w, svcErr, "")
	s.Equal(http.StatusInternalServerError, w.Code)
}

func (s *HandlerTestSuite) TestConvertToAPIError() {
	svcErr := &tidcommon.ServiceError{
		Code: "FES-1234",
		Error: tidcommon.I18nMessage{
			Key:          "test.error",
			DefaultValue: "test message",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "test.error.desc",
			DefaultValue: "test description",
		},
	}
	resp := convertToAPIError(svcErr)
	s.Equal("FES-1234", resp.Code)
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_InvalidJSON() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{SecureCookies: false}), 0)

	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)
	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_ServiceError() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &ErrorDirectFlowInitiationNotPermitted)

	h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{SecureCookies: false}), 0)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)
	s.Equal(http.StatusForbidden, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_ByFlowID() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	mockSvc.EXPECT().ExecuteByID(mock.Anything, "administration-1", "", true,
		"", mock.Anything, "", []SessionData{{FlowIdentifier: "identifier-1", SessionHandle: "body-handle"}}).
		Return(&FlowStep{
			ExecutionID: "execution-1", Status: providers.FlowStatusComplete,
		}, nil)
	h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{SecureCookies: false}), 0)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute",
		bytes.NewBufferString(`{"flowId":"administration-1","verbose":true,`+
			`"sessionData":[{"flowIdentifier":"identifier-1","sessionHandle":" body-handle "}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)
	s.Equal(http.StatusOK, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_Success() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	flowStep := &FlowStep{
		ExecutionID: "exec-1",
		Status:      providers.FlowStatusIncomplete,
	}
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(flowStep, (*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{SecureCookies: false}), 0)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)
	s.Equal(http.StatusOK, w.Code)
}

// TestHandleFlowExecutionRequest_PropagatesInboundSSOCookie verifies the inbound per-flow SSO
// cookie is read off the request and propagated onto the context handed to the flow service.
func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_PropagatesInboundSSOCookie() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)

	var gotInbound session.InboundHandleInterface
	var gotOK bool
	transport := session.NewHandleTransport(session.TransportConfig{SecureCookies: false})
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(ctx context.Context, _ string, _ string, _ string, _ bool, _ string,
			_ map[string]string, _ string, _ string, _ string, _ []SessionData) {
			gotInbound, gotOK = session.InboundFrom(ctx)
		}).
		Return(&FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete},
			(*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, transport, 0)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	// Obtain the per-flow cookie from the transport itself, as a browser would from an earlier response.
	issued := httptest.NewRecorder()
	transport.Write(&session.Exchange{Response: issued}, "flow-1", "inbound-handle", time.Hour)
	for _, ck := range issued.Result().Cookies() {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	s.Require().True(gotOK, "inbound SSO transport inputs must be propagated onto the service context")
	s.Equal("inbound-handle", gotInbound.HandleFor("flow-1"))
}

// TestHandleFlowExecutionRequest_CapturesCurrentRequest verifies the current step's request headers
// and query params are captured onto the service context, with credential headers stripped.
func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_CapturesCurrentRequest() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)

	var gotRequest *providers.CurrentRequest
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(ctx context.Context, _ string, _ string, _ string, _ bool, _ string,
			_ map[string]string, _ string, _ string, _ string, _ []SessionData) {
			gotRequest = common.CurrentRequestFrom(ctx)
		}).
		Return(&FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete},
			(*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{SecureCookies: false}), 0)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute?utm_source=newsletter&client_secret=supersecret",
		bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "curl/8.0")
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	s.Require().NotNil(gotRequest, "current request must be captured onto the service context")
	s.Equal("curl/8.0", http.Header(gotRequest.Headers).Get("User-Agent"))
	s.Empty(http.Header(gotRequest.Headers).Get("Authorization"), "credential headers must be stripped")
	// The inbound host is folded in under Host so {{request(current.header.Host)}} resolves.
	s.Equal("example.com", http.Header(gotRequest.Headers).Get("Host"))
	s.Require().Contains(gotRequest.QueryParams, "utm_source")
	s.Equal("newsletter", gotRequest.QueryParams["utm_source"][0])
	// client_secret is a credential query param and must be stripped at capture.
	s.NotContains(gotRequest.QueryParams, "client_secret", "credential query params must be stripped")
}

// TestHandleFlowExecutionRequest_WritesSSOHandleCookie verifies a minted handle is emitted as the
// per-flow cookie with the configured TTL and secure/http-only transport settings.
func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_WritesSSOHandleCookie() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	flowStep := &FlowStep{
		ExecutionID:  "exec-1",
		Status:       providers.FlowStatusComplete,
		SSOHandleOut: "minted-handle",
		SSOFlowID:    "flow-1",
	}
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(flowStep, (*tidcommon.ServiceError)(nil))

	// secure=true and a non-zero TTL so the emitted cookie carries the expected transport settings.
	transport := session.NewHandleTransport(session.TransportConfig{SecureCookies: true})
	h := newFlowExecutionHandler(mockSvc, transport, time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	cookies := w.Result().Cookies()
	s.Require().Len(cookies, 1, "expected the per-flow SSO handle cookie to be set")
	ssoCookie := cookies[0]
	// Read the cookie back through the transport to prove it is keyed by the step's SSO flow ID.
	next := httptest.NewRequest(http.MethodPost, "/flow/execute", nil)
	next.AddCookie(ssoCookie)
	s.Equal("minted-handle", transport.Read(&session.Exchange{Request: next}).HandleFor("flow-1"))
	s.Equal("minted-handle", ssoCookie.Value)
	s.Equal(int(time.Hour.Seconds()), ssoCookie.MaxAge)
	s.Positive(ssoCookie.MaxAge, "cookie TTL must be non-zero")
	s.True(ssoCookie.Secure)
	s.True(ssoCookie.HttpOnly)
}

// TestHandleFlowExecutionRequest_ReturnsSessionData verifies the sessions a step started and ended
// reach the response body.
func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_ReturnsSessionData() {
	issued := []IssuedSessionData{{
		SessionData: SessionData{FlowIdentifier: "identifier-1", SessionHandle: "minted-handle"},
		ExpiresIn:   3600,
	}}
	cleared := []string{"identifier-2"}
	mockSvc := NewFlowExecServiceInterfaceMock(s.T())
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).
		Return(&FlowStep{
			ExecutionID:     "exec-1",
			Status:          providers.FlowStatusComplete,
			SessionData:     issued,
			ClearedSessions: cleared,
		}, (*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{}), time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	var resp FlowResponse
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &resp))
	s.Equal(issued, resp.SessionData)
	s.Equal(cleared, resp.ClearedSessions)
}

// TestHandleFlowExecutionRequest_PassesRequestSessionData verifies the session data a caller sends in the
// request body reaches the flow service sanitized.
func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_PassesRequestSessionData() {
	mockSvc := NewFlowExecServiceInterfaceMock(s.T())
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		[]SessionData{
			{FlowIdentifier: "identifier-1", SessionHandle: "body-handle"},
			{FlowIdentifier: "identifier-2", SessionHandle: "other-handle"},
		}).
		Return(&FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusComplete},
			(*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{}), time.Hour)
	body := `{"applicationId":"app-1","flowType":"AUTHENTICATION","sessionData":[` +
		`{"flowIdentifier":" identifier-1 ","sessionHandle":" body-handle "},` +
		`{"flowIdentifier":"identifier-2","sessionHandle":"other-handle"}]}`
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_RejectsIncompleteSessionData() {
	cases := []struct {
		name  string
		entry string
	}{
		{"no flow identifier", `{"sessionHandle":"body-handle"}`},
		{"no session handle", `{"flowIdentifier":"identifier-1"}`},
		{"a blank session handle", `{"flowIdentifier":"identifier-1","sessionHandle":"  "}`},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			mockSvc := NewFlowExecServiceInterfaceMock(s.T())
			h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{}), time.Hour)
			body := `{"applicationId":"app-1","flowType":"AUTHENTICATION","sessionData":[` + tc.entry + `]}`
			req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			h.HandleFlowExecutionRequest(w, req)

			s.Equal(http.StatusBadRequest, w.Code)
			s.Contains(w.Body.String(), APIErrorInvalidSessionData.Code)
		})
	}
}

// TestHandleFlowExecutionRequest_NilSSOTransport verifies a deployment without an SSO handle transport
// attaches no inbound handle, emits no SSO cookie, and still serves the request.
func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_NilSSOTransport() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	flowStep := &FlowStep{
		ExecutionID:    "exec-1",
		Status:         providers.FlowStatusComplete,
		SSOHandleOut:   "minted-handle",
		SSOFlowID:      "flow-1",
		SSOClearFlowID: "flow-2",
	}
	gotOK := true
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(ctx context.Context, _ string, _ string, _ string, _ bool, _ string,
			_ map[string]string, _ string, _ string, _ string, _ []SessionData) {
			_, gotOK = session.InboundFrom(ctx)
		}).
		Return(flowStep, (*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, nil, time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	s.False(gotOK, "no inbound SSO handle must be attached without a transport")
	s.Empty(w.Result().Cookies(), "no SSO cookie must be emitted without a transport")
}

// TestHandleFlowExecutionRequest_ClearsSSOHandleCookie verifies a terminated session expires the
// per-flow cookie of the step's SSO clear flow ID.
func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_ClearsSSOHandleCookie() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	flowStep := &FlowStep{
		ExecutionID:    "exec-1",
		Status:         providers.FlowStatusComplete,
		SSOClearFlowID: "flow-1",
	}
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(flowStep, (*tidcommon.ServiceError)(nil))

	transport := session.NewHandleTransport(session.TransportConfig{SecureCookies: false})
	h := newFlowExecutionHandler(mockSvc, transport, time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	cookies := w.Result().Cookies()
	s.Require().Len(cookies, 1, "expected the per-flow SSO handle cookie to be cleared")
	s.Equal(-1, cookies[0].MaxAge)
	s.Equal("", cookies[0].Value)

	// The cleared cookie must carry the same name the transport issues for the flow.
	issued := httptest.NewRecorder()
	transport.Write(&session.Exchange{Response: issued}, "flow-1", "handle", time.Hour)
	s.Equal(issued.Result().Cookies()[0].Name, cookies[0].Name)
}

// The Attestation-Token request header must be read and forwarded to the service layer as the
// attestation token argument.
func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_AttestationTokenHeaderForwarded() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "play-integrity-token",
		mock.Anything).
		Return(&FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete},
			(*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{SecureCookies: false}), 0)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Attestation-Token", "play-integrity-token")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)
	s.Equal(http.StatusOK, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_StepWithError() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	stepErr := &tidcommon.ServiceError{
		Code: "FES-9999",
		Error: tidcommon.I18nMessage{
			Key:          "step.error",
			DefaultValue: "step failed",
		},
	}
	flowStep := &FlowStep{
		ExecutionID: "exec-1",
		Status:      providers.FlowStatusError,
		Error:       stepErr,
	}
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(flowStep, (*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewHandleTransport(session.TransportConfig{SecureCookies: false}), 0)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)
	s.Equal(http.StatusOK, w.Code)
}

func (s *HandlerTestSuite) TestHandleFlowError_WithErrorAssertion_IncludesAssertionInBody() {
	w := httptest.NewRecorder()
	svcErr := &tidcommon.ServiceError{
		Code: "FES-1013",
		Type: tidcommon.ClientErrorType,
		Error: tidcommon.I18nMessage{
			Key:          "client.error",
			DefaultValue: "bad request",
		},
	}

	handleFlowError(context.Background(), w, svcErr, "signed-error-assertion")

	s.Equal(http.StatusBadRequest, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	s.Equal("FES-1013", body["code"])
	s.Equal("signed-error-assertion", body["errorAssertion"])
	s.NotNil(body["message"])
}

func (s *HandlerTestSuite) TestHandleFlowError_WithoutErrorAssertion_OmitsAssertionField() {
	w := httptest.NewRecorder()
	svcErr := &tidcommon.ServiceError{
		Code: "FES-4001",
		Type: tidcommon.ClientErrorType,
		Error: tidcommon.I18nMessage{
			Key:          "client.error",
			DefaultValue: "bad request",
		},
	}

	handleFlowError(context.Background(), w, svcErr, "")

	s.Equal(http.StatusBadRequest, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	s.Equal("FES-4001", body["code"])
	_, hasAssertion := body["errorAssertion"]
	s.False(hasAssertion)
}
