// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	auditConsentPassword = "AuditConsent123!"
	auditConsentEmail    = "audit-consent-marker@example.com"
	auditConsentAttr     = "email"

	consentPromptKey   = "consentPrompt"
	consentDecisionKey = "consent_decisions"
)

// consentIDPattern is the shape of the ID the consent service assigns.
var consentIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// auditConsentFlowNodes sign the user in with a password and ask for consent before the assertion.
var auditConsentFlowNodes = []map[string]interface{}{
	{"id": "start", "type": "START", "onSuccess": "credentials_auth"},
	{
		"id": "credentials_auth", "type": "TASK_EXECUTION",
		"executor": map[string]interface{}{"name": "CredentialsAuthExecutor"}, "onSuccess": "consent",
	},
	{
		"id": "consent", "type": "TASK_EXECUTION",
		"executor": map[string]interface{}{"name": "ConsentExecutor"}, "onSuccess": "auth_assert",
	},
	{
		"id": "auth_assert", "type": "TASK_EXECUTION",
		"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end",
	},
	{"id": "end", "type": "END"},
}

// consentPromptPurpose is one purpose of a consent prompt.
type consentPromptPurpose struct {
	PurposeName string `json:"purposeName"`
	Type        string `json:"type"`
	Essential   []struct {
		Name string `json:"name"`
	} `json:"essential"`
	Optional []struct {
		Name string `json:"name"`
	} `json:"optional"`
}

// A sign-in that records the user's first consent decisions writes consent.create, and a later sign-in
// that changes them writes consent.update for the same consent. Both have the user who decided as actor
// and subject, the consent as target, and record no purpose, element or attribute value. Submitting the
// credentials, and a sign-in that reuses the consent, write no consent line.
//
// Sign-in runs on a public path that accepts any caller's bearer token. The flow requests here carry the
// admin's token, so the line naming the user rather than the admin shows the actor comes from the
// consent and not from the request's subject.
func (ts *AuditLogTestSuite) TestASignInThatRecordsConsentWritesItsEvent() {
	appID, userID, username := ts.createConsentSignIn()
	userRef := testutils.AuditRef{Type: "user", ID: userID}
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	ts.Require().NotEqual(adminRef, userRef, "the owner of the bearer token is not the user who signs in")

	// The user denies the attribute, which is recorded as a new consent; a denial is re-prompted, so the
	// next sign-in can grant it, which changes the same consent.
	prompted, denyStep := ts.signInToConsentPrompt(adminToken, appID, username)
	denied, deniedStep := ts.submitConsent(adminToken, appID, denyStep, false)
	ts.Require().Equal("COMPLETE", deniedStep.FlowStatus, "denying an optional attribute completes the sign-in")
	reprompted, grantStep := ts.signInToConsentPrompt(adminToken, appID, username)
	granted, grantedStep := ts.submitConsent(adminToken, appID, grantStep, true)
	ts.Require().Equal("COMPLETE", grantedStep.FlowStatus, "granting the attribute completes the sign-in")
	// The consent now covers everything asked for, so this sign-in completes without a prompt.
	reused, reusedStep := ts.executeFlow(adminToken, appID, consentSignInRequest(appID, username))
	ts.Require().Equal("COMPLETE", reusedStep.FlowStatus, "an active consent is reused without a prompt")

	perCall := ts.linesPerCall(prompted, denied, reprompted, granted, reused)
	for _, tc := range []struct {
		name  string
		lines []testutils.AuditLine
	}{
		{"the first credentials", perCall[0]},
		{"the second credentials", perCall[2]},
		{"the reusing sign-in", perCall[4]},
	} {
		ts.Zero(countEvent(tc.lines, "consent.create")+countEvent(tc.lines, "consent.update"),
			"%s write no consent line, since no consent decision was recorded", tc.name)
	}
	created := ts.onlyLineOf(perCall[1], "consent.create", "the denial")
	updated := ts.onlyLineOf(perCall[3], "consent.update", "the grant")
	ts.Regexp(consentIDPattern, created.Target.ID, "the target is the ID of the consent that was recorded")
	ts.Equal(created.Target, updated.Target, "the update changed the consent the create recorded")
	for _, tc := range []struct {
		line   testutils.AuditLine
		event  string
		action string
	}{
		{created, "consent.create", "create"},
		{updated, "consent.update", "update"},
	} {
		ts.Equal(tc.event, tc.line.Event)
		ts.Equal(tc.action, tc.line.Action, tc.event)
		ts.Equal("consent", tc.line.Target.Type, tc.event)
		ts.Equal(userRef, tc.line.Subject, "%s: the user who decided is the subject", tc.event)
		ts.Equal(userRef, tc.line.Actor, "%s: the user who decided is the actor", tc.event)
		ts.NotEqual(adminRef, tc.line.Actor, "%s: the actor is not the owner of the bearer token", tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, tc.line.Outcome, tc.event)
		ts.Nil(tc.line.Client, "%s: a loopback peer is not recorded", tc.event)
		ts.NotContains(tc.line.Raw, appID, "%s: the application the consent is for is not recorded", tc.event)
		ts.NotContains(tc.line.Raw, auditConsentAttr, "%s: no consent element is recorded", tc.event)
		ts.NotContains(tc.line.Raw, auditConsentEmail, "%s: no attribute value is recorded", tc.event)
		ts.NotContains(tc.line.Raw, username, "%s: the identifier typed at sign-in is not recorded", tc.event)
		ts.NotContains(tc.line.Raw, auditConsentPassword, "%s: no credential is recorded", tc.event)
	}
}

// createConsentSignIn creates an application that asks for consent to an attribute before it signs a
// user in, and a user who has that attribute, all removed when the test ends. It returns the
// application's ID and the user's ID and username.
func (ts *AuditLogTestSuite) createConsentSignIn() (string, string, string) {
	ts.T().Helper()
	ouID := ts.createOrganizationUnit("consent")
	userType := testutils.UserType{
		Handle:      ts.prefix + "consent-person",
		DisplayName: "Audit Consent Person",
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username":       map[string]interface{}{"type": "string"},
			"password":       map[string]interface{}{"type": "string", "credential": true},
			auditConsentAttr: map[string]interface{}{"type": "string"},
		},
	}
	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "failed to create the consent user type")
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(userTypeID) })

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Audit Consent Flow",
		FlowType: "AUTHENTICATION",
		Handle:   ts.prefix + "consent-flow",
		Nodes:    auditConsentFlowNodes,
	})
	ts.Require().NoError(err, "failed to create the consent flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             ts.prefix + "consent-app",
		OUID:             ouID,
		ClientID:         ts.prefix + "consent-client",
		ClientSecret:     "AuditConsentClientSecret0123456789",
		RedirectURIs:     []string{"https://audit-consent.example.com/callback"},
		AllowedUserTypes: []string{userType.Handle},
		AuthFlowID:       flowID,
		AssertionConfig:  map[string]interface{}{"userAttributes": []string{auditConsentAttr}},
		LoginConsent:     map[string]interface{}{"validityPeriod": 3600},
	})
	ts.Require().NoError(err, "failed to create the consent application")
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })

	username := ts.prefix + "consent-user"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username, "password": auditConsentPassword, auditConsentAttr: auditConsentEmail,
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{OUID: ouID, Type: userType.Handle, Attributes: attributes})
	ts.Require().NoError(err, "failed to create the consent user")
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })
	return appID, userID, username
}

// signInToConsentPrompt submits the user's credentials and requires the flow to stop at a consent prompt
// for the attribute.
func (ts *AuditLogTestSuite) signInToConsentPrompt(token, appID, username string) (
	*http.Response, *common.FlowStep) {
	ts.T().Helper()
	resp, step := ts.executeFlow(token, appID, consentSignInRequest(appID, username))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the sign-in stops at a consent prompt")
	ts.Require().NotEmpty(step.Data.AdditionalData[consentPromptKey], "the prompt carries its purposes")
	return resp, step
}

// submitConsent answers the attribute purpose of a consent prompt, granting or denying every element.
func (ts *AuditLogTestSuite) submitConsent(token, appID string, step *common.FlowStep, approve bool) (
	*http.Response, *common.FlowStep) {
	ts.T().Helper()
	var purposes []consentPromptPurpose
	ts.Require().NoError(json.Unmarshal([]byte(step.Data.AdditionalData[consentPromptKey]), &purposes))
	type elementDecision struct {
		Name     string `json:"name"`
		Approved bool   `json:"approved"`
	}
	type purposeDecision struct {
		PurposeName string            `json:"purposeName"`
		Approved    bool              `json:"approved"`
		Elements    []elementDecision `json:"elements"`
	}
	decided := make([]purposeDecision, 0, len(purposes))
	for _, purpose := range purposes {
		ts.Require().Equal("attributes", purpose.Type, "the application asks for consent to attributes only")
		elements := make([]elementDecision, 0, len(purpose.Essential)+len(purpose.Optional))
		for _, element := range append(purpose.Essential, purpose.Optional...) {
			elements = append(elements, elementDecision{Name: element.Name, Approved: approve})
		}
		decided = append(decided, purposeDecision{PurposeName: purpose.PurposeName, Approved: approve,
			Elements: elements})
	}
	ts.Require().NotEmpty(decided, "the prompt asks about a purpose")
	decisions, err := json.Marshal(map[string]interface{}{"approved": approve, "purposes": decided})
	ts.Require().NoError(err)
	return ts.executeFlow(token, appID, map[string]interface{}{
		"executionId":    step.ExecutionID,
		"challengeToken": step.ChallengeToken,
		"inputs":         map[string]string{consentDecisionKey: string(decisions)},
	})
}

// consentSignInRequest starts the application's sign-in flow with the user's credentials.
func consentSignInRequest(appID, username string) map[string]interface{} {
	return map[string]interface{}{
		"applicationId": appID,
		"flowType":      "AUTHENTICATION",
		"inputs":        map[string]string{"username": username, "password": auditConsentPassword},
	}
}
