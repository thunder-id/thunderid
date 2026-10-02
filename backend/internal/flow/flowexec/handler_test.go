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
	h := newFlowExecutionHandler(mockSvc, session.NewCookieTransport(false), 0)
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
	h := newFlowExecutionHandler(mockSvc, session.NewCookieTransport(false), 0)

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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &ErrorDirectFlowInitiationNotPermitted)

	h := newFlowExecutionHandler(mockSvc, session.NewCookieTransport(false), 0)
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
		"", mock.Anything, "").Return(&FlowStep{
		ExecutionID: "execution-1", Status: providers.FlowStatusComplete,
	}, nil)
	h := newFlowExecutionHandler(mockSvc, session.NewCookieTransport(false), 0)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute",
		bytes.NewBufferString(`{"flowId":"administration-1","verbose":true}`))
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(flowStep, (*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewCookieTransport(false), 0)
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

	var gotInbound session.InboundHandle
	var gotOK bool
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(ctx context.Context, _ string, _ string, _ string, _ bool, _ string,
			_ map[string]string, _ string, _ string, _ string) {
			gotInbound, gotOK = session.InboundFrom(ctx)
		}).
		Return(&FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete},
			(*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewCookieTransport(false), 0)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: session.CookieName("flow-1"), Value: "inbound-handle"})
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	s.Require().True(gotOK, "inbound SSO transport inputs must be propagated onto the service context")
	s.Equal("inbound-handle", gotInbound.HandleFor("flow-1"))
}

func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_PropagatesClientInfo() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)

	var got session.ClientInfo
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(ctx context.Context, _ string, _ string, _ string, _ bool, _ string,
			_ map[string]string, _ string, _ string, _ string) {
			got = session.ClientInfoFrom(ctx)
		}).
		Return(&FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete},
			(*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewCookieTransport(false), 0)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) Chrome/140.0")
	req.RemoteAddr = "203.0.113.10:54321"
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	s.Equal("203.0.113.10", got.IP)
	s.Equal("Mozilla/5.0 (Macintosh) Chrome/140.0", got.UserAgent)
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(flowStep, (*tidcommon.ServiceError)(nil))

	// secure=true and a non-zero TTL so the emitted cookie carries the expected transport settings.
	h := newFlowExecutionHandler(mockSvc, session.NewCookieTransport(true), time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/flow/execute", bytes.NewBufferString(testFlowExecRequestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleFlowExecutionRequest(w, req)

	s.Equal(http.StatusOK, w.Code)
	var ssoCookie *http.Cookie
	for _, ck := range w.Result().Cookies() {
		if ck.Name == session.CookieName("flow-1") {
			ssoCookie = ck
		}
	}
	s.Require().NotNil(ssoCookie, "expected the per-flow SSO handle cookie to be set")
	s.Equal("minted-handle", ssoCookie.Value)
	s.Equal(int(time.Hour.Seconds()), ssoCookie.MaxAge)
	s.Positive(ssoCookie.MaxAge, "cookie TTL must be non-zero")
	s.True(ssoCookie.Secure)
	s.True(ssoCookie.HttpOnly)
}

// The Attestation-Token request header must be read and forwarded to the service layer as the
// attestation token argument.
func (s *HandlerTestSuite) TestHandleFlowExecutionRequest_AttestationTokenHeaderForwarded() {
	t := s.T()
	mockSvc := NewFlowExecServiceInterfaceMock(t)
	mockSvc.EXPECT().Execute(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "play-integrity-token").
		Return(&FlowStep{ExecutionID: "exec-1", Status: providers.FlowStatusIncomplete},
			(*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewCookieTransport(false), 0)
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(flowStep, (*tidcommon.ServiceError)(nil))

	h := newFlowExecutionHandler(mockSvc, session.NewCookieTransport(false), 0)
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
