// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	ccClientID     = "authz_details_cc_client"
	ccClientSecret = "authz_details_cc_secret"
	ccResource     = "https://authz-details-cc.example.com"

	// processorAccount is the creditor account the mock processor fills in on an approved payment.
	processorAccount = "DE89370400440532013000"
	// processorRefusal is the reason the mock processor gives when it refuses a payment.
	processorRefusal = "The amount exceeds the daily limit"
	// refusedAmount is the payment amount the mock processor refuses.
	refusedAmount = 900

	errorInvalidAuthorizationDetails = "invalid_authorization_details"
)

// paymentSchema is a type schema whose creditor account only the processor supplies.
var paymentSchema = json.RawMessage(`{"type":"object","required":["amount"],"properties":{` +
	`"amount":{"type":"number","minimum":0,"maximum":1000},` +
	`"creditorAccount":{"type":"string","policy":{"suppliedBy":"processor"}}}}`)

// mockProcessor is a type processor that approves payments up to refusedAmount, filling in the creditor
// account, and refuses larger ones.
type mockProcessor struct {
	server *httptest.Server
	mutex  sync.Mutex
	calls  []processorCall
}

// processorCall is one call the mock processor received, with the headers it carried.
type processorCall struct {
	request processRequest
	header  http.Header
}

// newMockProcessor starts a mock processor on the loopback interface.
func newMockProcessor() *mockProcessor {
	p := &mockProcessor{}
	p.server = httptest.NewServer(http.HandlerFunc(p.handle))
	return p
}

// handle decides on each detail the way the mock processor's type says.
func (p *mockProcessor) handle(w http.ResponseWriter, r *http.Request) {
	var req processRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p.mutex.Lock()
	p.calls = append(p.calls, processorCall{request: req, header: r.Header.Clone()})
	p.mutex.Unlock()

	results := make([]processResult, 0, len(req.Details))
	for _, d := range req.Details {
		var detail struct {
			Amount float64 `json:"amount"`
		}
		_ = json.Unmarshal(d.Detail, &detail)
		if detail.Amount >= refusedAmount {
			results = append(results, processResult{DetailID: d.DetailID, Decision: false,
				Context: map[string]string{"reason_user": processorRefusal, "reason_admin": "limit rule"}})
			continue
		}
		results = append(results, processResult{DetailID: d.DetailID, Decision: true,
			Patch: map[string]json.RawMessage{"/creditorAccount": json.RawMessage(`"` + processorAccount + `"`)}})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
}

// reset forgets the calls received so far, so a test sees only its own.
func (p *mockProcessor) reset() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.calls = nil
}

// callFor returns the most recent call that carried a detail of the given type.
func (p *mockProcessor) callFor(detailType string) (processorCall, bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for i := len(p.calls) - 1; i >= 0; i-- {
		for _, d := range p.calls[i].request.Details {
			if d.Type == detailType {
				return p.calls[i], true
			}
		}
	}
	return processorCall{}, false
}

// newStaticProcessor starts a processor on the loopback interface that answers every call with the given
// status and body.
func newStaticProcessor(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// unreachableEndpoint returns a loopback processor endpoint nothing listens on.
func unreachableEndpoint() string {
	server := httptest.NewServer(http.NotFoundHandler())
	endpoint := server.URL + "/process"
	server.Close()
	return endpoint
}

// unusableAnswers are processor answers that decide nothing, keyed by what is wrong with them.
var unusableAnswers = map[string]struct {
	status int
	body   string
}{
	"an error status":         {http.StatusInternalServerError, `{"results":[]}`},
	"a body that is not JSON": {http.StatusOK, `not json`},
	"no result for a detail":  {http.StatusOK, `{"results":[]}`},
}

type ClientCredentialsTestSuite struct {
	suite.Suite
	processor        *mockProcessor
	ouID             string
	resourceServerID string
	appID            string
	paymentTypeID    string
	paymentType      string
	plainTypeID      string
	plainType        string
}

func TestClientCredentialsTestSuite(t *testing.T) {
	suite.Run(t, new(ClientCredentialsTestSuite))
}

func (s *ClientCredentialsTestSuite) SetupSuite() {
	s.processor = newMockProcessor()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "authz-details-cc-ou",
		Name:        "Authorization Details Client Credentials OU",
		Description: "Organization unit for authorization details client credentials tests",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Authorization Details Client Credentials API",
		Identifier: ccResource,
		OUID:       s.ouID,
	}, []testutils.Action{})
	s.Require().NoError(err)
	s.resourceServerID = rsID

	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "Authorization Details Client Credentials App",
		Type: "m2m",
		OUID: s.ouID,
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                ccClientID,
				"clientSecret":            ccClientSecret,
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	s.Require().NoError(err)
	s.appID = appID

	s.paymentType = uniqueTypeValue("cc-payment")
	s.paymentTypeID = createType(s.T(), typeRequest{Type: s.paymentType, Name: "Payment", Schema: paymentSchema,
		Processor: &processorConfig{Endpoint: s.processor.server.URL + "/process"}}).ID
	s.plainType = uniqueTypeValue("cc-plain")
	s.plainTypeID = createType(s.T(), typeRequest{Type: s.plainType, Name: "Plain", Schema: flightSchema}).ID
}

func (s *ClientCredentialsTestSuite) SetupTest() {
	s.processor.reset()
}

func (s *ClientCredentialsTestSuite) TearDownSuite() {
	for _, id := range []string{s.paymentTypeID, s.plainTypeID} {
		if id != "" {
			deleteType(s.T(), id)
		}
	}
	if s.appID != "" {
		_ = testutils.DeleteApplication(s.appID)
	}
	if s.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(s.resourceServerID)
	}
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
	if s.processor != nil {
		s.processor.server.Close()
	}
}

func (s *ClientCredentialsTestSuite) TestApprovedDetailsAreIssuedWithTheProcessorPatch() {
	details := `[{"type":"` + s.paymentType + `","amount":45}]`
	status, body := requestClientCredentials(s.T(), ccClientID, ccClientSecret, ccResource, details)
	s.Require().Equal(http.StatusOK, status, string(body))

	var token struct {
		AccessToken          string            `json:"access_token"`
		RefreshToken         string            `json:"refresh_token"`
		AuthorizationDetails []json.RawMessage `json:"authorization_details"`
	}
	s.Require().NoError(json.Unmarshal(body, &token))
	expected := `[{"type":"` + s.paymentType + `","amount":45,"creditorAccount":"` + processorAccount + `"}]`
	s.JSONEq(expected, rawJSON(s.T(), token.AuthorizationDetails),
		"the processor's patch is applied, with no internal identifier")
	s.Empty(token.RefreshToken, "client_credentials issues no refresh token")
	assertNoGrantClaims(s.T(), body, "the token response")

	call, ok := s.processor.callFor(s.paymentType)
	s.Require().True(ok, "the processor is called")
	s.Equal("application", call.request.Subject.Type, "the client itself is the subject")
	s.Equal(s.appID, call.request.Subject.ID)
	s.Equal("application", call.request.Client.Type)
	s.Equal(ccClientID, call.request.Client.ID)
	s.Equal(ccResource, call.request.Resource)
	s.Require().Len(call.request.Details, 1)
	s.NotEmpty(call.request.Details[0].DetailID)
	s.Equal(s.paymentType, call.request.Details[0].Type)
	s.Equal(1, call.request.Details[0].TypeVersion)
	s.JSONEq(`{"type":"`+s.paymentType+`","amount":45}`, string(call.request.Details[0].Detail),
		"the processor sees the detail as requested")

	assertTokenDetails(s.T(), token.AccessToken, expected)
	assertIntrospectedDetails(s.T(), token.AccessToken, ccClientID, ccClientSecret, expected)
}

func (s *ClientCredentialsTestSuite) TestRefusedDetailsAreAnInvalidRequest() {
	details := `[{"type":"` + s.paymentType + `","amount":950}]`
	status, body := requestClientCredentials(s.T(), ccClientID, ccClientSecret, ccResource, details)
	s.Equal(http.StatusBadRequest, status, string(body))

	resp := oauthError(s.T(), body)
	s.Equal(errorInvalidAuthorizationDetails, resp.Error)
	s.Contains(resp.ErrorDescription, processorRefusal, "the processor's reason reaches the client")
}

// A detail the processor refuses is left out, and the others are issued.
func (s *ClientCredentialsTestSuite) TestPartlyRefusedDetailsIssueTheRest() {
	details := `[{"type":"` + s.paymentType + `","amount":45},{"type":"` + s.paymentType + `","amount":950}]`
	status, body := requestClientCredentials(s.T(), ccClientID, ccClientSecret, ccResource, details)
	s.Require().Equal(http.StatusOK, status, string(body))

	token := readTokenResult(s.T(), status, body)
	expected := `[{"type":"` + s.paymentType + `","amount":45,"creditorAccount":"` + processorAccount + `"}]`
	s.JSONEq(expected, rawJSON(s.T(), token.details), "the refused detail is left out")
	assertTokenDetails(s.T(), token.accessToken, expected)
}

// A processor that cannot be reached or gives no usable answer fails the request as a server error, and
// nothing is let through unchecked.
func (s *ClientCredentialsTestSuite) TestUnusableProcessorIsAServerError() {
	endpoints := map[string]string{"an unreachable processor": unreachableEndpoint()}
	for name, answer := range unusableAnswers {
		server := newStaticProcessor(answer.status, answer.body)
		defer server.Close()
		endpoints[name] = server.URL + "/process"
	}
	for name, endpoint := range endpoints {
		value := uniqueTypeValue("cc-unusable")
		created := createType(s.T(), typeRequest{Type: value, Name: "Unusable", Schema: paymentSchema,
			Processor: &processorConfig{Endpoint: endpoint}})
		status, body := requestClientCredentials(s.T(), ccClientID, ccClientSecret, ccResource,
			`[{"type":"`+value+`","amount":45}]`)
		deleteType(s.T(), created.ID)

		s.Equal(http.StatusInternalServerError, status, "%s: %s", name, body)
		s.Equal("server_error", oauthError(s.T(), body).Error, name)
	}
}

// The processor's credential is sent with each call, as the type's outbound authentication says.
func (s *ClientCredentialsTestSuite) TestProcessorAuthenticationReachesTheProcessor() {
	cases := map[string]struct {
		auth  processorAuthentication
		check func(http.Header)
	}{
		"bearer": {
			auth: processorAuthentication{Type: "bearer", Properties: map[string]string{"token": "cc-bearer-token"}},
			check: func(h http.Header) {
				s.Equal("Bearer cc-bearer-token", h.Get("Authorization"))
			},
		},
		"api key": {
			auth: processorAuthentication{Type: "api_key",
				Properties: map[string]string{"X-Processor-Key": "cc-api-key"}},
			check: func(h http.Header) {
				s.Equal("cc-api-key", h.Get("X-Processor-Key"))
				s.Empty(h.Get("Authorization"))
			},
		},
		"basic": {
			auth: processorAuthentication{Type: "basic",
				Properties: map[string]string{"username": "cc-user", "password": "cc-password"}},
			check: func(h http.Header) {
				req := &http.Request{Header: h}
				username, password, ok := req.BasicAuth()
				s.True(ok, "basic credentials are sent")
				s.Equal("cc-user", username)
				s.Equal("cc-password", password)
			},
		},
	}
	for name, c := range cases {
		value := uniqueTypeValue("cc-authenticated")
		auth := c.auth
		created := createType(s.T(), typeRequest{Type: value, Name: "Authenticated", Schema: paymentSchema,
			Processor: &processorConfig{Endpoint: s.processor.server.URL + "/process", Authentication: &auth}})
		status, body := requestClientCredentials(s.T(), ccClientID, ccClientSecret, ccResource,
			`[{"type":"`+value+`","amount":45}]`)
		deleteType(s.T(), created.ID)

		s.Equal(http.StatusOK, status, "%s: %s", name, body)
		call, ok := s.processor.callFor(value)
		if s.True(ok, "%s: the processor is called", name) {
			c.check(call.header)
		}
	}
}

func (s *ClientCredentialsTestSuite) TestTypeWithoutAProcessorIsRefused() {
	details := `[{"type":"` + s.plainType + `","route":"CMB-SIN"}]`
	status, body := requestClientCredentials(s.T(), ccClientID, ccClientSecret, ccResource, details)
	s.Equal(http.StatusBadRequest, status, string(body))

	resp := oauthError(s.T(), body)
	s.Equal(errorInvalidAuthorizationDetails, resp.Error)
	// RAR-1024: a type without a processor needs a user to consent.
	s.Equal("authorization details of type "+s.plainType+" cannot be requested without a user",
		resp.ErrorDescription)
}

func (s *ClientCredentialsTestSuite) TestMalformedDetailsAreRefused() {
	unknownType := uniqueTypeValue("unknown")
	for name, tc := range map[string]struct{ details, description string }{
		"not JSON": {"not json", "authorization_details must be a JSON array"},
		"an unknown type": {`[{"type":"` + unknownType + `"}]`,
			"authorization_details entry 0 names the unknown type " + unknownType},
		"an undeclared field": {`[{"type":"` + s.paymentType + `","amount":5,"note":"x"}]`,
			"authorization_details entry 0 does not conform to its type: "},
		"a processor field": {`[{"type":"` + s.paymentType + `","amount":5,"creditorAccount":"x"}]`,
			"authorization_details entry 0 is not allowed: field creditorAccount is supplied by the processor"},
		"an imprecise number": {`[{"type":"` + s.paymentType + `","amount":1000.0000000000000001}]`,
			"authorization_details entry 0 does not conform to its type: the number at amount has more precision"},
	} {
		status, body := requestClientCredentials(s.T(), ccClientID, ccClientSecret, ccResource, tc.details)
		s.Equal(http.StatusBadRequest, status, name)
		resp := oauthError(s.T(), body)
		s.Equal(errorInvalidAuthorizationDetails, resp.Error, name)
		s.True(strings.HasPrefix(resp.ErrorDescription, tc.description), "%s: %s", name, resp.ErrorDescription)
	}
	_, called := s.processor.callFor(s.paymentType)
	s.False(called, "the processor is not called for details refused before it")
}

// Details bind the token to the requested resource server.
func (s *ClientCredentialsTestSuite) TestDetailsBindTheTokenToTheResource() {
	details := `[{"type":"` + s.paymentType + `","amount":45}]`
	status, body := requestClientCredentials(s.T(), ccClientID, ccClientSecret, ccResource, details)
	s.Require().Equal(http.StatusOK, status, string(body))

	s.Equal(ccResource, tokenAudience(s.T(), body), "a single audience is carried as a string")
}

// Details without a resource bind the token to the default resource server, as permission scopes do.
func (s *ClientCredentialsTestSuite) TestDetailsWithoutAResourceUseTheDefaultResourceServer() {
	s.Require().NoError(testutils.PutDefaultResourceServer(s.resourceServerID))
	defer func() { _ = testutils.PutDefaultResourceServer("") }()

	details := `[{"type":"` + s.paymentType + `","amount":45}]`
	status, body := requestClientCredentials(s.T(), ccClientID, ccClientSecret, "", details)
	s.Require().Equal(http.StatusOK, status, string(body))

	s.Equal(ccResource, tokenAudience(s.T(), body), "a single audience is carried as a string")
	call, ok := s.processor.callFor(s.paymentType)
	s.Require().True(ok)
	s.Equal(ccResource, call.request.Resource, "the processor is told the default resource server")
}

// Details with neither a resource nor a default resource server are refused before the processor is called.
func (s *ClientCredentialsTestSuite) TestDetailsWithoutAResourceOrDefaultAreAnInvalidTarget() {
	s.Require().NoError(testutils.PutDefaultResourceServer(""))

	details := `[{"type":"` + s.paymentType + `","amount":45}]`
	status, body := requestClientCredentials(s.T(), ccClientID, ccClientSecret, "", details)
	s.Equal(http.StatusBadRequest, status, string(body))

	s.Equal("invalid_target", oauthError(s.T(), body).Error)
	_, called := s.processor.callFor(s.paymentType)
	s.False(called, "the processor is not called for a request bound to no resource server")
}

// tokenAudience returns the aud claim of the access token in a token response.
func tokenAudience(t *testing.T, body []byte) any {
	t.Helper()
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil {
		t.Fatalf("decoding token response: %v", err)
	}
	claims, err := testutils.DecodeJWTPayloadMap(token.AccessToken)
	if err != nil {
		t.Fatalf("decoding access token: %v", err)
	}
	return claims["aud"]
}

// requestClientCredentials requests a client_credentials token carrying the given authorization details, for the
// given resource unless it is empty.
func requestClientCredentials(t *testing.T, clientID, clientSecret, resource, details string) (int, []byte) {
	t.Helper()
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if resource != "" {
		form.Set("resource", resource)
	}
	form.Set("authorization_details", details)
	return postForm(t, "/oauth2/token", form, clientID, clientSecret)
}

// introspect returns the introspection response for a token.
func introspect(t *testing.T, token, clientID, clientSecret string) map[string]interface{} {
	t.Helper()
	status, body := postForm(t, "/oauth2/introspect", url.Values{"token": {token}}, clientID, clientSecret)
	if status != http.StatusOK {
		t.Fatalf("introspecting: status %d, body %s", status, body)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decoding introspection response: %v", err)
	}
	return resp
}

// postForm sends a form to an OAuth endpoint with HTTP Basic client authentication.
func postForm(t *testing.T, path string, form url.Values, clientID, clientSecret string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+path,
		bytes.NewBufferString(form.Encode()))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	resp, err := testutils.GetNoRedirectHTTPClient().Do(req)
	if err != nil {
		t.Fatalf("sending POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	return resp.StatusCode, body
}

// oauthError decodes an OAuth error response.
func oauthError(t *testing.T, body []byte) oauthErrorResponse {
	t.Helper()
	var resp oauthErrorResponse
	if err := json.Unmarshal(body, &resp); err != nil || resp.Error == "" {
		t.Fatalf("expected an OAuth error response, got %s", strings.TrimSpace(string(body)))
	}
	return resp
}

// grantClaims are the private claims the refresh token keeps for the grant, which nothing else may carry.
var grantClaims = []string{"access_token_authorization_details", "access_token_authorization_detail_versions"}

// rawJSON encodes a list of details as one JSON array.
func rawJSON(t *testing.T, details []json.RawMessage) string {
	t.Helper()
	if details == nil {
		details = []json.RawMessage{}
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		t.Fatalf("encoding details: %v", err)
	}
	return string(encoded)
}

// assertNoGrantClaims checks that a JSON object carries none of the refresh token's private grant claims.
func assertNoGrantClaims(t *testing.T, body []byte, what string) {
	t.Helper()
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	for _, claim := range grantClaims {
		assert.NotContains(t, fields, claim, "%s does not carry %s", what, claim)
	}
}

// assertTokenDetails checks that the access token carries exactly the expected details, and none of the
// refresh token's private grant claims.
func assertTokenDetails(t *testing.T, accessToken, expected string) {
	t.Helper()
	payload, err := testutils.DecodeJWTPayloadMap(accessToken)
	require.NoError(t, err)
	claim, ok := payload[authorizationDetailsField]
	require.True(t, ok, "the access token carries the details")
	encoded, err := json.Marshal(claim)
	require.NoError(t, err)
	assert.JSONEq(t, expected, string(encoded), "the access token carries the granted details")
	for _, name := range grantClaims {
		assert.NotContains(t, payload, name, "the access token does not carry %s", name)
	}
}

// assertIntrospectedDetails checks that introspection reports exactly the expected details, and none of the
// refresh token's private grant claims.
func assertIntrospectedDetails(t *testing.T, token, clientID, clientSecret, expected string) {
	t.Helper()
	introspected := introspect(t, token, clientID, clientSecret)
	assert.Equal(t, true, introspected["active"])
	claim, ok := introspected[authorizationDetailsField]
	require.True(t, ok, "introspection carries the details")
	encoded, err := json.Marshal(claim)
	require.NoError(t, err)
	assert.JSONEq(t, expected, string(encoded), "introspection carries the granted details")
	for _, name := range grantClaims {
		assert.NotContains(t, introspected, name, "introspection does not carry %s", name)
	}
}

const authorizationDetailsField = "authorization_details"

// tokenResult is a token endpoint answer with the authorization details it carries.
type tokenResult struct {
	status       int
	body         []byte
	accessToken  string
	refreshToken string
	details      []json.RawMessage
}

// readTokenResult reads a token endpoint answer.
func readTokenResult(t *testing.T, status int, body []byte) tokenResult {
	t.Helper()
	result := tokenResult{status: status, body: body}
	if status == http.StatusOK {
		var token struct {
			AccessToken          string            `json:"access_token"`
			RefreshToken         string            `json:"refresh_token"`
			AuthorizationDetails []json.RawMessage `json:"authorization_details"`
		}
		require.NoError(t, json.Unmarshal(body, &token))
		result.accessToken = token.AccessToken
		result.refreshToken = token.RefreshToken
		result.details = token.AuthorizationDetails
	}
	return result
}
