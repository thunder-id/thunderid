// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	acClientID     = "authz_details_ac_client"
	acClientSecret = "authz_details_ac_secret"
	acRedirectURI  = "https://localhost:3000/authz-details"
	acResource     = "https://authz-details-ac.example.com"
	acUserType     = "authz_details_user"
	acUsername     = "authzdetailsuser"
	acPassword     = "testpassword"

	consentPromptKey          = "consentPrompt"
	consentDecisionsInputKey  = "consent_decisions"
	authorizationDetailsField = "authorization_details"
	purposeTypeDetails        = "authorization_details"
)

// detailsFlow authenticates the user, runs the type processors and asks for consent before asserting.
var detailsFlow = testutils.Flow{
	Name:     "Authorization Details Consent Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "authz_details_consent_flow",
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
				"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
			}},
		},
		{
			"id":   "credentials_auth",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "CredentialsAuthExecutor",
				"inputs": []map[string]interface{}{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
			},
			"onSuccess": "authorization_details",
		},
		{
			"id":        "authorization_details",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthorizationDetailExecutor"},
			"onSuccess": "consent",
		},
		{
			"id":        "consent",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "ConsentExecutor"},
			"onSuccess": "auth_assert",
		},
		{
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		{"id": "end", "type": "END"},
	},
}

type AuthorizationCodeTestSuite struct {
	suite.Suite
	processor        *mockProcessor
	ouID             string
	userTypeID       string
	userID           string
	resourceServerID string
	flowID           string
	appID            string
	paymentTypeID    string
	paymentType      string
	flightTypeID     string
	flightType       string
}

func TestAuthorizationCodeTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationCodeTestSuite))
}

func (s *AuthorizationCodeTestSuite) SetupSuite() {
	s.processor = newMockProcessor()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "authz-details-ac-ou",
		Name:        "Authorization Details Authorization Code OU",
		Description: "Organization unit for authorization details login tests",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      acUserType,
		DisplayName: "Authorization Details User",
		OUID:        s.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	s.Require().NoError(err)
	s.userTypeID = userTypeID

	userID, err := testutils.CreateUser(testutils.User{
		Type:       acUserType,
		OUID:       s.ouID,
		Attributes: json.RawMessage(`{"username":"` + acUsername + `","password":"` + acPassword + `"}`),
	})
	s.Require().NoError(err)
	s.userID = userID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Authorization Details Authorization Code API",
		Identifier: acResource,
		OUID:       s.ouID,
	}, []testutils.Action{})
	s.Require().NoError(err)
	s.resourceServerID = rsID

	flowID, err := testutils.CreateFlow(detailsFlow)
	s.Require().NoError(err)
	s.flowID = flowID

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             "Authorization Details Authorization Code App",
		Type:             "fullstack",
		OUID:             s.ouID,
		AuthFlowID:       s.flowID,
		AllowedUserTypes: []string{acUserType},
		LoginConsent:     map[string]interface{}{"validityPeriod": 3600},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                acClientID,
				"clientSecret":            acClientSecret,
				"redirectUris":            []string{acRedirectURI},
				"grantTypes":              []string{"authorization_code", "refresh_token"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"scopes":                  []string{"openid"},
			},
		}},
	})
	s.Require().NoError(err)
	s.appID = appID

	s.paymentType = uniqueTypeValue("ac-payment")
	s.paymentTypeID = createType(s.T(), typeRequest{Type: s.paymentType, Name: "Payment", Schema: paymentSchema,
		ConsentTemplate: "Pay {{amount}}",
		Processor:       &processorConfig{Endpoint: s.processor.server.URL + "/process"}}).ID
	s.flightType = uniqueTypeValue("ac-flight")
	s.flightTypeID = createType(s.T(), typeRequest{Type: s.flightType, Name: "Flight", Schema: flightSchema,
		ConsentTemplate: "Book {{route}}"}).ID
}

func (s *AuthorizationCodeTestSuite) SetupTest() {
	s.processor.reset()
}

func (s *AuthorizationCodeTestSuite) TearDownSuite() {
	for _, id := range []string{s.paymentTypeID, s.flightTypeID} {
		if id != "" {
			deleteType(s.T(), id)
		}
	}
	if s.appID != "" {
		_ = testutils.DeleteApplication(s.appID)
	}
	if s.flowID != "" {
		_ = testutils.DeleteFlow(s.flowID)
	}
	if s.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(s.resourceServerID)
	}
	if s.userID != "" {
		_ = testutils.DeleteUser(s.userID)
	}
	if s.userTypeID != "" {
		_ = testutils.DeleteUserType(s.userTypeID)
	}
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
	if s.processor != nil {
		s.processor.server.Close()
	}
}

func (s *AuthorizationCodeTestSuite) TestApprovedDetailsReachTheToken() {
	details := `[{"type":"` + s.paymentType + `","amount":45}]`
	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)
	s.Require().Len(purpose.Optional, 1, "the detail is prompted")
	s.Equal(s.paymentType, purpose.Optional[0].DetailType)
	s.Equal("Pay 45", purpose.Optional[0].Summary)

	call, ok := s.processor.callFor(s.paymentType)
	s.Require().True(ok, "the processor decides before the user is asked")
	s.Equal("user", call.request.Subject.Type)
	s.Equal(s.userID, call.request.Subject.ID)
	s.Equal("application", call.request.Client.Type)
	s.Equal(acClientID, call.request.Client.ID)
	s.Equal(acResource, call.request.Resource)
	s.Require().Len(call.request.Details, 1)
	s.NotEmpty(call.request.Details[0].DetailID)
	s.Equal(purpose.Optional[0].Name, call.request.Details[0].DetailID,
		"the prompt names the detail by the identifier the processor saw")
	s.Equal(s.paymentType, call.request.Details[0].Type)
	s.Equal(1, call.request.Details[0].TypeVersion)
	s.JSONEq(`{"type":"`+s.paymentType+`","amount":45}`, string(call.request.Details[0].Detail),
		"the processor sees the detail as requested")

	final := s.decide(executionID, step.ChallengeToken, purpose, true)
	token := s.exchange(authID, final.Assertion, "")
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	expected := `[{"type":"` + s.paymentType + `","amount":45,"creditorAccount":"` + processorAccount + `"}]`
	s.JSONEq(expected, rawJSON(s.T(), token.details))
	assertNoGrantClaims(s.T(), token.body, "the token response")
	assertTokenDetails(s.T(), token.accessToken, expected)
	assertIntrospectedDetails(s.T(), token.accessToken, acClientID, acClientSecret, expected)
}

func (s *AuthorizationCodeTestSuite) TestDeniedDetailIsNotGranted() {
	details := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}]`
	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)

	final := s.decide(executionID, step.ChallengeToken, purpose, false)
	token := s.exchange(authID, final.Assertion, "")
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	s.Empty(token.details, "a detail the user denied is not granted")
}

func (s *AuthorizationCodeTestSuite) TestClientNarrowsTheGrantAtTheTokenEndpoint() {
	details := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}]`
	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)
	final := s.decide(executionID, step.ChallengeToken, purpose, true)

	narrowed := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":250}]`
	token := s.exchange(authID, final.Assertion, narrowed)
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	s.Require().Len(token.details, 1)
	s.JSONEq(`{"type":"`+s.flightType+`","route":"CMB-SIN","maxFare":250}`, string(token.details[0]))
}

func (s *AuthorizationCodeTestSuite) TestClientCannotWidenTheGrant() {
	details := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}]`
	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)
	final := s.decide(executionID, step.ChallengeToken, purpose, true)

	widened := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":800}]`
	token := s.exchange(authID, final.Assertion, widened)
	s.Equal(http.StatusBadRequest, token.status, string(token.body))
	s.Equal(errorInvalidAuthorizationDetails, oauthError(s.T(), token.body).Error)
}

func (s *AuthorizationCodeTestSuite) TestInvalidDetailsAreRefusedAtTheAuthorizationEndpoint() {
	for name, details := range map[string]string{
		"an unknown type": `[{"type":"` + uniqueTypeValue("unknown") + `"}]`,
		"not JSON":        "not json",
	} {
		resp, err := testutils.InitiateAuthorizationFlowWithExtraParams(acClientID, acRedirectURI, "code", "openid",
			"authz_details_state", map[string]string{"resource": acResource, "authorization_details": details})
		s.Require().NoError(err)
		_ = resp.Body.Close()

		location := resp.Header.Get("Location")
		s.Require().NotEmpty(location, name)
		s.NoError(testutils.ValidateOAuth2ErrorRedirect(location, errorInvalidAuthorizationDetails, ""), name)
	}
}

// A token request may only narrow the granted details, so details that do not read as a narrowing of one granted
// detail each are refused.
func (s *AuthorizationCodeTestSuite) TestClientNarrowingOutsideTheGrantIsRefused() {
	granted := `{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}`
	narrowed := `{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":250}`
	for name, tc := range map[string]struct{ details, description string }{
		"not JSON": {"not json", "authorization_details must be a JSON array"},
		"a type not in the grant": {`[{"type":"` + s.paymentType + `","amount":45}]`,
			"authorization_details entry 0 does not match a granted detail"},
		"a detail that does not conform": {`[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":"low"}]`,
			"authorization_details entry 0 does not conform to its type"},
		"one granted detail claimed twice": {`[` + narrowed + `,` + narrowed + `]`,
			"authorization_details entry 1 does not match a granted detail"},
	} {
		authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(`[` + granted + `]`)
		final := s.decide(executionID, step.ChallengeToken, purpose, true)

		token := s.exchange(authID, final.Assertion, tc.details)
		s.Equal(http.StatusBadRequest, token.status, "%s: %s", name, token.body)
		resp := oauthError(s.T(), token.body)
		s.Equal(errorInvalidAuthorizationDetails, resp.Error, name)
		s.True(strings.HasPrefix(resp.ErrorDescription, tc.description), "%s: %s", name, resp.ErrorDescription)
	}
}

// A token request cannot name details when the user granted none.
func (s *AuthorizationCodeTestSuite) TestClientCannotNameDetailsWhenNoneWereGranted() {
	details := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}]`
	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)
	final := s.decide(executionID, step.ChallengeToken, purpose, false)

	token := s.exchange(authID, final.Assertion, details)
	s.Equal(http.StatusBadRequest, token.status, string(token.body))
	resp := oauthError(s.T(), token.body)
	s.Equal(errorInvalidAuthorizationDetails, resp.Error)
	s.Equal("no authorization details were granted", resp.ErrorDescription)
}

// A grant of several types is narrowed type by type, and a processor-supplied field the client leaves out keeps
// its granted value, while one the client repeats must match it.
func (s *AuthorizationCodeTestSuite) TestClientNarrowsAGrantOfSeveralTypes() {
	payment := `{"type":"` + s.paymentType + `","amount":45}`
	issuedPayment := `{"type":"` + s.paymentType + `","amount":45,"creditorAccount":"` + processorAccount + `"}`
	details := `[` + payment + `,{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}]`
	narrowedFlight := `{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":250}`
	for name, requestedPayment := range map[string]string{
		"the supplied field left out": payment,
		"the supplied field repeated": issuedPayment,
	} {
		authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)
		s.Require().Len(purpose.Optional, 2, name)
		final := s.decide(executionID, step.ChallengeToken, purpose, true)

		token := s.exchange(authID, final.Assertion, `[`+narrowedFlight+`,`+requestedPayment+`]`)
		s.Require().Equal(http.StatusOK, token.status, "%s: %s", name, token.body)
		s.JSONEq(`[`+narrowedFlight+`,`+issuedPayment+`]`, rawJSON(s.T(), token.details), name)
	}
}

// A detail cannot be narrowed once its type has changed since the grant.
func (s *AuthorizationCodeTestSuite) TestClientCannotNarrowADetailWhoseTypeChanged() {
	value := uniqueTypeValue("ac-changed")
	req := typeRequest{Type: value, Name: "Changed", Schema: flightSchema, ConsentTemplate: "Book {{route}}"}
	changed := createType(s.T(), req)
	defer deleteType(s.T(), changed.ID)

	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(
		`[{"type":"` + value + `","route":"CMB-SIN","maxFare":400}]`)
	final := s.decide(executionID, step.ChallengeToken, purpose, true)
	req.ConsentTemplate = "Book a flight on {{route}}"
	updateType(s.T(), changed.ID, req)

	token := s.exchange(authID, final.Assertion, `[{"type":"`+value+`","route":"CMB-SIN","maxFare":250}]`)
	s.Equal(http.StatusBadRequest, token.status, string(token.body))
	s.Equal(errorInvalidAuthorizationDetails, oauthError(s.T(), token.body).Error)
}

// A change the field's policy allows is still refused when it takes the field past a limit the policy sets.
func (s *AuthorizationCodeTestSuite) TestClientNarrowingPastAFieldLimitIsRefused() {
	value := uniqueTypeValue("ac-limited")
	limited := createType(s.T(), typeRequest{Type: value, Name: "Limited", Schema: objectSchema(
		`"route":{"type":"string"},"validFrom":{"type":"string","format":"date",` +
			`"policy":{"modification":"increase","modifiableBy":["client"],"maximum":"2027-12-31"}}`)})
	defer deleteType(s.T(), limited.ID)

	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(
		`[{"type":"` + value + `","route":"CMB-SIN","validFrom":"2027-06-01"}]`)
	final := s.decide(executionID, step.ChallengeToken, purpose, true)

	token := s.exchange(authID, final.Assertion,
		`[{"type":"`+value+`","route":"CMB-SIN","validFrom":"2028-01-15"}]`)
	s.Equal(http.StatusBadRequest, token.status, string(token.body))
	resp := oauthError(s.T(), token.body)
	s.Equal(errorInvalidAuthorizationDetails, resp.Error)
	s.Equal("authorization_details entry 0 is not allowed: field validFrom is above its maximum",
		resp.ErrorDescription)
}

// A refresh renews a reusable detail while its type stays on the version it was granted on, leaves out a
// detail of a type that does not allow reuse, and leaves out the reusable one once its type changes in a way
// that moves the version.
func (s *AuthorizationCodeTestSuite) TestRefreshDropsADetailWhoseTypeChanged() {
	value := uniqueTypeValue("ac-reusable")
	req := typeRequest{Type: value, Name: "Reusable", Schema: flightSchema, ConsentTemplate: "Book {{route}}",
		Reuse: true}
	reusable := createType(s.T(), req)
	defer deleteType(s.T(), reusable.ID)

	reusableDetail := `{"type":"` + value + `","route":"CMB-SIN","maxFare":400}`
	oneOffDetail := `{"type":"` + s.flightType + `","route":"CMB-LHR","maxFare":900}`
	details := `[` + reusableDetail + `,` + oneOffDetail + `]`
	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)
	s.Require().Len(purpose.Optional, 2, "both details are prompted")
	final := s.decide(executionID, step.ChallengeToken, purpose, true)
	token := s.exchange(authID, final.Assertion, "")
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	s.JSONEq(details, rawJSON(s.T(), token.details), "the code exchange issues both details")
	s.Require().NotEmpty(token.refreshToken)

	renewed := s.refresh(token.refreshToken)
	s.Require().Equal(http.StatusOK, renewed.status, string(renewed.body))
	s.JSONEq(`[`+reusableDetail+`]`, rawJSON(s.T(), renewed.details),
		"only the reusable detail is renewed while its type is unchanged")
	assertNoGrantClaims(s.T(), renewed.body, "the refreshed token response")
	assertTokenDetails(s.T(), renewed.accessToken, `[`+reusableDetail+`]`)
	assertIntrospectedDetails(s.T(), renewed.accessToken, acClientID, acClientSecret, `[`+reusableDetail+`]`)

	req.ConsentTemplate = "Book a flight on {{route}}"
	s.Require().Equal(reusable.Version+1, updateType(s.T(), reusable.ID, req).Version)

	afterChange := s.refresh(renewed.refreshToken)
	s.Require().Equal(http.StatusOK, afterChange.status, string(afterChange.body))
	s.Empty(afterChange.details, "the detail is not renewed after its type changed")
}

// A refresh leaves out a detail whose type was deleted, and one whose type was deleted and registered again
// under the same value, since the user approved neither type.
func (s *AuthorizationCodeTestSuite) TestRefreshDropsADetailWhoseTypeWasDeleted() {
	value := uniqueTypeValue("ac-deleted")
	req := typeRequest{Type: value, Name: "Deleted", Schema: flightSchema, ConsentTemplate: "Book {{route}}",
		Reuse: true}
	deleted := createType(s.T(), req)

	details := `[{"type":"` + value + `","route":"CMB-SIN","maxFare":400}]`
	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)
	final := s.decide(executionID, step.ChallengeToken, purpose, true)
	first := s.exchange(authID, final.Assertion, "")
	s.Require().Equal(http.StatusOK, first.status, string(first.body))
	authID, _, step = s.login(acClientID, details, nil)
	s.Require().Equal("COMPLETE", step.FlowStatus, "the approval covers the second request")
	second := s.exchange(authID, step.Assertion, "")
	s.Require().Equal(http.StatusOK, second.status, string(second.body))

	deleteType(s.T(), deleted.ID)
	renewed := s.refresh(first.refreshToken)
	s.Require().Equal(http.StatusOK, renewed.status, string(renewed.body))
	s.Empty(renewed.details, "the detail is not renewed after its type was deleted")

	recreated := createType(s.T(), req)
	defer deleteType(s.T(), recreated.ID)
	renewed = s.refresh(second.refreshToken)
	s.Require().Equal(http.StatusOK, renewed.status, string(renewed.body))
	s.Empty(renewed.details, "the detail is not renewed under a type registered again")
}

// refresh redeems a refresh token.
func (s *AuthorizationCodeTestSuite) refresh(refreshToken string) tokenResult {
	result, err := testutils.RefreshAccessTokenRaw(acClientID, acClientSecret, refreshToken, "")
	s.Require().NoError(err)
	return s.tokenResult(result.StatusCode, result.Body)
}

// loginExpectingDetailsPrompt starts an authorization request for the given details and signs the test user
// in, requiring a consent prompt. It returns the authorization id, the execution id, the prompt step and the
// authorization details purpose of the prompt.
func (s *AuthorizationCodeTestSuite) loginExpectingDetailsPrompt(details string) (
	string, string, *testutils.FlowStep, consentPurposePrompt) {
	authID, executionID, step := s.login(acClientID, details, nil)
	return authID, executionID, step, detailsPurpose(s.T(), step)
}

// login starts an authorization request of the client for the given details, with any extra parameters, and
// signs the test user in. It returns the authorization id, the execution id and the step after sign-in.
func (s *AuthorizationCodeTestSuite) login(clientID, details string, extra map[string]string) (
	string, string, *testutils.FlowStep) {
	params := map[string]string{"resource": acResource, "authorization_details": details}
	for key, value := range extra {
		params[key] = value
	}
	resp, err := testutils.InitiateAuthorizationFlowWithExtraParams(clientID, acRedirectURI, "code", "openid",
		"authz_details_state", params)
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	return s.signIn(resp.Header.Get("Location"))
}

// signInExpectingDetailsPrompt follows the login redirect of an authorization request and signs the test user
// in, requiring a consent prompt. It returns the authorization id, the execution id, the prompt step and the
// authorization details purpose of the prompt.
func (s *AuthorizationCodeTestSuite) signInExpectingDetailsPrompt(location string) (
	string, string, *testutils.FlowStep, consentPurposePrompt) {
	authID, executionID, step := s.signIn(location)
	return authID, executionID, step, detailsPurpose(s.T(), step)
}

// signIn follows the login redirect of an authorization request and signs the test user in. It returns the
// authorization id, the execution id and the step after sign-in.
func (s *AuthorizationCodeTestSuite) signIn(location string) (string, string, *testutils.FlowStep) {
	s.Require().NotEmpty(location, "expected a redirect to the login")
	authID, executionID, err := testutils.ExtractAuthData(location)
	s.Require().NoError(err, "unexpected redirect %s", location)

	initial, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	s.Require().NoError(err)
	step, err := testutils.ExecuteAuthenticationFlow(executionID,
		map[string]string{"username": acUsername, "password": acPassword}, "action_001", initial.ChallengeToken)
	s.Require().NoError(err)
	return authID, executionID, step
}

// decide answers the consent prompt, approving or denying every detail while approving the prompt as a
// whole, and returns the completed step.
func (s *AuthorizationCodeTestSuite) decide(executionID, challengeToken string, purpose consentPurposePrompt,
	approve bool) *testutils.FlowStep {
	return decideConsent(s.T(), executionID, challengeToken, purpose, approve)
}

// detailsPurpose requires the step to be a consent prompt and returns its authorization details purpose.
func detailsPurpose(t *testing.T, step *testutils.FlowStep) consentPurposePrompt {
	t.Helper()
	require.Equal(t, "INCOMPLETE", step.FlowStatus, "expected a consent prompt")
	require.NotNil(t, step.Data)

	raw, ok := step.Data.AdditionalData[consentPromptKey].(string)
	require.True(t, ok, "expected %q in the flow data", consentPromptKey)
	var purposes []consentPurposePrompt
	require.NoError(t, json.Unmarshal([]byte(raw), &purposes))
	for _, p := range purposes {
		if p.Type == purposeTypeDetails {
			return p
		}
	}
	require.FailNow(t, fmt.Sprintf("no authorization details purpose in %s", raw))
	return consentPurposePrompt{}
}

// decideConsent answers a consent prompt, approving or denying every detail while approving the prompt as a whole,
// and returns the completed step.
func decideConsent(t *testing.T, executionID, challengeToken string, purpose consentPurposePrompt,
	approve bool) *testutils.FlowStep {
	t.Helper()
	elements := make([]consentElementDecision, 0, len(purpose.Optional))
	for _, e := range purpose.Optional {
		elements = append(elements, consentElementDecision{Name: e.Name, Approved: approve})
	}
	decisions, err := json.Marshal(consentDecisions{Approved: true, Purposes: []consentPurposeDecision{{
		PurposeName: purpose.PurposeName, Approved: true, Elements: elements}}})
	require.NoError(t, err)

	step, err := testutils.ExecuteAuthenticationFlow(executionID,
		map[string]string{consentDecisionsInputKey: string(decisions)}, "", challengeToken)
	require.NoError(t, err)
	require.Equal(t, "COMPLETE", step.FlowStatus, "expected the flow to complete")
	require.NotEmpty(t, step.Assertion)
	return step
}

// submitConsent sends consent decisions to the flow and returns the step that follows, whatever its status.
func submitConsent(t *testing.T, executionID, challengeToken string, decisions consentDecisions) *testutils.FlowStep {
	t.Helper()
	encoded, err := json.Marshal(decisions)
	require.NoError(t, err)
	step, err := testutils.ExecuteAuthenticationFlow(executionID,
		map[string]string{consentDecisionsInputKey: string(encoded)}, "", challengeToken)
	require.NoError(t, err)
	return step
}

// failedLogin requires the step to have failed the flow with the given code, relays its error assertion to the
// authorization callback, and returns the query of the client redirect.
func failedLogin(t *testing.T, authID string, step *testutils.FlowStep, code string) url.Values {
	t.Helper()
	require.Equal(t, "ERROR", step.FlowStatus, "expected the flow to fail")
	require.NotNil(t, step.Error)
	require.Equal(t, code, step.Error.Code)
	require.NotEmpty(t, step.ErrorAssertion, "a failed login carries an error assertion")
	require.Empty(t, step.Assertion)

	authz, err := testutils.CompleteAuthorization(authID, step.ErrorAssertion)
	require.NoError(t, err)
	redirect, err := url.Parse(authz.RedirectURI)
	require.NoError(t, err)
	query := redirect.Query()
	require.Empty(t, query.Get("code"), "no code is issued for a failed login")
	return query
}

// tokenResult is a token endpoint answer with the authorization details it carries.
type tokenResult struct {
	status       int
	body         []byte
	accessToken  string
	refreshToken string
	details      []json.RawMessage
}

// exchange completes the authorization with the assertion and redeems the code, sending the given
// authorization details when not empty.
func (s *AuthorizationCodeTestSuite) exchange(authID, assertion, details string) tokenResult {
	authz, err := testutils.CompleteAuthorization(authID, assertion)
	s.Require().NoError(err)
	code, err := testutils.ExtractAuthorizationCode(authz.RedirectURI)
	s.Require().NoError(err, "expected a code in %s", authz.RedirectURI)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", acRedirectURI)
	form.Set("resource", acResource)
	if details != "" {
		form.Set("authorization_details", details)
	}
	status, body := postForm(s.T(), "/oauth2/token", form, acClientID, acClientSecret)
	return s.tokenResult(status, body)
}

// tokenResult reads a token endpoint answer.
func (s *AuthorizationCodeTestSuite) tokenResult(status int, body []byte) tokenResult {
	return readTokenResult(s.T(), status, body)
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
