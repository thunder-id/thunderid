// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/connection/httpconnectionmock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
)

const (
	testExtensionConnectionID = "conn-risk"
	testExtensionPassword     = "svc-password-value"
)

// recordedExtensionRequest is one request the fake service received.
type recordedExtensionRequest struct {
	method string
	header http.Header
	body   []byte
}

type ExtensionExecutorTestSuite struct {
	suite.Suite
	executor    *extensionExecutor
	connections *httpconnectionmock.HTTPConnectionServiceInterfaceMock
	events      *observabilityprovidermock.ObservabilityProviderMock
	server      *httptest.Server

	mu        sync.Mutex
	requests  []recordedExtensionRequest
	published []*providers.Event
}

func TestExtensionExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionExecutorTestSuite))
}

func (s *ExtensionExecutorTestSuite) SetupSuite() {
	s.Require().NoError(config.InitializeServerRuntime("test", &config.Config{}))
}

func (s *ExtensionExecutorTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

func (s *ExtensionExecutorTestSuite) SetupTest() {
	s.requests = nil
	s.published = nil
	s.connections = httpconnectionmock.NewHTTPConnectionServiceInterfaceMock(s.T())
	s.events = observabilityprovidermock.NewObservabilityProviderMock(s.T())
	s.events.EXPECT().IsEnabled().Return(true).Maybe()
	s.events.EXPECT().PublishEvent(mock.Anything, mock.Anything).Run(func(_ context.Context, evt *providers.Event) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.published = append(s.published, evt)
	}).Maybe()
	s.executor = newExtensionExecutor(s.flowFactory(), s.connections, s.events)
}

func (s *ExtensionExecutorTestSuite) TearDownTest() {
	if s.server != nil {
		s.server.Close()
		s.server = nil
	}
}

func (s *ExtensionExecutorTestSuite) flowFactory() *coremock.FlowFactoryInterfaceMock {
	factory := coremock.NewFlowFactoryInterfaceMock(s.T())
	factory.On("CreateExecutor", ExecutorNameExtension, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, mock.Anything).
		Return(newMockExecutor(ExecutorNameExtension, providers.ExecutorTypeUtility,
			[]providers.Input{}, []providers.Input{}))
	return factory
}

// serve starts the fake service with handler and records every request it receives.
func (s *ExtensionExecutorTestSuite) serve(handler http.HandlerFunc) {
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.requests = append(s.requests, recordedExtensionRequest{method: r.Method, header: r.Header.Clone(),
			body: body})
		s.mu.Unlock()
		handler(w, r)
	}))
}

// answer starts the fake service answering every request with status and body.
func (s *ExtensionExecutorTestSuite) answer(status int, body string) {
	s.serve(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

// resolveTo makes the connection resolve to url with the headers a real basic-authenticated
// connection carries.
func (s *ExtensionExecutorTestSuite) resolveTo(url string, timeout time.Duration) {
	headers := http.Header{"X-Tenant": {"acme"}}
	authenticator, err := httpauth.New(outboundauth.Config{Type: outboundauth.TypeBasic,
		Properties: map[string]string{
			outboundauth.FieldBasicUsername: "svc-extension", outboundauth.FieldBasicPassword: testExtensionPassword,
		}})
	s.Require().NoError(err)
	req := httptest.NewRequest(http.MethodPost, "https://extension.example.com/", nil)
	req.Header = headers
	s.Require().NoError(authenticator.Authenticate(context.Background(), req))
	s.connections.EXPECT().GetResolvedConnection(mock.Anything, testExtensionConnectionID).
		Return(&httpconnection.ResolvedHTTPConnection{URL: url, Timeout: timeout, Headers: headers}, nil)
}

// nodeContext is a sign-in step after password authentication, holding a password input and a mix
// of runtime data.
func (s *ExtensionExecutorTestSuite) nodeContext() *providers.NodeContext {
	return &providers.NodeContext{
		Context:       context.Background(),
		ExecutionID:   "019a1b2c-3d4e-7f60-8a9b-0c1d2e3f4a5b",
		FlowType:      providers.FlowTypeAuthentication,
		CurrentNodeID: "risk_check",
		UserInputs:    map[string]string{"username": "alice", "password": "alice-password"},
		RuntimeData: map[string]string{
			common.RuntimeKeyApplicationID: "app-1",
			userAttributeUserID:            "user-1",
			common.RuntimeKeyClientID:      "client-1",
			"riskTier":                     "gold",
		},
		NodeProperties: map[string]interface{}{common.NodePropertyConnectionID: testExtensionConnectionID},
	}
}

// run serves body with status 200, executes the step and returns the response.
func (s *ExtensionExecutorTestSuite) run(ctx *providers.NodeContext, body string) *providers.ExecutorResponse {
	s.answer(http.StatusOK, body)
	s.resolveTo(s.server.URL, 2*time.Second)
	execResp, err := s.executor.Execute(ctx)
	s.Require().NoError(err)
	return execResp
}

// answerInTurn starts the fake service answering each request with the next body, repeating the
// last, and resolves the step's connection to it.
func (s *ExtensionExecutorTestSuite) answerInTurn(bodies ...string) {
	next := 0
	s.serve(func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		body := bodies[min(next, len(bodies)-1)]
		next++
		s.mu.Unlock()
		_, _ = w.Write([]byte(body))
	})
	s.resolveTo(s.server.URL, 2*time.Second)
}

// arrive executes the step as the engine does each time the flow reaches it, merging the response's
// runtime data into the flow's.
func (s *ExtensionExecutorTestSuite) arrive(ctx *providers.NodeContext) *providers.ExecutorResponse {
	execResp, err := s.executor.Execute(ctx)
	s.Require().NoError(err)
	ctx.RuntimeData = sysutils.MergeStringMaps(ctx.RuntimeData, execResp.RuntimeData)
	return execResp
}

// sentBody returns the JSON body of the only request the service received.
func (s *ExtensionExecutorTestSuite) sentBody() map[string]json.RawMessage {
	s.Require().Len(s.requests, 1)
	var body map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(s.requests[0].body, &body))
	return body
}

func (s *ExtensionExecutorTestSuite) assertCallFailed(execResp *providers.ExecutorResponse) {
	s.T().Helper()
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Require().NotNil(execResp.Error)
	s.Equal(ErrExtensionCallFailed.Code, execResp.Error.Code)
	s.Empty(execResp.Error.ErrorDescription.Params)
	s.Empty(execResp.RuntimeData)
}

func (s *ExtensionExecutorTestSuite) assertConfigInvalidWithoutCall(execResp *providers.ExecutorResponse) {
	s.T().Helper()
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Require().NotNil(execResp.Error)
	s.Equal(ErrExtensionConfigInvalid.Code, execResp.Error.Code)
	s.Empty(s.requests, "no request may be sent")
}

func (s *ExtensionExecutorTestSuite) onlyEvent() *providers.Event {
	s.T().Helper()
	s.Require().Len(s.published, 1, "exactly one audit event per execution")
	return s.published[0]
}

// R1

func (s *ExtensionExecutorTestSuite) TestExecute_PostsToTheConnectionWithItsHeaders() {
	execResp := s.run(s.nodeContext(), `{"actionStatus":"SUCCESS"}`)

	s.Equal(providers.ExecComplete, execResp.Status)
	s.Require().Len(s.requests, 1)
	sent := s.requests[0]
	s.Equal(http.MethodPost, sent.method)
	s.Equal("application/json", sent.header.Get("Content-Type"))
	s.Equal("acme", sent.header.Get("X-Tenant"))
	username, password, ok := (&http.Request{Header: sent.header}).BasicAuth()
	s.True(ok)
	s.Equal("svc-extension", username)
	s.Equal(testExtensionPassword, password)
	s.NotEmpty(sent.header.Get("X-Correlation-ID"))
}

func (s *ExtensionExecutorTestSuite) TestExecute_SendsTheFlowTraceIDAsCorrelationID() {
	ctx := s.nodeContext()
	ctx.Context = sysContext.WithTraceID(context.Background(), "trace-123")

	s.run(ctx, `{"actionStatus":"SUCCESS"}`)

	s.Equal("trace-123", s.requests[0].header.Get("X-Correlation-ID"))
}

func (s *ExtensionExecutorTestSuite) TestExecute_KeepsACorrelationIDTheConnectionSets() {
	s.answer(http.StatusOK, `{"actionStatus":"SUCCESS"}`)
	s.connections.EXPECT().GetResolvedConnection(mock.Anything, testExtensionConnectionID).
		Return(&httpconnection.ResolvedHTTPConnection{URL: s.server.URL, Timeout: time.Second,
			Headers: http.Header{"X-Correlation-Id": {"fixed"}}}, nil)

	_, err := s.executor.Execute(s.nodeContext())

	s.Require().NoError(err)
	s.Equal("fixed", s.requests[0].header.Get("X-Correlation-ID"))
}

func (s *ExtensionExecutorTestSuite) TestExecute_TimeoutFromTheConnectionIsNotRetried() {
	s.serve(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})
	s.resolveTo(s.server.URL, 100*time.Millisecond)

	execResp, err := s.executor.Execute(s.nodeContext())

	s.Require().NoError(err)
	s.assertCallFailed(execResp)
	s.Len(s.requests, 1)
	s.Equal(string(extensionErrorKindTimeout), s.onlyEvent().Data[event.DataKey.ErrorKind])
}

func (s *ExtensionExecutorTestSuite) TestExecute_NoConnectionFailsWithConfigErrorBeforeAnyCall() {
	s.answer(http.StatusOK, `{"actionStatus":"SUCCESS"}`)
	for name, properties := range map[string]map[string]interface{}{
		"absent":     {},
		"empty":      {common.NodePropertyConnectionID: " "},
		"not string": {common.NodePropertyConnectionID: 42},
	} {
		s.Run(name, func() {
			ctx := s.nodeContext()
			ctx.NodeProperties = properties

			execResp, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.assertConfigInvalidWithoutCall(execResp)
		})
	}
	s.connections.AssertNotCalled(s.T(), "GetResolvedConnection", mock.Anything, mock.Anything)
}

func (s *ExtensionExecutorTestSuite) TestExecute_DeletedConnectionFailsWithConfigErrorBeforeAnyCall() {
	s.answer(http.StatusOK, `{"actionStatus":"SUCCESS"}`)
	s.connections.EXPECT().GetResolvedConnection(mock.Anything, testExtensionConnectionID).
		Return(nil, &httpconnection.ErrorNotFound)

	execResp, err := s.executor.Execute(s.nodeContext())

	s.Require().NoError(err)
	s.assertConfigInvalidWithoutCall(execResp)
}

func (s *ExtensionExecutorTestSuite) TestExecute_UnresolvableConnectionIsAnInternalError() {
	s.answer(http.StatusOK, `{"actionStatus":"SUCCESS"}`)
	s.connections.EXPECT().GetResolvedConnection(mock.Anything, testExtensionConnectionID).
		Return(nil, &tidcommon.InternalServerError)

	execResp, err := s.executor.Execute(s.nodeContext())

	s.Require().NoError(err)
	s.assertCallFailed(execResp)
	s.Empty(s.requests, "no request may be sent")
	evt := s.onlyEvent()
	s.Equal(string(extensionOutcomeError), evt.Data[event.DataKey.Outcome])
	s.Equal(string(extensionErrorKindInternal), evt.Data[event.DataKey.ErrorKind])
}

func (s *ExtensionExecutorTestSuite) TestExecute_WithoutConnectionsFailsWithConfigError() {
	executor := newExtensionExecutor(s.flowFactory(), nil, nil)

	execResp, err := executor.Execute(s.nodeContext())

	s.Require().NoError(err)
	s.assertConfigInvalidWithoutCall(execResp)
}

// R4

func (s *ExtensionExecutorTestSuite) TestExecute_SuccessCompletes() {
	execResp := s.run(s.nodeContext(), `{"actionStatus":"SUCCESS"}`)

	s.Equal(providers.ExecComplete, execResp.Status)
	s.Nil(execResp.Error)
	s.False(execResp.AuthUser.IsAuthenticated(), "an extension never sets the authenticated user")
}

func (s *ExtensionExecutorTestSuite) TestExecute_FailedFailsWithDescriptionAndReason() {
	execResp := s.run(s.nodeContext(),
		`{"actionStatus":"FAILED","failureReason":"high_risk","failureDescription":"Sign-in blocked."}`)

	s.Equal(providers.ExecFailure, execResp.Status)
	s.Require().NotNil(execResp.Error)
	s.Equal(ErrExtensionActionFailed.Code, execResp.Error.Code)
	s.Equal("Request rejected", execResp.Error.Error.DefaultValue)
	s.Equal("Sign-in blocked.", execResp.Error.ErrorDescription.String())
	s.Equal(map[string]string{"description": "Sign-in blocked.", "reason": "high_risk"},
		execResp.Error.ErrorDescription.Params)
	s.Equal(map[string]string{"failureReason": "high_risk", "failureDescription": "Sign-in blocked.",
		common.ExtensionAttemptCountKey("risk_check"): "1"}, execResp.RuntimeData)
}

func (s *ExtensionExecutorTestSuite) TestExecute_IncompleteRequestsInputWithDescription() {
	execResp := s.run(s.nodeContext(), `{"actionStatus":"INCOMPLETE","failureReason":"otp_required",`+
		`"failureDescription":"Enter the code we sent you.","challengeId":"c-1"}`)

	s.Equal(providers.ExecUserInputRequired, execResp.Status)
	s.Require().NotNil(execResp.Error)
	s.Equal(ErrExtensionInputRequired.Code, execResp.Error.Code)
	s.Equal("Additional input required", execResp.Error.Error.DefaultValue)
	s.Equal(map[string]string{"description": "Enter the code we sent you.", "reason": "otp_required"},
		execResp.Error.ErrorDescription.Params)
	s.Equal(map[string]string{
		"extension.risk_check.challengeId":            "c-1",
		"failureReason":                               "otp_required",
		"failureDescription":                          "Enter the code we sent you.",
		common.ExtensionAttemptCountKey("risk_check"): "1",
	}, execResp.RuntimeData)
}

func (s *ExtensionExecutorTestSuite) TestExecute_IncompleteWithoutDescriptionReturnsNoError() {
	execResp := s.run(s.nodeContext(), `{"actionStatus":"INCOMPLETE","redirectUrl":null}`)

	s.Equal(providers.ExecUserInputRequired, execResp.Status)
	s.Nil(execResp.Error)
}

func (s *ExtensionExecutorTestSuite) TestExecute_AnswersOutsideTheContractAreErrors() {
	const marker = "BODY-TEXT-MARKER"
	cases := []struct {
		name   string
		status int
		body   string
		kind   extensionErrorKind
	}{
		{"service ERROR", 200,
			`{"actionStatus":"ERROR","failureReason":"x","failureDescription":"` + marker + `"}`, ""},
		{"missing actionStatus", 200, `{"note":"` + marker + `"}`, extensionErrorKindContract},
		{"non-string actionStatus", 200, `{"actionStatus":1,"note":"` + marker + `"}`, extensionErrorKindContract},
		{"unknown actionStatus", 200, `{"actionStatus":"OK","note":"` + marker + `"}`, extensionErrorKindContract},
		{"lower-case actionStatus", 200, `{"actionStatus":"success"}`, extensionErrorKindContract},
		{"executor-only outcome", 200, `{"actionStatus":"CONFIG_ERROR"}`, extensionErrorKindContract},
		{"server error", 500, `{"actionStatus":"SUCCESS","note":"` + marker + `"}`, extensionErrorKindStatus},
		{"client error", 400, `{"actionStatus":"SUCCESS"}`, extensionErrorKindStatus},
		{"empty body", 200, ``, extensionErrorKindBody},
		{"blank body", 200, "  \n", extensionErrorKindBody},
		{"array body", 200, `["` + marker + `"]`, extensionErrorKindBody},
		{"string body", 200, `"` + marker + `"`, extensionErrorKindBody},
		{"null body", 200, `null`, extensionErrorKindBody},
		{"malformed body", 200, `{"actionStatus":"SUCCESS"`, extensionErrorKindBody},
		{"trailing data", 200, `{"actionStatus":"SUCCESS"} {"x":1}`, extensionErrorKindBody},
		{"over the cap", 200, `{"actionStatus":"SUCCESS","pad":"` + strings.Repeat("a", 65536) + `"}`,
			extensionErrorKindBody},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			defer s.TearDownTest()
			s.answer(tc.status, tc.body)
			s.resolveTo(s.server.URL, time.Second)

			execResp, err := s.executor.Execute(s.nodeContext())

			s.Require().NoError(err)
			s.assertCallFailed(execResp)
			encoded, err := json.Marshal(execResp.Error)
			s.Require().NoError(err)
			s.NotContains(string(encoded), marker)
			evt := s.onlyEvent()
			s.Equal(string(extensionOutcomeError), evt.Data[event.DataKey.Outcome])
			if tc.kind == "" {
				s.NotContains(evt.Data, event.DataKey.ErrorKind)
			} else {
				s.Equal(string(tc.kind), evt.Data[event.DataKey.ErrorKind])
			}
		})
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_RedirectIsNeverFollowed() {
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetCalls++
		_, _ = w.Write([]byte(`{"actionStatus":"SUCCESS"}`))
	}))
	defer target.Close()
	s.serve(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	})
	s.resolveTo(s.server.URL, time.Second)

	execResp, err := s.executor.Execute(s.nodeContext())

	s.Require().NoError(err)
	s.assertCallFailed(execResp)
	s.Zero(targetCalls, "a redirect target is never called")
	s.Equal(string(extensionErrorKindStatus), s.onlyEvent().Data[event.DataKey.ErrorKind])
	s.Equal(http.StatusTemporaryRedirect, s.onlyEvent().Data[event.DataKey.HTTPStatus])
}

func (s *ExtensionExecutorTestSuite) TestExecute_UnreachableServiceIsATransportError() {
	s.answer(http.StatusOK, `{"actionStatus":"SUCCESS"}`)
	url := s.server.URL
	s.server.Close()
	s.server = nil
	s.resolveTo(url, time.Second)

	execResp, err := s.executor.Execute(s.nodeContext())

	s.Require().NoError(err)
	s.assertCallFailed(execResp)
	evt := s.onlyEvent()
	s.Equal(string(extensionErrorKindTransport), evt.Data[event.DataKey.ErrorKind])
	s.NotContains(evt.Data, event.DataKey.HTTPStatus)
}

func (s *ExtensionExecutorTestSuite) TestExecute_BodyOfExactlyTheCapIsRead() {
	prefix, suffix := `{"actionStatus":"SUCCESS","pad":"`, `"}`
	body := prefix + strings.Repeat("a", extensionMaxResponseBytes-len(prefix)-len(suffix)) + suffix
	s.Require().Len(body, 65536)

	execResp := s.run(s.nodeContext(), body)

	s.Equal(providers.ExecComplete, execResp.Status)
}

func (s *ExtensionExecutorTestSuite) TestExecute_IncompleteWithARedirectURLIsAnErrorInThisBuild() {
	for _, redirectURL := range []string{`"https://pin.example.com/entry"`, `"javascript:alert(1)"`,
		`"data:text/html,x"`, `"/relative"`, `"//pin.example.com"`, `42`} {
		s.Run(redirectURL, func() {
			s.SetupTest()
			defer s.TearDownTest()

			execResp := s.run(s.nodeContext(), `{"actionStatus":"INCOMPLETE","redirectUrl":`+redirectURL+`}`)

			s.assertCallFailed(execResp)
			s.NotEqual(providers.ExecExternalRedirection, execResp.Status)
			s.Empty(execResp.RedirectURL)
		})
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_RedirectURLIsNeverStored() {
	execResp := s.run(s.nodeContext(), `{"actionStatus":"SUCCESS","redirectUrl":"https://x.example.com","risk":"1"}`)

	s.Equal(providers.ExecComplete, execResp.Status)
	s.Equal(map[string]string{"extension.risk_check.risk": "1"}, execResp.RuntimeData)
}

func (s *ExtensionExecutorTestSuite) TestExecute_StopsWithoutACallOnceMaxAttemptsAreUsed() {
	s.answerInTurn(`{"actionStatus":"FAILED","failureReason":"wrong_pin"}`,
		`{"actionStatus":"INCOMPLETE","failureDescription":"Try again."}`, `{"actionStatus":"FAILED"}`)
	ctx := s.nodeContext()
	ctx.NodeProperties[common.NodePropertyMaxAttempts] = float64(3)
	for range 3 {
		s.arrive(ctx)
	}
	s.published = nil

	execResp := s.arrive(ctx)

	s.Len(s.requests, 3, "the service is not called once the limit is reached")
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Require().NotNil(execResp.Error)
	s.Equal(ErrExtensionAttemptsExceeded, *execResp.Error)
	s.Empty(execResp.RuntimeData)
	evt := s.onlyEvent()
	s.Equal(string(extensionOutcomeAttemptsExceeded), evt.Data[event.DataKey.Outcome])
	s.Equal(testExtensionConnectionID, evt.Data[event.DataKey.ConnectionID])
	s.NotContains(evt.Data, event.DataKey.HTTPStatus)
}

func (s *ExtensionExecutorTestSuite) TestExecute_DefaultLimitIsFiveFailedAnswers() {
	s.answerInTurn(`{"actionStatus":"FAILED"}`)
	ctx := s.nodeContext()
	for range 5 {
		s.Equal(ErrExtensionActionFailed.Code, s.arrive(ctx).Error.Code)
	}

	execResp := s.arrive(ctx)

	s.Len(s.requests, 5)
	s.Equal(ErrExtensionAttemptsExceeded.Code, execResp.Error.Code)
}

func (s *ExtensionExecutorTestSuite) TestExecute_SuccessAndErrorAnswersAreNotCounted() {
	s.answerInTurn(`{"actionStatus":"SUCCESS"}`, `{"actionStatus":"ERROR"}`, `{"note":"outside the contract"}`,
		`{"actionStatus":"FAILED"}`, `{"actionStatus":"SUCCESS"}`)
	ctx := s.nodeContext()
	ctx.NodeProperties[common.NodePropertyMaxAttempts] = float64(1)
	for range 4 {
		s.arrive(ctx)
	}
	s.Equal("1", ctx.RuntimeData[common.ExtensionAttemptCountKey("risk_check")])

	execResp := s.arrive(ctx)

	s.Len(s.requests, 4)
	s.Equal(ErrExtensionAttemptsExceeded.Code, execResp.Error.Code)
}

func (s *ExtensionExecutorTestSuite) TestExecute_EachStepCountsItsOwnAttempts() {
	s.answerInTurn(`{"actionStatus":"FAILED"}`)
	ctx := s.nodeContext()
	ctx.NodeProperties[common.NodePropertyMaxAttempts] = float64(1)
	s.arrive(ctx)
	ctx.CurrentNodeID = "pin_check"

	execResp := s.arrive(ctx)

	s.Equal(ErrExtensionActionFailed.Code, execResp.Error.Code, "another step's answers do not count")
	s.Len(s.requests, 2)
}

func (s *ExtensionExecutorTestSuite) TestExecute_AttemptCountIsNeverSent() {
	s.answerInTurn(`{"actionStatus":"FAILED"}`)
	ctx := s.nodeContext()
	key := common.ExtensionAttemptCountKey("risk_check")
	ctx.RuntimeData[key] = "2"
	ctx.NodeProperties[common.NodePropertyRuntimeDataKeys] = []interface{}{key, "riskTier"}

	s.arrive(ctx)

	s.JSONEq(`{"riskTier":"gold"}`, string(s.sentBody()["runtimeData"]))
	s.Equal("3", ctx.RuntimeData[key])
}

func (s *ExtensionExecutorTestSuite) TestExecute_AnswerNamingTheAttemptCountIsAnError() {
	execResp := s.run(s.nodeContext(),
		`{"actionStatus":"SUCCESS","`+common.ExtensionAttemptCountKey("risk_check")+`":"0"}`)

	s.assertCallFailed(execResp)
	s.Equal(string(extensionErrorKindReservedKey), s.onlyEvent().Data[event.DataKey.ErrorKind])
}

// R5

func (s *ExtensionExecutorTestSuite) TestExecute_SendsNoUserInputsWithoutInputKeys() {
	s.run(s.nodeContext(), `{"actionStatus":"SUCCESS"}`)

	s.JSONEq(`{}`, string(s.sentBody()["userInputs"]))
	s.NotContains(string(s.requests[0].body), "alice-password")
}

func (s *ExtensionExecutorTestSuite) TestExecute_SendsOnlyNamedInputsThatArePresent() {
	ctx := s.nodeContext()
	ctx.NodeProperties[common.NodePropertyInputKeys] = []interface{}{"username", "password", "otp"}

	s.run(ctx, `{"actionStatus":"SUCCESS"}`)

	s.JSONEq(`{"username":"alice","password":"alice-password"}`, string(s.sentBody()["userInputs"]))
}

func (s *ExtensionExecutorTestSuite) TestExecute_UnusablePropertyValueFailsBeforeAnyCallAndStoresNothing() {
	for name, properties := range map[string]map[string]interface{}{
		"input keys as a string":        {common.NodePropertyInputKeys: "username,password"},
		"runtime data keys as a string": {common.NodePropertyRuntimeDataKeys: "riskTier"},
		"empty input keys":              {common.NodePropertyInputKeys: []interface{}{}},
		"blank runtime data key":        {common.NodePropertyRuntimeDataKeys: []interface{}{" "}},
		"empty response keys":           {common.NodePropertyResponseKeys: []interface{}{}},
		"non-string response key":       {common.NodePropertyResponseKeys: []interface{}{"riskScore", 1}},
		"blank response key in string":  {common.NodePropertyResponseKeys: "riskScore,"},
		"max attempts zero":             {common.NodePropertyMaxAttempts: float64(0)},
		"max attempts over the bound":   {common.NodePropertyMaxAttempts: float64(21)},
	} {
		s.Run(name, func() {
			s.SetupTest()
			defer s.TearDownTest()
			s.answer(http.StatusOK, `{"actionStatus":"SUCCESS","riskScore":"1"}`)
			ctx := s.nodeContext()
			ctx.RuntimeData["extension.risk_check.riskScore"] = "earlier"
			for key, value := range properties {
				ctx.NodeProperties[key] = value
			}

			execResp, err := s.executor.Execute(ctx)

			s.Require().NoError(err)
			s.assertConfigInvalidWithoutCall(execResp)
			s.Empty(execResp.RuntimeData, "nothing may be stored")
			s.Equal("earlier", ctx.RuntimeData["extension.risk_check.riskScore"])
			s.Equal(string(extensionOutcomeConfigError), s.onlyEvent().Data[event.DataKey.Outcome])
			s.connections.AssertNotCalled(s.T(), "GetResolvedConnection", mock.Anything, mock.Anything)
		})
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_NeverSendsSecretEngineState() {
	ctx := s.nodeContext()
	secrets := map[string]string{
		common.RuntimeKeyOTPSessionToken:                                  "otp-session",
		common.RuntimeKeyOAuthState:                                       "oauth-state",
		runtimePasskeySessionToken:                                        "passkey-session",
		common.SSOCheckpointKey(common.RuntimeKeySSOSessionSaved, "join"): "sso-handle",
		common.RuntimeKeyLinkingCheckpoint:                                "checkpoint",
		common.RuntimeKeyProvenSubject:                                    "proven-user",
		userAttributePassword:                                             "plain-password",
	}
	names := make([]interface{}, 0, len(secrets)+1)
	names = append(names, "riskTier")
	for key, value := range secrets {
		ctx.RuntimeData[key] = value
		names = append(names, key)
	}
	ctx.NodeProperties[common.NodePropertyRuntimeDataKeys] = names

	s.run(ctx, `{"actionStatus":"SUCCESS"}`)

	s.JSONEq(`{"riskTier":"gold"}`, string(s.sentBody()["runtimeData"]))
	for _, value := range secrets {
		s.NotContains(string(s.requests[0].body), value)
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_DefaultRuntimeDataIsApplicationIDAndUserIDOnly() {
	s.run(s.nodeContext(), `{"actionStatus":"SUCCESS"}`)

	s.JSONEq(`{"applicationId":"app-1","userID":"user-1"}`, string(s.sentBody()["runtimeData"]))
}

func (s *ExtensionExecutorTestSuite) TestExecute_RuntimeDataKeysRestrictsToNamedKeysThatArePresent() {
	ctx := s.nodeContext()
	ctx.NodeProperties[common.NodePropertyRuntimeDataKeys] = []interface{}{common.RuntimeKeyClientID, "riskScore"}

	s.run(ctx, `{"actionStatus":"SUCCESS"}`)

	s.JSONEq(`{"clientId":"client-1"}`, string(s.sentBody()["runtimeData"]))
}

func (s *ExtensionExecutorTestSuite) TestExecute_SendsFlowContextWithoutSecrets() {
	ctx := s.nodeContext()
	ctx.CurrentAction = "submit"
	ctx.Application = providers.Application{
		ID: "app-1", OUID: "ou-1", Name: "Sample App", Type: "browser",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			OAuthConfig: &providers.OAuthConfigWithSecret{ClientID: "client-1", ClientSecret: "client-secret-value"},
		}},
	}
	ctx.AuthUser.SetStateFor("default", providers.AuthState{
		EntityReference: &providers.EntityReference{
			EntityID: "user-1", EntityCategory: "user", EntityType: "customer", OUID: "ou-1"},
		EntityReferenceToken: "entity-reference-token-value",
		AttributeToken:       "attribute-token-value",
		Attributes: &providers.AttributesResponse{
			Attributes: map[string]*providers.AttributeResponse{"email": {Value: "alice@example.com"}}},
	})

	s.run(ctx, `{"actionStatus":"SUCCESS"}`)

	s.JSONEq(`{
		"version": "1",
		"executionId": "019a1b2c-3d4e-7f60-8a9b-0c1d2e3f4a5b",
		"flowType": "AUTHENTICATION",
		"currentAction": "submit",
		"currentNodeId": "risk_check",
		"userInputs": {},
		"runtimeData": {"applicationId": "app-1", "userID": "user-1"},
		"application": {"id": "app-1", "ouId": "ou-1", "name": "Sample App", "type": "browser"},
		"authUser": {"default": {"entityReference": {"entityId": "user-1", "entityCategory": "user",
			"entityType": "customer", "ouId": "ou-1"}}}
	}`, string(s.requests[0].body))
	for _, secret := range []string{"client-secret-value", "entity-reference-token-value", "attribute-token-value",
		"alice@example.com"} {
		s.NotContains(string(s.requests[0].body), secret)
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_SendsEmptyAuthUserBeforeAuthentication() {
	ctx := s.nodeContext()
	delete(ctx.RuntimeData, userAttributeUserID)

	s.run(ctx, `{"actionStatus":"SUCCESS"}`)

	body := s.sentBody()
	s.JSONEq(`{}`, string(body["authUser"]))
	s.JSONEq(`{"applicationId":"app-1"}`, string(body["runtimeData"]))
	s.NotContains(body, "currentAction")
}

// R6

func (s *ExtensionExecutorTestSuite) TestExecute_StoresTextNumbersBooleansAndJSON() {
	execResp := s.run(s.nodeContext(), `{"actionStatus":"SUCCESS","risk":"low","score":12.50,`+
		`"big":12345678901234567890,`+
		`"trusted":true,"claims":{"tier":"gold","levels":[1, 2]},"skipped":null}`)

	s.Equal(map[string]string{
		"extension.risk_check.risk": "low", "extension.risk_check.score": "12.50",
		"extension.risk_check.big": "12345678901234567890", "extension.risk_check.trusted": "true",
		"extension.risk_check.claims": `{"levels":[1,2],"tier":"gold"}`,
	}, execResp.RuntimeData)
}

func (s *ExtensionExecutorTestSuite) TestExecute_ResponseKeysAllowlist() {
	for name, responseKeys := range map[string]interface{}{
		"array":        []interface{}{"username"},
		"comma string": "username",
	} {
		s.Run(name, func() {
			s.SetupTest()
			defer s.TearDownTest()
			ctx := s.nodeContext()
			ctx.NodeProperties[common.NodePropertyResponseKeys] = responseKeys

			execResp := s.run(ctx, `{"actionStatus":"SUCCESS","username":"jdoe","accessToken":"upstream-token"}`)

			s.Equal(map[string]string{"extension.risk_check.username": "jdoe"}, execResp.RuntimeData)
		})
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_ReservedKeyTheStepWouldStoreIsError() {
	for _, key := range []string{userAttributeUserID, userAttributeGroups, userAttributePassword,
		common.RuntimeKeyUserIDCamelCase,
		common.RuntimeKeyMappedRoleIDs,
		ouIDKey, common.RuntimeKeyApplicationID, runtimePasskeySessionToken, common.RuntimeKeyProvenSubject,
		common.SSOCheckpointKey(common.RuntimeKeySSOSessionPresent, "join")} {
		for _, status := range []string{"SUCCESS", "INCOMPLETE"} {
			allowlists := map[string]interface{}{"no allowlist": nil, "allowlist names it": "safe," + key}
			for name, responseKeys := range allowlists {
				s.Run(key+"/"+status+"/"+name, func() {
					s.SetupTest()
					defer s.TearDownTest()
					ctx := s.nodeContext()
					if responseKeys != nil {
						ctx.NodeProperties[common.NodePropertyResponseKeys] = responseKeys
					}

					execResp := s.run(ctx, `{"actionStatus":"`+status+`","safe":"1","`+key+`":"x"}`)

					s.assertCallFailed(execResp)
					s.Equal(string(extensionErrorKindReservedKey), s.onlyEvent().Data[event.DataKey.ErrorKind])
				})
			}
		}
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_ReservedKeyTheStepDoesNotStoreIsIgnored() {
	for _, status := range []string{"SUCCESS", "INCOMPLETE"} {
		s.Run(status, func() {
			s.SetupTest()
			defer s.TearDownTest()
			ctx := s.nodeContext()
			ctx.NodeProperties[common.NodePropertyResponseKeys] = "safe"

			execResp := s.run(ctx, `{"actionStatus":"`+status+`","safe":"1","userId":"u-9","password":"p"}`)

			s.NotEqual(providers.ExecFailure, execResp.Status)
			s.Equal("1", execResp.RuntimeData["extension.risk_check.safe"])
			for key := range execResp.RuntimeData {
				s.NotContains(key, "userId")
				s.NotContains(key, "password")
			}
		})
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_ReservedKeyInAFailedAnswerIsIgnored() {
	execResp := s.run(s.nodeContext(), `{"actionStatus":"FAILED","failureReason":"high_risk","userId":"u-9"}`)

	s.Equal(providers.ExecFailure, execResp.Status)
	s.Equal(ErrExtensionActionFailed.Code, execResp.Error.Code)
	s.Equal("high_risk", execResp.RuntimeData[extensionFieldFailureReason])
	for key := range execResp.RuntimeData {
		s.NotContains(key, "userId")
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_SuccessWithGroupsIsAnErrorAndStoresNothing() {
	execResp := s.run(s.nodeContext(), `{"actionStatus":"SUCCESS","riskTier":"gold","groups":"[\"admins\"]"}`)

	s.assertCallFailed(execResp)
	s.Equal(string(extensionErrorKindReservedKey), s.onlyEvent().Data[event.DataKey.ErrorKind])
}

func (s *ExtensionExecutorTestSuite) TestExecute_ProfileAttributesAndServiceDataAreStorable() {
	execResp := s.run(s.nodeContext(), `{"actionStatus":"SUCCESS","username":"jdoe","email":"j@example.com",`+
		`"given_name":"J","family_name":"Doe","riskTier":"gold","score":1,"enabled":true,"claims":{},`+
		`"tags":[],"serviceRef":"r","accessToken":"t","riskScore":"2","challengeId":"c"}`)

	s.Equal(providers.ExecComplete, execResp.Status)
	s.Len(execResp.RuntimeData, 13)
	for key := range execResp.RuntimeData {
		s.True(strings.HasPrefix(key, "extension.risk_check."), key)
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_FailureFieldsAreNotReserved() {
	ctx := s.nodeContext()
	ctx.RuntimeData["failureReason"] = "high_risk"

	execResp := s.run(ctx, `{"actionStatus":"FAILED","failureReason":"high_risk"}`)

	s.Equal(ErrExtensionActionFailed.Code, execResp.Error.Code)
}

func (s *ExtensionExecutorTestSuite) TestExecute_SuccessResetsEarlierFailureFields() {
	ctx := s.nodeContext()
	failed := s.run(ctx, `{"actionStatus":"FAILED","failureReason":"high_risk","failureDescription":"No."}`)
	for key, value := range failed.RuntimeData {
		ctx.RuntimeData[key] = value
	}
	s.TearDownTest()
	s.SetupTest()

	succeeded := s.run(ctx, `{"actionStatus":"SUCCESS"}`)

	s.Equal(map[string]string{"failureReason": "", "failureDescription": ""}, succeeded.RuntimeData)
}

func (s *ExtensionExecutorTestSuite) TestExecute_EachCallReplacesTheStepNamespace() {
	s.answerInTurn(`{"actionStatus":"INCOMPLETE","failureDescription":"Enter the code.","challengeId":"c-1",`+
		`"riskScore":"40"}`, `{"actionStatus":"SUCCESS","riskScore":"12"}`)
	ctx := s.nodeContext()
	ctx.RuntimeData["extension.pin_check.verifiedUserId"] = "u-9"
	ctx.NodeProperties[common.NodePropertyRuntimeDataKeys] = []interface{}{"extension.risk_check.challengeId"}
	s.arrive(ctx)
	s.Equal("c-1", ctx.RuntimeData["extension.risk_check.challengeId"])

	s.arrive(ctx)

	s.Require().Len(s.requests, 2)
	var second map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(s.requests[1].body, &second))
	s.JSONEq(`{"extension.risk_check.challengeId":"c-1"}`, string(second["runtimeData"]),
		"the request is built before the namespace is replaced")
	s.NotContains(ctx.RuntimeData, "extension.risk_check.challengeId")
	s.Equal("12", ctx.RuntimeData["extension.risk_check.riskScore"])
	s.Equal("u-9", ctx.RuntimeData["extension.pin_check.verifiedUserId"], "another step's namespace is kept")
}

func (s *ExtensionExecutorTestSuite) TestExecute_ErrorAnswerClearsTheStepNamespace() {
	ctx := s.nodeContext()
	ctx.RuntimeData["extension.risk_check.riskScore"] = "12"

	execResp := s.run(ctx, `{"actionStatus":"ERROR"}`)

	s.assertCallFailed(execResp)
	s.NotContains(ctx.RuntimeData, "extension.risk_check.riskScore")
}

func (s *ExtensionExecutorTestSuite) TestExecute_NoAnswerKeepsTheStepNamespace() {
	s.answer(http.StatusOK, `{"actionStatus":"SUCCESS"}`)
	url := s.server.URL
	s.server.Close()
	s.server = nil
	s.resolveTo(url, time.Second)
	ctx := s.nodeContext()
	ctx.RuntimeData["extension.risk_check.riskScore"] = "12"

	execResp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.assertCallFailed(execResp)
	s.Equal("12", ctx.RuntimeData["extension.risk_check.riskScore"])
}

// R7

func (s *ExtensionExecutorTestSuite) TestExecute_FailureDescriptionIsCleanedAndCut() {
	description := "Blocked\u0000 by\t\tpolicy\r\n\u0007" + strings.Repeat("é", 300)
	encoded, err := json.Marshal(description)
	s.Require().NoError(err)

	execResp := s.run(s.nodeContext(), `{"actionStatus":"FAILED","failureDescription":`+string(encoded)+`}`)

	shown := execResp.Error.ErrorDescription.Params["description"]
	s.LessOrEqual(len([]rune(shown)), 256)
	s.True(strings.HasPrefix(shown, "Blocked by policy "), shown)
	for _, r := range shown {
		s.False(r < 0x20 || r == 0x7f, "control character %q left in the description", r)
	}
	s.Equal(shown, execResp.Error.ErrorDescription.String())
	s.Equal(shown, execResp.RuntimeData["failureDescription"])
}

func (s *ExtensionExecutorTestSuite) TestExecute_FailureDescriptionDropsFormatCharacters() {
	execResp := s.run(s.nodeContext(),
		`{"actionStatus":"FAILED","failureDescription":"Call \u202eegnahc\u202c support\u200b now\ufeff"}`)

	shown := execResp.Error.ErrorDescription.Params["description"]
	s.Equal("Call egnahc support now", shown)
	for _, r := range shown {
		s.False(unicode.Is(unicode.Cf, r), "format character %U left in the description", r)
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_CredentialFailureReasonsReturnTheNativeCredentialError() {
	for _, reason := range []string{"invalid_credentials", "user_not_found"} {
		for _, status := range []string{"FAILED", "INCOMPLETE"} {
			s.Run(reason+"/"+status, func() {
				s.SetupTest()
				defer s.TearDownTest()

				execResp := s.run(s.nodeContext(), `{"actionStatus":"`+status+`","failureReason":"`+reason+
					`","failureDescription":"No account for alice@example.com"}`)

				s.Require().NotNil(execResp.Error)
				s.Equal(ErrInvalidCredentials, *execResp.Error, "the same error the built-in steps return")
				s.Equal(reason, execResp.RuntimeData["failureReason"])
				encoded, err := json.Marshal(execResp)
				s.Require().NoError(err)
				s.NotContains(string(encoded), "alice@example.com")
				s.Equal(reason, s.onlyEvent().Data[event.DataKey.FailureReason])
			})
		}
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_FailedWithoutDescriptionUsesDefault() {
	execResp := s.run(s.nodeContext(), `{"actionStatus":"FAILED","failureDescription":" \n "}`)

	s.Equal("The request was rejected", execResp.Error.ErrorDescription.String())
	s.Equal(map[string]string{"description": "The request was rejected"}, execResp.Error.ErrorDescription.Params)
}

func (s *ExtensionExecutorTestSuite) TestExecute_InvalidFailureReasonIsDropped() {
	for _, reason := range []string{`"high risk"`, `"<script>"`, `"` + strings.Repeat("a", 65) + `"`, `7`} {
		s.Run(reason, func() {
			s.SetupTest()
			defer s.TearDownTest()

			execResp := s.run(s.nodeContext(), `{"actionStatus":"FAILED","failureReason":`+reason+`}`)

			s.NotContains(execResp.Error.ErrorDescription.Params, "reason")
			s.Empty(execResp.RuntimeData["failureReason"])
			s.NotContains(s.onlyEvent().Data, event.DataKey.FailureReason)
		})
	}
	s.Run("64 characters is kept", func() {
		s.SetupTest()
		defer s.TearDownTest()
		reason := strings.Repeat("a", 64)

		execResp := s.run(s.nodeContext(), `{"actionStatus":"FAILED","failureReason":"`+reason+`"}`)

		s.Equal(reason, execResp.Error.ErrorDescription.Params["reason"])
	})
}

// R8

func (s *ExtensionExecutorTestSuite) TestExecute_PublishesOneAuditEventPerExecution() {
	s.run(s.nodeContext(), `{"actionStatus":"FAILED","failureReason":"high_risk"}`)

	evt := s.onlyEvent()
	s.Equal(string(event.EventTypeExtensionCall), evt.Type)
	category, err := event.GetCategory(event.EventTypeExtensionCall)
	s.Require().NoError(err)
	s.Equal(event.CategoryFlows, category)
	s.Equal(providers.StatusFailure, evt.Status)
	s.Equal("019a1b2c-3d4e-7f60-8a9b-0c1d2e3f4a5b", evt.Data[event.DataKey.ExecutionID])
	s.Equal("risk_check", evt.Data[event.DataKey.NodeID])
	s.Equal(string(providers.FlowTypeAuthentication), evt.Data[event.DataKey.FlowType])
	s.Equal(testExtensionConnectionID, evt.Data[event.DataKey.ConnectionID])
	s.Equal(string(extensionOutcomeFailed), evt.Data[event.DataKey.Outcome])
	s.Equal("high_risk", evt.Data[event.DataKey.FailureReason])
	s.Equal(http.StatusOK, evt.Data[event.DataKey.HTTPStatus])
	s.Contains(evt.Data, event.DataKey.DurationMs)
	s.NotContains(evt.Data, event.DataKey.ErrorKind)
}

func (s *ExtensionExecutorTestSuite) TestExecute_ConfigErrorPublishesAConfigErrorEvent() {
	ctx := s.nodeContext()
	ctx.NodeProperties = map[string]interface{}{}

	_, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	evt := s.onlyEvent()
	s.Equal(string(extensionOutcomeConfigError), evt.Data[event.DataKey.Outcome])
	s.NotContains(evt.Data, event.DataKey.HTTPStatus)
}

func (s *ExtensionExecutorTestSuite) TestExecute_AuditEventCarriesNoSensitiveValues() {
	s.answer(http.StatusOK, `{"actionStatus":"INCOMPLETE","failureReason":"otp_required",`+
		`"failureDescription":"DESCRIPTION-MARKER","bodyField":"RESPONSE-MARKER"}`)
	s.connections.EXPECT().GetResolvedConnection(mock.Anything, testExtensionConnectionID).
		Return(&httpconnection.ResolvedHTTPConnection{URL: s.server.URL + "/URL-MARKER?q=QUERY-MARKER",
			Timeout: time.Second, Headers: http.Header{"X-Api-Key": {"HEADER-MARKER"}}}, nil)
	ctx := s.nodeContext()
	ctx.UserInputs["password"] = "INPUT-MARKER"
	ctx.RuntimeData["riskTier"] = "RUNTIME-MARKER"
	ctx.NodeProperties[common.NodePropertyInputKeys] = []interface{}{"password"}
	ctx.NodeProperties[common.NodePropertyRuntimeDataKeys] = []interface{}{"riskTier"}

	_, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	encoded, err := json.Marshal(s.onlyEvent())
	s.Require().NoError(err)
	for _, marker := range []string{"DESCRIPTION-MARKER", "RESPONSE-MARKER", "URL-MARKER", "QUERY-MARKER",
		"HEADER-MARKER", "INPUT-MARKER", "RUNTIME-MARKER", s.server.URL, testExtensionPassword} {
		s.NotContains(string(encoded), marker)
	}
}

func (s *ExtensionExecutorTestSuite) TestExecute_NoEventWhenObservabilityIsDisabled() {
	events := observabilityprovidermock.NewObservabilityProviderMock(s.T())
	events.EXPECT().IsEnabled().Return(false)
	executor := newExtensionExecutor(s.flowFactory(), s.connections, events)
	ctx := s.nodeContext()
	ctx.NodeProperties = map[string]interface{}{}

	_, err := executor.Execute(ctx)

	s.Require().NoError(err)
	events.AssertNotCalled(s.T(), "PublishEvent", mock.Anything, mock.Anything)
}

// extensionStorableRuntimeKeys are the executor constants that name, or look like they name,
// runtime-data keys but that an extension may return. None of them decides who the user is or what
// they may do. Every other such constant must be reserved.
var extensionStorableRuntimeKeys = map[string]bool{
	"userAttributeUsername": true, "userAttributeEmail": true,
	// userAttributeSub is a claim name inside externalIdentity, never a runtime-data key.
	"userAttributeSub": true,
	// User inputs, read from user inputs only.
	"userTypeKey": true, "agentTypeKey": true, "ownerKey": true,
	// Collected agent attributes.
	"descriptionKey": true, "logoURLKey": true, "delegatedKey": true, "redirectURIsKey": true,
	// Additional data sent to the client.
	"runtimePasskeyChallenge": true, "runtimePasskeyCreationOptions": true,
	"extensionFieldFailureReason": true, "extensionFieldFailureDescription": true,
}

// TestReservedKeysCoverEveryRuntimeKeyConstant fails when a runtime-data key constant of the flow
// common package or of this package is not classified, so the reserved list cannot fall behind.
func (s *ExtensionExecutorTestSuite) TestReservedKeysCoverEveryRuntimeKeyConstant() {
	pkg, err := importer.ForCompiler(token.NewFileSet(), "source", nil).
		Import("github.com/thunder-id/thunderid/internal/flow/common")
	s.Require().NoError(err)
	for _, name := range pkg.Scope().Names() {
		if c, ok := pkg.Scope().Lookup(name).(*types.Const); ok && strings.HasPrefix(name, "RuntimeKey") &&
			c.Val().Kind() == constant.String {
			s.True(isReservedRuntimeKey(constant.StringVal(c.Val())), "common.%s is not classified", name)
		}
	}

	constants := s.executorRuntimeKeyConstants()
	s.Require().Contains(constants, "userAttributeGroups")
	for name, value := range constants {
		s.NotEqual(extensionStorableRuntimeKeys[name], isReservedRuntimeKey(value),
			"%s (%q) must be either reserved or listed as storable", name, value)
	}
}

// executorRuntimeKeyConstants returns the untyped string constants of this package that are used as
// a RuntimeData index or named like a runtime-data key. Typed constants are enumerations, not keys.
func (s *ExtensionExecutorTestSuite) executorRuntimeKeyConstants() map[string]string {
	files, err := filepath.Glob("*.go")
	s.Require().NoError(err)
	values, indexed := make(map[string]string), make(map[string]bool)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		s.Require().NoError(err)
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.GenDecl:
				for _, spec := range n.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if n.Tok != token.CONST || !ok || valueSpec.Type != nil {
						continue
					}
					for i, name := range valueSpec.Names {
						if i < len(valueSpec.Values) {
							if lit, ok := valueSpec.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								values[name.Name], _ = strconv.Unquote(lit.Value)
							}
						}
					}
				}
			case *ast.IndexExpr:
				if selector, ok := n.X.(*ast.SelectorExpr); ok && selector.Sel.Name == "RuntimeData" {
					if ident, ok := n.Index.(*ast.Ident); ok {
						indexed[ident.Name] = true
					}
				}
			}
			return true
		})
	}
	constants := make(map[string]string)
	for name, value := range values {
		if indexed[name] || strings.HasPrefix(name, "runtime") || strings.HasPrefix(name, "userAttribute") ||
			strings.HasSuffix(name, "Key") {
			constants[name] = value
		}
	}
	return constants
}
