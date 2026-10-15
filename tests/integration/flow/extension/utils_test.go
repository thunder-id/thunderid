// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL = "https://localhost:8095"

	extensionExecutorName = "ExtensionExecutor"

	// Error codes the extension executor returns. The spec names them by constant only; the numbers
	// are the ones the backend assigns from the next free FET range.
	errCodeExtensionConfigInvalid = "FET-1112"
	errCodeExtensionCallFailed    = "FET-1113"
	errCodeExtensionActionFailed  = "FET-1114"
	errCodeExtensionInputRequired = "FET-1115"

	// Messages the spec fixes for each error.
	msgExtensionConfigInvalid = "Configuration error"
	msgExtensionCallFailed    = "Extension call failed"
	msgExtensionActionFailed  = "Request rejected"
	msgExtensionInputRequired = "Additional input required"
	descDefaultRejection      = "The request was rejected"

	// The native credential failure, which an extension's credential failure is reported as.
	errCodeInvalidCredentials  = "FET-1005"
	keyInvalidCredentials      = "flows.executor.errors.invalid_credentials"
	msgInvalidCredentials      = "Invalid credentials provided"
	keyInvalidCredentialsDesc  = "flows.executor.errors.invalid_credentials_desc"
	descInvalidCredentials     = "The credentials provided are invalid"
	errCodeInternalServerError = "SSE-5000"
	msgInternalServerError     = "Internal server error"

	// HTTP endpoint connection errors.
	errCodeConnectionNotFound    = "CON-1013"
	errCodeConnectionInUse       = "CON-1021"
	errCodeFlowInvalidExecConfig = "FLM-1023"

	// servicePath and serviceQuery make up the connection URL. The query proves the call goes to the
	// connection's whole URL and not to a rebuilt one.
	servicePath  = "/extension/evaluate"
	serviceQuery = "tenant=acme"

	// seedPath and probePath are called by HTTP request executor nodes, not by extensions. The first
	// seeds engine state into runtime data, the second proves it was there.
	seedPath  = "/seed"
	probePath = "/probe"

	successBody = `{"actionStatus":"SUCCESS"}`
)

// credentialInputs are the inputs of the username and password prompt.
var credentialInputs = []map[string]interface{}{
	{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
	{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
}

const credentialsAction = "action_credentials"

func startNode(next string) map[string]interface{} {
	return map[string]interface{}{"id": "start", "type": "START", "onSuccess": next}
}

func endNode() map[string]interface{} {
	return map[string]interface{}{"id": "end", "type": "END"}
}

func promptCredentialsNode(next string) map[string]interface{} {
	return map[string]interface{}{
		"id":   "prompt_credentials",
		"type": "PROMPT",
		"prompts": []map[string]interface{}{
			{
				"inputs": credentialInputs,
				"action": map[string]interface{}{"ref": credentialsAction, "nextNode": next},
			},
		},
	}
}

func credentialsAuthNode(onSuccess string) map[string]interface{} {
	return map[string]interface{}{
		"id":           "credentials_auth",
		"type":         "TASK_EXECUTION",
		"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
		"onSuccess":    onSuccess,
		"onIncomplete": "prompt_credentials",
	}
}

func authAssertNode() map[string]interface{} {
	return map[string]interface{}{
		"id":        "auth_assert",
		"type":      "TASK_EXECUTION",
		"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
		"onSuccess": "end",
	}
}

// extensionNode is an extension step. routes holds onSuccess, onFailure and onIncomplete as needed.
func extensionNode(id string, properties map[string]interface{}, routes map[string]string) map[string]interface{} {
	node := map[string]interface{}{
		"id":       id,
		"type":     "TASK_EXECUTION",
		"executor": map[string]interface{}{"name": extensionExecutorName},
	}
	if properties != nil {
		node["properties"] = properties
	}
	for route, target := range routes {
		node[route] = target
	}
	return node
}

// singleExtensionFlow is the smallest login flow with one extension step, placed after the user is
// authenticated and with no onFailure, so any failure of the step ends the flow:
//
//	start -> prompt_credentials -> credentials_auth -> ext_step -> auth_assert -> end
func singleExtensionFlow(name, handle string, extProperties map[string]interface{}) testutils.Flow {
	return testutils.Flow{
		Name:     name,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			startNode("prompt_credentials"),
			promptCredentialsNode("credentials_auth"),
			credentialsAuthNode("ext_step"),
			extensionNode("ext_step", extProperties, map[string]string{"onSuccess": "auth_assert"}),
			authAssertNode(),
			endNode(),
		},
	}
}

// extensionRequestBody is the documented request contract. application and authUser are kept raw so
// tests can check exactly which fields are sent.
type extensionRequestBody struct {
	Version       string                                `json:"version"`
	ExecutionID   string                                `json:"executionId"`
	FlowType      string                                `json:"flowType"`
	CurrentAction string                                `json:"currentAction"`
	CurrentNodeID string                                `json:"currentNodeId"`
	UserInputs    map[string]string                     `json:"userInputs"`
	RuntimeData   map[string]string                     `json:"runtimeData"`
	Application   map[string]json.RawMessage            `json:"application"`
	AuthUser      map[string]map[string]json.RawMessage `json:"authUser"`
}

// serviceCall is one request the mock service received.
type serviceCall struct {
	Method  string
	Path    string
	Query   string
	Headers http.Header
	RawBody string
	Body    extensionRequestBody
	// Fields holds the top-level fields of the body as sent.
	Fields map[string]json.RawMessage
}

// serviceAnswer is what the mock service answers to one extension step.
type serviceAnswer struct {
	status   int
	body     string
	location string
	delay    time.Duration
}

func okAnswer(body string) serviceAnswer {
	return serviceAnswer{status: http.StatusOK, body: body}
}

// seedValues are what the seed endpoint returns, mapped into runtime data by the seeding node.
type seedValues struct {
	OTPSessionToken string
	OAuthState      string
	ClientID        string
}

// mockService stands in for an organization's service. It records every request, answers each
// extension step by the currentNodeId the request names (SUCCESS unless a test sets otherwise), and
// owns a second server that counts hits, as the target of a 3xx answer that must never be followed.
type mockService struct {
	server     *httptest.Server
	target     *httptest.Server
	seed       seedValues
	mu         sync.Mutex
	calls      []serviceCall
	answers    map[string]serviceAnswer
	targetHits int
}

func newMockService(seed seedValues) *mockService {
	m := &mockService{seed: seed, answers: map[string]serviceAnswer{}}
	m.server = httptest.NewServer(http.HandlerFunc(m.handle))
	m.target = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		m.mu.Lock()
		m.targetHits++
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(successBody))
	}))
	return m
}

func (m *mockService) handle(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	call := serviceCall{
		Method:  r.Method,
		Path:    r.URL.Path,
		Query:   r.URL.RawQuery,
		Headers: r.Header.Clone(),
		RawBody: string(raw),
	}
	_ = json.Unmarshal(raw, &call.Body)
	_ = json.Unmarshal(raw, &call.Fields)

	m.mu.Lock()
	m.calls = append(m.calls, call)
	answer, ok := m.answers[call.Body.CurrentNodeID]
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case seedPath:
		seed, _ := json.Marshal(map[string]string{
			"otp": m.seed.OTPSessionToken, "state": m.seed.OAuthState, "client": m.seed.ClientID,
		})
		_, _ = w.Write(seed)
		return
	case probePath:
		_, _ = w.Write([]byte(`{"ok":true}`))
		return
	case servicePath:
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if !ok {
		answer = okAnswer(successBody)
	}
	if answer.delay > 0 {
		select {
		case <-time.After(answer.delay):
		case <-r.Context().Done():
			return
		}
	}
	if answer.location != "" {
		w.Header().Set("Location", answer.location)
	}
	w.WriteHeader(answer.status)
	_, _ = w.Write([]byte(answer.body))
}

// answer sets what the service answers to the extension step nodeID.
func (m *mockService) answer(nodeID string, a serviceAnswer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.answers[nodeID] = a
}

func (m *mockService) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
	m.answers = map[string]serviceAnswer{}
	m.targetHits = 0
}

// allCalls returns every request received, on any path.
func (m *mockService) allCalls() []serviceCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]serviceCall(nil), m.calls...)
}

// callsAt returns the requests received on path.
func (m *mockService) callsAt(path string) []serviceCall {
	var out []serviceCall
	for _, c := range m.allCalls() {
		if c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

// callsFrom returns the extension calls made by the step nodeID, in order.
func (m *mockService) callsFrom(nodeID string) []serviceCall {
	var out []serviceCall
	for _, c := range m.callsAt(servicePath) {
		if c.Body.CurrentNodeID == nodeID {
			out = append(out, c)
		}
	}
	return out
}

func (m *mockService) redirectTargetHits() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.targetHits
}

// connectionURL is the URL the suite's HTTP endpoint connection points at.
func (m *mockService) connectionURL() string {
	return m.server.URL + servicePath + "?" + serviceQuery
}

func (m *mockService) close() {
	m.server.Close()
	m.target.Close()
}

// identityFixtures are the OU, user type, user and application a suite signs in with.
type identityFixtures struct {
	ouID       string
	userTypeID string
	userID     string
	appID      string
	username   string
	password   string
}

// createIdentityFixtures creates the OU, user type and user. The application is created by the
// suite once its flow exists.
func createIdentityFixtures(s *suite.Suite, ou testutils.OrganizationUnit, userType testutils.UserType,
	user testutils.User) *identityFixtures {
	f := &identityFixtures{}

	var attrs map[string]interface{}
	s.Require().NoError(json.Unmarshal(user.Attributes, &attrs), "Failed to read test user attributes")
	f.username, _ = attrs["username"].(string)
	f.password, _ = attrs["password"].(string)

	ouID, err := testutils.CreateOrganizationUnit(ou)
	s.Require().NoError(err, "Failed to create test organization unit")
	f.ouID = ouID

	userType.OUID = ouID
	userTypeID, err := testutils.CreateUserType(userType)
	s.Require().NoError(err, "Failed to create test user type")
	f.userTypeID = userTypeID

	user.OUID = ouID
	userIDs, err := testutils.CreateMultipleUsers(user)
	s.Require().NoError(err, "Failed to create test user")
	s.Require().Len(userIDs, 1, "Exactly one test user should be created")
	f.userID = userIDs[0]
	return f
}

// createApplication creates app in the fixtures' OU, bound to authFlowID.
func (f *identityFixtures) createApplication(s *suite.Suite, app testutils.Application, authFlowID string) {
	app.OUID = f.ouID
	app.AuthFlowID = authFlowID
	appID, err := testutils.CreateApplication(app)
	s.Require().NoError(err, "Failed to create test application")
	f.appID = appID
}

// deleteApplication removes the application. It runs before flows are deleted, since an application
// binds its flow.
func (f *identityFixtures) deleteApplication(s *suite.Suite) {
	if f == nil || f.appID == "" {
		return
	}
	if err := testutils.DeleteApplication(f.appID); err != nil {
		s.T().Logf("Failed to delete test application during teardown: %v", err)
	}
}

// deleteIdentities removes the user, user type and OU, in reverse creation order.
func (f *identityFixtures) deleteIdentities(s *suite.Suite) {
	if f == nil {
		return
	}
	if f.userID != "" {
		if err := testutils.CleanupUsers([]string{f.userID}); err != nil {
			s.T().Logf("Failed to delete test user during teardown: %v", err)
		}
	}
	if f.userTypeID != "" {
		if err := testutils.DeleteUserType(f.userTypeID); err != nil {
			s.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
	if f.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(f.ouID); err != nil {
			s.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// startLogin initiates the flow and checks that it stops at the credentials prompt.
func (f *identityFixtures) startLogin(s *suite.Suite) *common.FlowStep {
	s.T().Helper()
	step, err := common.InitiateAuthenticationFlow(f.appID, false, nil, "")
	s.Require().NoError(err, "Failed to initiate authentication flow")
	s.Require().Equal("INCOMPLETE", step.FlowStatus, "Flow should stop at the credentials prompt")
	s.Require().True(common.ValidateRequiredInputs(step.Data.Inputs, []string{"username", "password"}),
		"The first prompt should ask for username and password")
	return step
}

// submit posts inputs for an action on the step and returns the next step and the raw response. The
// extension steps run synchronously inside this request, so their calls have reached the mock
// service by the time it returns.
func submit(s *suite.Suite, step *common.FlowStep, inputs map[string]string,
	action string) (*common.FlowStep, string) {
	s.T().Helper()
	next, raw, err := common.CompleteFlowWithRawResponse(step.ExecutionID, inputs, action, step.ChallengeToken)
	s.Require().NoError(err, "Flow step should return 200 with a flow step: %s", raw)
	return next, raw
}

// submitCredentials posts the fixture user's username and password on the credentials prompt.
func (f *identityFixtures) submitCredentials(s *suite.Suite, step *common.FlowStep) (*common.FlowStep, string) {
	s.T().Helper()
	return submit(s, step, map[string]string{"username": f.username, "password": f.password}, credentialsAction)
}

// assertSignedIn checks that the flow completed with an assertion for the fixture user.
func (f *identityFixtures) assertSignedIn(s *suite.Suite, result *common.FlowStep, raw string) {
	s.T().Helper()
	s.Require().Nil(result.Error, "A completed flow should carry no error: %s", raw)
	s.Require().Equal("COMPLETE", result.FlowStatus, "Flow should complete: %s", raw)
	s.Require().NotEmpty(result.Assertion, "A completed flow should return an assertion")
	claims, err := testutils.DecodeJWT(result.Assertion)
	s.Require().NoError(err, "The assertion should be a decodable JWT")
	s.Equal(f.userID, claims.Sub, "The assertion subject should be the user who signed in")
}

var paramPlaceholder = regexp.MustCompile(`{{\s*param\(\s*(\w+)\s*\)\s*}}`)

// renderedDescription is the error description as a client renders it: the default text with each
// param placeholder replaced by its value.
func renderedDescription(e *common.ErrorResponse) string {
	return paramPlaceholder.ReplaceAllStringFunc(e.Description.DefaultValue, func(match string) string {
		return e.Description.Params[paramPlaceholder.FindStringSubmatch(match)[1]]
	})
}

// apiRequest sends an authenticated management API request and returns the status and body.
func apiRequest(s *suite.Suite, method, path string, body interface{}) (int, []byte) {
	s.T().Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		s.Require().NoError(err, "Failed to marshal request body")
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	s.Require().NoError(err, "Failed to build %s %s", method, path)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err, "%s %s failed", method, path)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err, "Failed to read the %s %s response", method, path)
	return resp.StatusCode, raw
}

// errorCode returns the code of an API error body.
func errorCode(body []byte) string {
	var e struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &e)
	return e.Code
}

// connectionUsages is the payload of GET /connections/http/{id}/usages.
type connectionUsages struct {
	TotalResults *int           `json:"totalResults"`
	Count        int            `json:"count"`
	Summary      map[string]int `json:"summary"`
	Usages       []struct {
		ResourceType     string `json:"resourceType"`
		ID               string `json:"id"`
		DisplayName      string `json:"displayName"`
		BehaviorOnDelete string `json:"behaviorOnDelete"`
	} `json:"usages"`
}

func getConnectionUsages(s *suite.Suite, connectionID string) connectionUsages {
	s.T().Helper()
	status, body := apiRequest(s, http.MethodGet, "/connections/http/"+connectionID+"/usages", nil)
	s.Require().Equal(http.StatusOK, status, "Usages should be listed: %s", string(body))
	var usages connectionUsages
	s.Require().NoError(json.Unmarshal(body, &usages), "Usages response should decode: %s", string(body))
	s.Require().NotNil(usages.TotalResults, "Usages should report a known total: %s", string(body))
	return usages
}

// containsAny reports which of the needles occur in haystack.
func containsAny(haystack string, needles ...string) []string {
	var found []string
	for _, n := range needles {
		if n != "" && strings.Contains(haystack, n) {
			found = append(found, n)
		}
	}
	return found
}
