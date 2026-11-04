// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// ouBoundaryMockGooglePort must be 8093, the Google endpoint testutils.GoogleMockBaseURL points the server at.
	ouBoundaryMockGooglePort = 8093
	ouBoundaryGoogleClientID = "ou_boundary_google_client"
	ouBoundaryGoogleSecret   = "ou_boundary_google_secret"
	ouBoundaryGoogleExecutor = "GoogleOIDCAuthExecutor"
)

// ouBoundaryGoogleFlow returns a sign-in flow that signs a Google identity in as the local user its link names.
func ouBoundaryGoogleFlow(handle, idpID string) testutils.Flow {
	return testutils.Flow{
		Name:     "OU Boundary " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "google_auth"},
			{
				"id":         "google_auth",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"idpId": idpID},
				"executor":   map[string]interface{}{"name": ouBoundaryGoogleExecutor},
				"onSuccess":  "auth_assert",
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
}

// setUpLinkedAccountFixture starts the mock Google, links on email, and creates the applications the linked account
// scenarios sign in through: a sign-in and a linking application in the parent shared with its children, and a JIT
// application unshared in the child.
func (ts *OUBoundedSignInTestSuite) setUpLinkedAccountFixture() {
	mock, err := testutils.NewMockGoogleOIDCServer(ouBoundaryMockGooglePort, ouBoundaryGoogleClientID,
		ouBoundaryGoogleSecret)
	ts.Require().NoError(err, "create the mock Google server")
	ts.mockGoogle = mock
	ts.Require().NoError(ts.mockGoogle.Start(), "start the mock Google server")

	ts.googleIDPID, err = testutils.CreateIDP(testutils.IDP{
		Name:        "OU Boundary Google",
		Description: "Google connection for the OU-bounded linked account scenarios",
		Type:        "GOOGLE",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: ouBoundaryGoogleClientID},
			{Name: "client_secret", Value: ouBoundaryGoogleSecret, IsSecret: true},
			{Name: "scopes", Value: "openid email profile"},
			{Name: "redirect_uri", Value: "http://localhost:3000/callback"},
		},
		AttributeConfiguration: &testutils.AttributeConfiguration{
			AccountLinking: &testutils.AccountLinking{Attributes: []string{"email"}},
		},
	})
	ts.Require().NoError(err, "create the Google connection")

	// The only self-registration type the JIT application allows, so provisioning has one type to use.
	ts.federatedType = ts.createUserType("ou-boundary-federated-person", ts.parentOUID, fedProvSchema)
	types := []string{ts.federatedType}
	assertion := map[string]interface{}{"userAttributes": []string{"ouId"}}

	ts.googleSharedApp = ts.createNativeApp("OU Boundary Google Shared", "ou_boundary_google_shared",
		ts.parentOUID, ts.createFlow(ouBoundaryGoogleFlow("ou_boundary_google", ts.googleIDPID)), types, assertion)

	verifyFlowID := ts.createFlow(common.VerifyPasswordFlow("ou_boundary_google_verify"))
	ts.googleLinkingApp = ts.createNativeApp("OU Boundary Google Linking", "ou_boundary_google_linking",
		ts.parentOUID, ts.createFlow(common.VerifiedLinkingFlow("ou_boundary_google_linking",
			ouBoundaryGoogleExecutor, ts.googleIDPID, verifyFlowID)), types, assertion)

	for _, appID := range []string{ts.googleSharedApp, ts.googleLinkingApp} {
		ts.Require().NoError(testutils.ShareApplicationWithChildUnits(appID), "share application %s", appID)
	}

	ts.googleJITChildApp = ts.createNativeApp("OU Boundary Google JIT Child", "ou_boundary_google_jit_child",
		ts.childOUID, ts.createFlow(buildFederatedProvisioningFlow("ou_boundary_google_jit", ts.googleIDPID, nil, "")),
		types, assertion)
}

// tearDownLinkedAccountFixture removes the Google connection, once the flows naming it are gone, and stops the mock.
func (ts *OUBoundedSignInTestSuite) tearDownLinkedAccountFixture() {
	if ts.googleIDPID != "" {
		if err := testutils.DeleteIDP(ts.googleIDPID); err != nil {
			ts.T().Logf("teardown: delete the Google connection: %v", err)
		}
	}
	if ts.mockGoogle != nil {
		if err := ts.mockGoogle.Stop(); err != nil {
			ts.T().Logf("teardown: stop the mock Google server: %v", err)
		}
	}
}

// createFederatedUser creates a user of the federated type living in ouID, whose email is username@ou-boundary.test.
func (ts *OUBoundedSignInTestSuite) createFederatedUser(ouID, username string) string {
	ts.T().Helper()
	id := ts.createUser(ts.federatedType, ouID, username)
	ts.federatedUserIDs = append(ts.federatedUserIDs, id)
	return id
}

// useGoogleIdentity makes the mock Google authorize the identity with this subject and email.
func (ts *OUBoundedSignInTestSuite) useGoogleIdentity(sub, email string) {
	ts.mockGoogle.AddUser(&testutils.GoogleUserInfo{
		Sub: sub, Email: email, EmailVerified: true, Name: "OU Boundary", GivenName: "OU", FamilyName: "Boundary",
	})
	// The mock calls this on its own handler goroutine, so it captures the email rather than reading suite state.
	ts.mockGoogle.SetAuthorizeFunc(func(string) (string, error) { return email, nil })
}

// linkGoogleIdentity links a Google identity to the user of ouID with that username, proven with its password.
func (ts *OUBoundedSignInTestSuite) linkGoogleIdentity(handle, ouID, username, sub string) {
	ts.T().Helper()
	ts.useGoogleIdentity(sub, username+"@ou-boundary.test")
	ts.Require().NoError(common.LinkAccount(common.LinkRequest{
		Handle:       handle,
		ExecutorName: ouBoundaryGoogleExecutor,
		IDPID:        ts.googleIDPID,
		OUID:         ouID,
		UserType:     ts.federatedType,
		Username:     username,
		Password:     ouBoundaryPassword,
	}), "link the Google identity to %s", username)
}

// googleSignIn signs a Google identity in to the application, bounded to ouID when it is set.
func (ts *OUBoundedSignInTestSuite) googleSignIn(appID, ouID, sub, email string) *common.FlowStep {
	ts.T().Helper()
	ts.useGoogleIdentity(sub, email)
	status, raw := ts.startSignIn(appID, ouID, nil)
	ts.Require().Equal(http.StatusOK, status, "start the sign-in: %s", string(raw))
	var step common.FlowStep
	ts.Require().NoError(json.Unmarshal(raw, &step))
	ts.Require().Equal("REDIRECTION", step.Type, "expected a redirection to Google: %s", string(raw))

	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	ts.Require().NoError(err, "authorize at the mock Google")
	next, err := common.CompleteFlow(step.ExecutionID, map[string]string{"code": code, "state": state}, "",
		step.ChallengeToken)
	ts.Require().NoError(err, "complete the Google callback")
	return next
}

// requireNoLocalUser checks that a Google sign-in found no local user to sign in as.
func (ts *OUBoundedSignInTestSuite) requireNoLocalUser(step *common.FlowStep) {
	ts.T().Helper()
	ts.NotEqual("COMPLETE", step.FlowStatus, "no local user is signed in: %+v", step)
	ts.Empty(step.Assertion, "a sign-in without a local user gets no assertion")
}

// A Google identity linked to a user of child two signs in there, and finds nobody when the sign-in is fixed to the
// child: link holders outside the fixed OU are not looked at.
func (ts *OUBoundedSignInTestSuite) TestLinkedAccount_HolderOutsideTheOUIsNotFound() {
	username := common.GenerateUniqueUsername("oub_link_outside")
	userID := ts.createFederatedUser(ts.childTwoOUID, username)
	sub := "oub-google-" + username
	ts.linkGoogleIdentity("oub_link_outside", ts.childTwoOUID, username, sub)
	email := username + "@ou-boundary.test"

	claims := ts.requireSignedInto(ts.googleSignIn(ts.googleSharedApp, ts.childTwoOUID, sub, email), ts.childTwoOUID)
	ts.Equal(userID, claims.Sub, "the link's holder in the OU is signed in")

	ts.requireNoLocalUser(ts.googleSignIn(ts.googleSharedApp, ts.childOUID, sub, email))
}

// A Google identity linked to a user of the parent is found in the child once the user is shared into it.
func (ts *OUBoundedSignInTestSuite) TestLinkedAccount_HolderSharedIntoTheOUIsFound() {
	username := common.GenerateUniqueUsername("oub_link_shared")
	userID := ts.createFederatedUser(ts.parentOUID, username)
	sub := "oub-google-" + username
	ts.linkGoogleIdentity("oub_link_shared", ts.parentOUID, username, sub)
	email := username + "@ou-boundary.test"

	ts.requireNoLocalUser(ts.googleSignIn(ts.googleSharedApp, ts.childOUID, sub, email))

	ts.shareUser(userID, ts.childOUID)
	claims := ts.requireSignedInto(ts.googleSignIn(ts.googleSharedApp, ts.childOUID, sub, email), ts.childOUID)
	ts.Equal(userID, claims.Sub, "the user shared into the child holds the link there")
}

// JIT under a fixed OU provisions a second holder of a Google identity already linked elsewhere, and each OU then
// signs the identity in as its own holder.
func (ts *OUBoundedSignInTestSuite) TestLinkedAccount_JITInTheOUAddsASecondHolder() {
	username := common.GenerateUniqueUsername("oub_link_jit")
	childTwoHolder := ts.createFederatedUser(ts.childTwoOUID, username)
	sub := "oub-google-" + username
	ts.linkGoogleIdentity("oub_link_jit", ts.childTwoOUID, username, sub)
	email := username + "@ou-boundary.test"

	provisioned := ts.requireSignedInto(ts.googleSignIn(ts.googleJITChildApp, "", sub, email), ts.childOUID)
	ts.federatedUserIDs = append(ts.federatedUserIDs, provisioned.Sub)
	ts.NotEqual(childTwoHolder, provisioned.Sub, "the holder in child two is not found, so a user is provisioned")

	again := ts.requireSignedInto(ts.googleSignIn(ts.googleJITChildApp, "", sub, email), ts.childOUID)
	ts.Equal(provisioned.Sub, again.Sub, "the provisioned user holds the link, so no third user is provisioned")

	claims := ts.requireSignedInto(ts.googleSignIn(ts.googleSharedApp, ts.childTwoOUID, sub, email), ts.childTwoOUID)
	ts.Equal(childTwoHolder, claims.Sub, "child two still signs the identity in as its own holder")
}

// Matching on email for account linking only considers users in the fixed OU: a user of child two with the identity's
// email is offered for linking there, and not when the sign-in is fixed to the child.
func (ts *OUBoundedSignInTestSuite) TestLinkedAccount_EmailLinkingMatchesOnlyUsersInTheOU() {
	username := common.GenerateUniqueUsername("oub_link_email")
	ts.createFederatedUser(ts.childTwoOUID, username)
	sub := "oub-google-" + username
	email := username + "@ou-boundary.test"

	outside := ts.googleSignIn(ts.googleLinkingApp, ts.childOUID, sub, email)
	ts.Empty(outside.Data.AdditionalData["linkingPromptDetails"], "no user of the child matches the email")
	ts.requireNoLocalUser(outside)

	inside := ts.googleSignIn(ts.googleLinkingApp, ts.childTwoOUID, sub, email)
	ts.Equal("INCOMPLETE", inside.FlowStatus, "the matching user of child two is offered: %+v", inside.Error)
	ts.NotEmpty(inside.Data.AdditionalData["linkingPromptDetails"], "the linking prompt names the matched user")
}
