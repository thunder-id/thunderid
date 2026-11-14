// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const linkedAccountEvent = "user.linked_account.create"

// Linking a federated identity to an existing user writes one line naming the linked user as actor, although
// every flow request carries the admin's token. A sign-in through the stored link, and one whose identity
// matches nobody, write none.
func (ts *AuditLogTestSuite) TestLinkingAFederatedIdentityNamesTheLinkedUserAsActor() {
	ouID := ts.createOrganizationUnit("linked")
	userType := testutils.UserType{
		Handle: ts.prefix + "linked-person", DisplayName: "Audit Linked Person", OUID: ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"email":    map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "failed to create the user type of the linked user")
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(userTypeID) })
	username := ts.prefix + "linked"
	email := username + "@example.com"
	password := credentialMarker + "Linked1!"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username, "email": email, "password": password,
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{OUID: ouID, Type: userType.Handle, Attributes: attributes})
	ts.Require().NoError(err, "failed to create the user the identity links to")
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })

	mockClientID, mockClientSecret := ts.prefix+"linked-idp-client", credentialMarker+"IdpSecret1!"
	mock, err := testutils.NewMockOIDCServer(mockClientID, mockClientSecret)
	ts.Require().NoError(err, "failed to create the mock OIDC identity provider")
	ts.T().Cleanup(func() { _ = mock.Stop() })
	linkedSub := ts.prefix + "ext-subject-linked"
	unmatchedSub := ts.prefix + "ext-subject-unmatched"
	for _, identity := range []struct{ sub, email string }{
		{linkedSub, email}, {unmatchedSub, ts.prefix + "nobody@example.com"},
	} {
		mock.AddUser(&testutils.OIDCUserInfo{
			Sub: identity.sub, Email: identity.email, EmailVerified: true, Name: "Audit Linked",
			Custom: map[string]interface{}{},
		})
	}
	activeSub := linkedSub
	mock.SetAuthorizeFunc(func(string) (string, error) { return activeSub, nil })

	idpID, err := testutils.CreateIDP(testutils.IDP{
		Name: ts.prefix + "linked-connection", Type: "OIDC", OUID: ouID,
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: mockClientID},
			{Name: "client_secret", Value: mockClientSecret, IsSecret: true},
			{Name: "redirect_uri", Value: "http://localhost:3000/callback"},
			{Name: "authorization_endpoint", Value: mock.GetAuthorizeURL()},
			{Name: "token_endpoint", Value: mock.GetTokenURL()},
			{Name: "userinfo_endpoint", Value: mock.GetUserInfoURL()},
			{Name: "jwks_endpoint", Value: mock.GetJWKSURL()},
			{Name: "scopes", Value: "openid,email,profile"},
		},
		AttributeConfiguration: &testutils.AttributeConfiguration{
			UserTypeResolution: &testutils.UserTypeResolution{Default: userType.Handle},
			UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
				UserType:   userType.Handle,
				Attributes: []testutils.AttributeMapping{{ExternalAttribute: "email", LocalAttribute: "email"}},
			}},
			AccountLinking: &testutils.AccountLinking{Attributes: []string{"email"}},
		},
	})
	ts.Require().NoError(err, "failed to create the OIDC connection")
	ts.T().Cleanup(func() { _ = testutils.DeleteIDP(idpID) })

	verifyFlowID, err := testutils.CreateFlow(common.VerifyPasswordFlow(ts.prefix + "linked-verify"))
	ts.Require().NoError(err, "failed to create the verification flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(verifyFlowID) })
	flowID, err := testutils.CreateFlow(common.VerifiedLinkingFlow(ts.prefix+"linked-flow", "OIDCAuthExecutor",
		idpID, verifyFlowID))
	ts.Require().NoError(err, "failed to create the linking flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	regFlowID, err := testutils.CreateIsolatedRegistrationFlow(ts.prefix + "linked-reg")
	ts.Require().NoError(err, "failed to create the registration flow of the application")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(regFlowID) })
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: ts.prefix + "linked-app", OUID: ouID, AuthFlowID: flowID, RegistrationFlowID: regFlowID,
		ClientID: ts.prefix + "linked-client", ClientSecret: credentialMarker + "ClientSecret1!",
		RedirectURIs: []string{"http://localhost:3000/callback"}, AllowedUserTypes: []string{userType.Handle},
	})
	ts.Require().NoError(err, "failed to create the application that runs the linking flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	userRef := testutils.AuditRef{Type: "user", ID: userID}
	ts.Require().NotEqual(adminRef, userRef, "the bearer token must belong to someone other than the linked user")

	// signIn starts the flow and returns the start's and the federated callback's responses, and the step
	// the callback got.
	signIn := func() ([]*http.Response, *common.FlowStep) {
		started, step := ts.executeFlow(adminToken, appID, map[string]interface{}{
			"applicationId": appID, "flowType": "AUTHENTICATION",
		})
		ts.Require().Equal("REDIRECTION", step.Type, "the flow must redirect to the identity provider")
		code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
		ts.Require().NoError(err, "failed to authorize at the mock identity provider")
		callback, step := ts.executeFlowStep(adminToken, appID, map[string]interface{}{
			"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
			"inputs": map[string]string{"code": code, "state": state},
		})
		return []*http.Response{started, callback}, step
	}

	linking, step := signIn()
	ts.Require().NotEmpty(step.Data.AdditionalData["linkingPromptDetails"],
		"the identity must match the user and get the linking prompt: %+v", step)
	confirmed, confirmStep := ts.executeFlow(adminToken, appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken, "action": common.LinkAccountAction,
	})
	linked, linkedStep := ts.executeFlow(adminToken, appID, map[string]interface{}{
		"executionId": confirmStep.ExecutionID, "challengeToken": confirmStep.ChallengeToken,
		"inputs": map[string]string{"username": username, "password": password}, "action": common.VerifyPasswordAction,
	})
	ts.Require().Equal("COMPLETE", linkedStep.FlowStatus, "verifying the matched user must link the identity")
	claims, err := testutils.DecodeJWT(linkedStep.Assertion)
	ts.Require().NoError(err)
	ts.Require().Equal(userID, claims.Sub, "the flow must complete as the linked user")
	linking = append(linking, confirmed, linked)

	resigned, resignedStep := signIn()
	ts.Require().Equal("COMPLETE", resignedStep.FlowStatus,
		"a linked identity must resolve through the link: %+v", resignedStep)

	activeSub = unmatchedSub
	unmatched, unmatchedStep := signIn()
	ts.Require().Empty(unmatchedStep.Data.AdditionalData["linkingPromptDetails"],
		"an identity that matches no user must get no linking prompt: %+v", unmatchedStep)
	// The federated node completes for an identity with no local user, so this refusal comes from the
	// linking node or the assertion after it, and the sign-in reached the linking node without a match.
	ts.Require().Equal("ERROR", unmatchedStep.FlowStatus, "%+v", unmatchedStep)
	ts.Require().NotNil(unmatchedStep.Error, "%+v", unmatchedStep)
	ts.Require().Equal("FET-1002", unmatchedStep.Error.Code, "an identity that matches no user is not identified")

	perCall := ts.linesPerCall(append(append(linking, resigned...), unmatched...)...)
	var linkingLines []testutils.AuditLine
	for _, written := range perCall[:len(linking)] {
		linkingLines = append(linkingLines, written...)
	}
	line := ts.onlyLineOf(linkingLines, linkedAccountEvent, "linking a federated identity")
	ts.Equal(linked.Header.Get(correlationIDHeader), line.TraceID,
		"the link is stored by the call that verifies the matched user")
	ts.Equal("create", line.Action)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
	ts.Equal(userRef, line.Actor, "the linked user is the actor, not the owner of the bearer token")
	ts.Equal(userRef, line.Target)
	ts.Equal(userRef, line.Subject)
	ts.Equal(map[string]string{"connection": idpID}, ts.details(line),
		"the connection is the only detail of a link")
	ts.NotContains(line.Raw, linkedSub, "the identity's subject at the provider is never recorded")
	ts.NotContains(line.Raw, email, "the attribute the identity matched on is never recorded")
	ts.NotContains(line.Raw, username, "the linked user is named by ID, never by username")
	ts.NotContains(line.Raw, credentialMarker, "no part of a credential is recorded")
	ts.NotContains(line.Raw, adminToken, "the bearer token is never recorded")
	ts.NotContains(line.Raw, linkedStep.Assertion, "the issued assertion is never recorded")

	for _, written := range perCall[len(linking):] {
		for _, other := range written {
			ts.NotEqual(linkedAccountEvent, other.Event,
				"a sign-in that stores no link, through a stored link or matching nobody, writes no %s line",
				linkedAccountEvent)
		}
	}
}
