// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	codeDetailsRefused       = "FET-1127"
	codeDetailsUnavailable   = "FET-1128"
	codeDetailsChangeRefused = "FET-1129"
	codeDetailsNotChecked    = "FET-1130"

	errorAccessDenied = "access_denied"

	acUncheckedClientID     = "authz_details_ac_unchecked_client"
	acUncheckedClientSecret = "authz_details_ac_unchecked_secret"
	acNoConsentClientID     = "authz_details_ac_no_consent_client"
	acNoConsentClientSecret = "authz_details_ac_no_consent_secret"
)

// tripSchema is a type schema with lists and a nested object the user may narrow on the consent screen.
var tripSchema = json.RawMessage(`{"type":"object","required":["route"],"properties":{` +
	`"route":{"type":"string","title":"Route"},` +
	`"travellers":{"type":"array","items":{"type":"string"},` +
	`"policy":{"modification":"removeItems","modifiableBy":["user"]}},` +
	`"excludedAirlines":{"type":"array","items":{"type":"string","enum":["XA","XB","XC"]},` +
	`"policy":{"modification":"addItems","modifiableBy":["user"]}},` +
	`"fares":{"type":"array","items":{"type":"number"}},` +
	`"budget":{"type":"object","properties":{"currency":{"type":"string"},` +
	`"limit":{"type":"number","minimum":0,"policy":{"modification":"decrease","modifiableBy":["user"]}}}}}}`)

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
	flowID, err := testutils.CreateFlow(detailsFlowWithout("Authorization Details Unchecked Flow",
		"authz_details_unchecked_flow", "authorization_details"))
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteFlow(flowID) }()

	appID := s.createDetailsApp("Authorization Details Unchecked App", acUncheckedClientID, acUncheckedClientSecret,
		flowID)
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

// The consent prompt describes each field of a detail, with the changes the user may make, and summarizes the
// detail from its type's template only when every placeholder has a value. The changes the user makes are issued.
func (s *AuthorizationCodeTestSuite) TestConsentPromptDescribesEachFieldOfADetail() {
	tripType := uniqueTypeValue("ac-trip")
	trip := createType(s.T(), typeRequest{Type: tripType, Name: "Trip", Schema: tripSchema,
		ConsentTemplate: "Book {{route}} for {{travellers}} at {{fares}} within {{budget}}"})
	defer deleteType(s.T(), trip.ID)
	plainType := uniqueTypeValue("ac-plain")
	plain := createType(s.T(), typeRequest{Type: plainType, Name: "Plain", Schema: flightSchema})
	defer deleteType(s.T(), plain.ID)

	full := `{"type":"` + tripType + `","route":"CMB-SIN","travellers":["Ann","Ben"],"excludedAirlines":["XA"],` +
		`"fares":[120.5,99],"budget":{"currency":"EUR","limit":500}}`
	partial := `{"type":"` + tripType + `","route":"CMB-LHR","travellers":["Ann"]}`
	other := `{"type":"` + plainType + `","route":"CMB-DXB","maxFare":300}`
	authID, executionID, step, purpose := s.loginExpectingDetailsPrompt(`[` + full + `,` + partial + `,` + other + `]`)
	s.Require().Len(purpose.Optional, 3)

	summary := purpose.Optional[0].Summary
	prefix := "Book CMB-SIN for Ann, Ben at 120.5, 99 within "
	s.Require().True(strings.HasPrefix(summary, prefix), summary)
	s.JSONEq(`{"currency":"EUR","limit":500}`, strings.TrimPrefix(summary, prefix), "an object is shown as JSON")
	s.Empty(purpose.Optional[1].Summary, "a template with a placeholder that has no value is not shown")
	s.Empty(purpose.Optional[2].Summary, "a type without a template has no summary")

	rows := map[string]consentDetailRow{}
	for _, row := range purpose.Optional[0].Rows {
		rows[row.Path] = row
	}
	s.Equal("Route", rows["route"].Label, "a field is labelled with its title")
	s.Equal("removeItems", rows["travellers"].Modification)
	s.Equal("addItems", rows["excludedAirlines"].Modification)
	s.JSONEq(`["XA","XB","XC"]`, string(rows["excludedAirlines"].Options), "the items the user may add are listed")
	s.Empty(rows["fares"].Modification, "a field without a policy is fixed")
	s.Equal("budget.currency", rows["budget.currency"].Label, "a field without a title is labelled with its path")
	s.Equal("decrease", rows["budget.limit"].Modification)
	s.Equal("number", rows["budget.limit"].As)

	elements := make([]consentElementDecision, 0, len(purpose.Optional))
	for _, e := range purpose.Optional {
		elements = append(elements, consentElementDecision{Name: e.Name, Approved: true})
	}
	elements[0].Modified = map[string]json.RawMessage{
		"travellers":       json.RawMessage(`["Ann"]`),
		"excludedAirlines": json.RawMessage(`["XA","XC"]`),
		"budget.limit":     json.RawMessage(`300`),
	}
	final := submitConsent(s.T(), executionID, step.ChallengeToken, consentDecisions{Approved: true,
		Purposes: []consentPurposeDecision{{PurposeName: purpose.PurposeName, Approved: true, Elements: elements}}})
	s.Require().Equal("COMPLETE", final.FlowStatus, "changes within the policies are accepted")

	token := s.exchange(authID, final.Assertion, "")
	s.Require().Equal(http.StatusOK, token.status, string(token.body))
	narrowed := `{"type":"` + tripType + `","route":"CMB-SIN","travellers":["Ann"],"excludedAirlines":["XA","XC"],` +
		`"fares":[120.5,99],"budget":{"currency":"EUR","limit":300}}`
	s.JSONEq(`[`+narrowed+`,`+partial+`,`+other+`]`, rawJSON(s.T(), token.details),
		"the details are issued as the user changed them")
}

// A flow that never asks for consent records no decision on the requested details, so the login is refused
// rather than issuing details nobody approved.
func (s *AuthorizationCodeTestSuite) TestFlowWithoutConsentDeniesTheDetails() {
	flowID, err := testutils.CreateFlow(detailsFlowWithout("Authorization Details No Consent Flow",
		"authz_details_no_consent_flow", "authorization_details", "consent"))
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteFlow(flowID) }()
	appID := s.createDetailsApp("Authorization Details No Consent App", acNoConsentClientID,
		acNoConsentClientSecret, flowID)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	authID, _, step := s.login(acNoConsentClientID, `[{"type":"`+s.flightType+`","route":"CMB-SIN"}]`, nil)
	s.Require().Equal("COMPLETE", step.FlowStatus)
	authz, err := testutils.CompleteAuthorization(authID, step.Assertion)
	s.Require().NoError(err)
	redirect, err := url.Parse(authz.RedirectURI)
	s.Require().NoError(err)
	s.Equal(errorAccessDenied, redirect.Query().Get("error"))
	s.Empty(redirect.Query().Get("code"), "no code is issued for details nobody approved")
}

// createDetailsApp registers an application of the test user type that signs in through the given flow.
func (s *AuthorizationCodeTestSuite) createDetailsApp(name, clientID, clientSecret, flowID string) string {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             name,
		Type:             "fullstack",
		OUID:             s.ouID,
		AuthFlowID:       flowID,
		AllowedUserTypes: []string{acUserType},
		LoginConsent:     map[string]interface{}{"validityPeriod": 3600},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"clientSecret":            clientSecret,
				"redirectUris":            []string{acRedirectURI},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"scopes":                  []string{"openid"},
			},
		}},
	})
	s.Require().NoError(err)
	return appID
}

// modifiedDecision approves the prompt and its single detail with the given changes.
func modifiedDecision(purpose consentPurposePrompt, modified map[string]json.RawMessage) consentDecisions {
	return consentDecisions{Approved: true, Purposes: []consentPurposeDecision{{
		PurposeName: purpose.PurposeName, Approved: true, Elements: []consentElementDecision{{
			Name: purpose.Optional[0].Name, Approved: true, Modified: modified}}}}}
}

// detailsFlowWithout is the details flow without the given nodes, each node going on to the next one kept.
func detailsFlowWithout(name, handle string, skipped ...string) testutils.Flow {
	source := detailsFlow.Nodes.([]map[string]interface{})
	nodes := make([]map[string]interface{}, 0, len(source))
	for _, node := range source {
		if slices.Contains(skipped, node["id"].(string)) {
			continue
		}
		if len(nodes) > 0 {
			if previous := nodes[len(nodes)-1]; previous["onSuccess"] != nil && previous["onSuccess"] != node["id"] {
				copied := maps.Clone(previous)
				copied["onSuccess"] = node["id"]
				nodes[len(nodes)-1] = copied
			}
		}
		nodes = append(nodes, node)
	}
	return testutils.Flow{Name: name, FlowType: "AUTHENTICATION", Handle: handle, Nodes: nodes}
}
