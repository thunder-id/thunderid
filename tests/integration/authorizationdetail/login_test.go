// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	codeDetailsRefused       = "FET-1112"
	codeDetailsUnavailable   = "FET-1113"
	codeDetailsChangeRefused = "FET-1114"
	codeDetailsNotChecked    = "FET-1115"

	errorAccessDenied = "access_denied"

	acUncheckedClientID     = "authz_details_ac_unchecked_client"
	acUncheckedClientSecret = "authz_details_ac_unchecked_secret"
)

// A user may narrow a detail on the consent screen as the field's policy allows, and the narrowed value is
// what is issued.
func (s *AuthorizationCodeTestSuite) TestUserNarrowsADetailWithinPolicy() {
	details := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}]`
	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)
	s.Require().Len(purpose.Optional, 1)

	final := submitConsent(s.T(), executionID, step.ChallengeToken,
		modifiedDecision(purpose, map[string]json.RawMessage{"maxFare": json.RawMessage(`300`)}))
	s.Require().Equal("COMPLETE", final.FlowStatus, "a change within the policy is accepted")

	token := s.exchange(authID, final.Assertion, "")
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	expected := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":300}]`
	s.JSONEq(expected, rawJSON(s.T(), token.details), "the narrowed detail is issued")
	assertTokenDetails(s.T(), token.accessToken, expected)
}

// A change on the consent screen that the field's policy does not allow the user fails the login, and the
// client is denied.
func (s *AuthorizationCodeTestSuite) TestUserChangeOutsidePolicyFailsTheLogin() {
	for name, change := range map[string]map[string]json.RawMessage{
		"a widened field":          {"maxFare": json.RawMessage(`500`)},
		"a field without a policy": {"route": json.RawMessage(`"CMB-LHR"`)},
	} {
		details := `[{"type":"` + s.flightType + `","route":"CMB-SIN","maxFare":400}]`
		authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)

		final := submitConsent(s.T(), executionID, step.ChallengeToken, modifiedDecision(purpose, change))
		query := failedLogin(s.T(), authID, final, codeDetailsChangeRefused)
		s.Equal(errorAccessDenied, query.Get("error"), name)
	}
}

// When the processor refuses every detail of a login, the login fails before the user is asked, and the client
// is denied with the processor's reason.
func (s *AuthorizationCodeTestSuite) TestProcessorRefusingEveryDetailDeniesTheLogin() {
	details := `[{"type":"` + s.paymentType + `","amount":950}]`
	authID, _, step := s.login(acClientID, details, nil)

	query := failedLogin(s.T(), authID, step, codeDetailsRefused)
	s.Contains(step.Error.Description.DefaultValue, processorRefusal, "the user is told the processor's reason")
	s.Equal(errorAccessDenied, query.Get("error"))
	s.Contains(query.Get("error_description"), processorRefusal, "the client is told the processor's reason")
}

// A detail the processor refuses is left out of the prompt and the token, and the others go on.
func (s *AuthorizationCodeTestSuite) TestPartlyRefusedDetailsLeaveTheRefusedOneOut() {
	details := `[{"type":"` + s.paymentType + `","amount":45},{"type":"` + s.paymentType + `","amount":950}]`
	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(details)
	s.Require().Len(purpose.Optional, 1, "only the admitted detail is prompted")
	s.Equal("Pay 45", purpose.Optional[0].Summary)

	final := s.decide(executionID, step.ChallengeToken, purpose, true)
	token := s.exchange(authID, final.Assertion, "")
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	expected := `[{"type":"` + s.paymentType + `","amount":45,"creditorAccount":"` + processorAccount + `"}]`
	s.JSONEq(expected, rawJSON(s.T(), token.details), "the refused detail is left out")
	assertTokenDetails(s.T(), token.accessToken, expected)
}

// A processor that cannot be reached or gives no usable answer fails the login, and the client is denied.
func (s *AuthorizationCodeTestSuite) TestUnusableProcessorFailsTheLogin() {
	endpoints := map[string]string{"an unreachable processor": unreachableEndpoint()}
	for name, answer := range unusableAnswers {
		server := newStaticProcessor(answer.status, answer.body)
		defer server.Close()
		endpoints[name] = server.URL + "/process"
	}
	for name, endpoint := range endpoints {
		value := uniqueTypeValue("ac-unusable")
		created := createType(s.T(), typeRequest{Type: value, Name: "Unusable", Schema: paymentSchema,
			Processor: &processorConfig{Endpoint: endpoint}})

		authID, _, step := s.login(acClientID, `[{"type":"`+value+`","amount":45}]`, nil)
		query := failedLogin(s.T(), authID, step, codeDetailsUnavailable)
		s.Equal(errorAccessDenied, query.Get("error"), name)
		deleteType(s.T(), created.ID)
	}
}

// A flow that asks for consent without running the authorization detail executor cannot grant a detail whose
// type declares a processor.
func (s *AuthorizationCodeTestSuite) TestFlowWithoutTheDetailExecutorDeniesProcessedTypes() {
	flow := uncheckedDetailsFlow()
	flowID, err := testutils.CreateFlow(flow)
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteFlow(flowID) }()

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             "Authorization Details Unchecked App",
		Type:             "fullstack",
		OUID:             s.ouID,
		AuthFlowID:       flowID,
		AllowedUserTypes: []string{acUserType},
		LoginConsent:     map[string]interface{}{"validityPeriod": 3600},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                acUncheckedClientID,
				"clientSecret":            acUncheckedClientSecret,
				"redirectUris":            []string{acRedirectURI},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"scopes":                  []string{"openid"},
			},
		}},
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	authID, _, step := s.login(acUncheckedClientID, `[{"type":"`+s.paymentType+`","amount":45}]`, nil)
	query := failedLogin(s.T(), authID, step, codeDetailsNotChecked)
	s.Equal(errorAccessDenied, query.Get("error"))
	_, called := s.processor.callFor(s.paymentType)
	s.False(called, "the processor is never called")
}

// An approval of a type that allows reuse covers a later request that stays within it, which is granted
// without a prompt; a request beyond it, or one with prompt=consent, is prompted.
func (s *AuthorizationCodeTestSuite) TestApprovalIsReusedUntilPromptConsent() {
	value := uniqueTypeValue("ac-reuse")
	reusable := createType(s.T(), typeRequest{Type: value, Name: "Reusable", Schema: flightSchema,
		ConsentTemplate: "Book {{route}}", Reuse: true})
	defer deleteType(s.T(), reusable.ID)

	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(
		`[{"type":"` + value + `","route":"CMB-SIN","maxFare":400}]`)
	final := s.decide(executionID, step.ChallengeToken, purpose, true)
	token := s.exchange(authID, final.Assertion, "")
	s.Require().Equal(http.StatusOK, token.status, string(token.body))

	covered := `[{"type":"` + value + `","route":"CMB-SIN","maxFare":300}]`
	authID, _, step = s.login(acClientID, covered, nil)
	s.Require().Equal("COMPLETE", step.FlowStatus, "a request within the approval is not prompted")
	s.Require().NotEmpty(step.Assertion)
	token = s.exchange(authID, step.Assertion, "")
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	s.JSONEq(covered, rawJSON(s.T(), token.details), "the covered detail is granted as requested")

	_, _, step = s.login(acClientID, `[{"type":"`+value+`","route":"CMB-SIN","maxFare":500}]`, nil)
	s.Len(detailsPurpose(s.T(), step).Optional, 1, "a request beyond the approval is prompted")

	_, _, step = s.login(acClientID, covered, map[string]string{"prompt": "consent"})
	s.Len(detailsPurpose(s.T(), step).Optional, 1, "prompt=consent prompts even for a covered detail")
}

// modifiedDecision approves the prompt and its single detail with the given changes.
func modifiedDecision(purpose consentPurposePrompt, modified map[string]json.RawMessage) consentDecisions {
	return consentDecisions{Approved: true, Purposes: []consentPurposeDecision{{
		PurposeName: purpose.PurposeName, Approved: true, Elements: []consentElementDecision{{
			Name: purpose.Optional[0].Name, Approved: true, Modified: modified}}}}}
}

// uncheckedDetailsFlow authenticates the user and asks for consent without running the type processors.
func uncheckedDetailsFlow() testutils.Flow {
	source := detailsFlow.Nodes.([]map[string]interface{})
	nodes := make([]map[string]interface{}, 0, len(source))
	for _, node := range source {
		switch node["id"] {
		case "authorization_details":
			continue
		case "credentials_auth":
			copied := make(map[string]interface{}, len(node))
			for key, value := range node {
				copied[key] = value
			}
			copied["onSuccess"] = "consent"
			node = copied
		}
		nodes = append(nodes, node)
	}
	return testutils.Flow{
		Name:     "Authorization Details Unchecked Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "authz_details_unchecked_flow",
		Nodes:    nodes,
	}
}
