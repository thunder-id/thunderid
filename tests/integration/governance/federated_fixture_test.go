// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	federatedClientID     = "governance_google_client"
	federatedClientSecret = "governance_google_secret"
	federatedRedirectURI  = "http://localhost:3000/callback"

	// pickAction submits the link holder the user picked.
	pickAction = "action_pick"

	// federatedIdPPort is where the test deployment points the server for the Google endpoints, so the
	// mock IdP cannot take a free port.
	federatedIdPPort = 8093
)

// federatedFixture is a Google IdP, a sign-in flow through it, and an application using that flow.
// An account is linked by recording the IdP identity against it, matched on email. Applications
// with other flows through the same IdP are added on demand and deleted with the fixture.
type federatedFixture struct {
	server   *testutils.MockGoogleOIDCServer
	idpID    string
	flowID   string
	appID    string
	ouID     string
	userType string

	extraAppIDs  []string
	extraFlowIDs []string
}

// newFederatedFixture stands the fixture up for users of userType in ouID.
func newFederatedFixture(t *testing.T, name, ouID, userType string) *federatedFixture {
	t.Helper()

	server, err := testutils.NewMockGoogleOIDCServer(federatedIdPPort, federatedClientID, federatedClientSecret)
	require.NoError(t, err, "failed to create the mock IdP")
	require.NoError(t, server.Start(), "failed to start the mock IdP")
	f := &federatedFixture{server: server, ouID: ouID, userType: userType}
	t.Cleanup(func() { f.tearDown(t) })

	f.idpID, err = testutils.CreateIDP(testutils.IDP{
		Name: "Governance " + name + " IdP",
		Type: "GOOGLE",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: federatedClientID},
			{Name: "client_secret", Value: federatedClientSecret, IsSecret: true},
			{Name: "redirect_uri", Value: federatedRedirectURI},
			{Name: "scopes", Value: "openid email profile"},
		},
		AttributeConfiguration: &testutils.AttributeConfiguration{
			AccountLinking: &testutils.AccountLinking{Attributes: []string{"email"}},
		},
	})
	require.NoError(t, err, "failed to create the IdP")

	f.flowID, err = testutils.CreateFlow(testutils.Flow{
		Name:     "Governance " + name + " federated flow",
		FlowType: "AUTHENTICATION",
		Handle:   "governance_" + name + "_federated_flow",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "google_auth"},
			{
				"id":         "google_auth",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"idpId": f.idpID},
				"executor":   map[string]interface{}{"name": "GoogleOIDCAuthExecutor"},
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
	})
	require.NoError(t, err, "failed to create the federated flow")

	f.appID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Governance " + name + " federated app",
		ClientID:         "governance_" + name + "_federated_client",
		ClientSecret:     "governance_" + name + "_federated_secret",
		RedirectURIs:     []string{federatedRedirectURI},
		AllowedUserTypes: []string{userType},
		AuthFlowID:       f.flowID,
		OUID:             ouID,
	})
	require.NoError(t, err, "failed to create the federated application")
	return f
}

// addIdentity registers an IdP identity and returns its email.
func (f *federatedFixture) addIdentity(sub string) string {
	email := federatedEmail(sub)
	f.server.AddUser(&testutils.GoogleUserInfo{Sub: sub, Email: email, EmailVerified: true, Name: sub})
	return email
}

// link records the identity with this email against the local account the password proves.
func (f *federatedFixture) link(t *testing.T, email, username, password string) {
	t.Helper()

	f.server.SetAuthorizeFunc(func(string) (string, error) { return email, nil })
	require.NoError(t, common.LinkAccount(common.LinkRequest{
		Handle:       "governance-link-" + username,
		ExecutorName: "GoogleOIDCAuthExecutor",
		IDPID:        f.idpID,
		OUID:         f.ouID,
		UserType:     f.userType,
		Username:     username,
		Password:     password,
	}), "failed to link the account")
}

// linkHolder is an account a federated identity is recorded against: its id and username, the
// password that proves it, and the organization unit and user type it belongs to.
type linkHolder struct {
	id, username, password, ouID, userType string
}

// linkHolders records the identity with this email against every holder, each proven by its password
// through a linking sign-in of its own unit and type. A sign-in checks for a link when it starts, so
// every sign-in reaches its linking prompt before any is confirmed; that is how several accounts come
// to hold one identity. Each holder must carry the email for the linking sign-in to match it.
func (f *federatedFixture) linkHolders(t *testing.T, email string, holders ...linkHolder) {
	t.Helper()

	f.server.SetAuthorizeFunc(func(string) (string, error) { return email, nil })
	pending := make([]*common.FlowStep, 0, len(holders))
	for _, holder := range holders {
		pending = append(pending, f.startLinking(t, holder))
	}
	for i, holder := range holders {
		step, err := common.CompleteFlow(pending[i].ExecutionID, nil, common.LinkAccountAction,
			pending[i].ChallengeToken)
		require.NoError(t, err, "failed to confirm the match for %s", holder.username)
		step, err = common.CompleteFlow(step.ExecutionID,
			map[string]string{"username": holder.username, "password": holder.password},
			common.VerifyPasswordAction, step.ChallengeToken)
		require.NoError(t, err, "failed to verify %s", holder.username)
		require.Equal(t, "COMPLETE", step.FlowStatus, "%s must be linked, got %+v", holder.username, step)
	}
}

// startLinking runs a linking sign-in through an application of the holder's unit and type to its
// linking prompt and returns that step. The application and its flows are deleted when t ends.
func (f *federatedFixture) startLinking(t *testing.T, holder linkHolder) *common.FlowStep {
	t.Helper()

	handle := "gov-link-" + holder.id

	var flowIDs []string
	t.Cleanup(func() {
		for _, flowID := range flowIDs {
			if err := testutils.DeleteFlow(flowID); err != nil {
				t.Logf("teardown: failed to delete flow %s: %v", flowID, err)
			}
		}
	})
	verifyFlowID, err := testutils.CreateFlow(common.VerifyPasswordFlow(handle + "-verify"))
	require.NoError(t, err, "failed to create the verification flow")
	flowIDs = append(flowIDs, verifyFlowID)
	flowID, err := testutils.CreateFlow(
		common.VerifiedLinkingFlow(handle, "GoogleOIDCAuthExecutor", f.idpID, verifyFlowID))
	require.NoError(t, err, "failed to create the linking flow")
	flowIDs = append(flowIDs, flowID)
	regFlowID, err := testutils.CreateIsolatedRegistrationFlow(handle + "-reg")
	require.NoError(t, err, "failed to create the linking registration flow")
	flowIDs = append(flowIDs, regFlowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:               handle,
		ClientID:           handle,
		ClientSecret:       handle + "-secret",
		RedirectURIs:       []string{federatedRedirectURI},
		AllowedUserTypes:   []string{holder.userType},
		OUID:               holder.ouID,
		AuthFlowID:         flowID,
		RegistrationFlowID: regFlowID,
	})
	require.NoError(t, err, "failed to create the linking application")
	t.Cleanup(func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			t.Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	})

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	require.NoError(t, err, "failed to start the linking sign-in")
	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	require.NoError(t, err, "failed to authorize at the IdP")
	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{"code": code, "state": state}, "",
		step.ChallengeToken)
	require.NoError(t, err, "failed to complete the federated step")
	require.Equal(t, "INCOMPLETE", step.FlowStatus, "%s must reach the linking prompt, got %+v",
		holder.username, step)
	require.NotEmpty(t, step.Data.AdditionalData["linkingPromptDetails"],
		"%s must be matched for linking, got %+v", holder.username, step)
	return step
}

// signIn runs the federated flow as the identity with this email. A refusal is returned as the flow's
// error; a transport failure fails the test.
func (f *federatedFixture) signIn(t *testing.T, email string) *common.FlowStep {
	t.Helper()

	f.server.SetAuthorizeFunc(func(string) (string, error) { return email, nil })

	step, err := common.InitiateAuthenticationFlow(f.appID, false, nil, "")
	require.NoError(t, err, "failed to start the federated flow")
	require.Equal(t, "REDIRECTION", step.Type, "the flow must redirect to the IdP")

	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	require.NoError(t, err, "failed to authorize at the IdP")

	done, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"code": code, "state": state}, "", step.ChallengeToken)
	require.NoError(t, err, "federated flow request failed")
	return done
}

// signInWithCode runs the federated flow as the identity with this email, but answers the IdP
// callback with code instead of the one the IdP issued, and returns the answer whatever it is. A
// request that reaches no server fails the test.
func (f *federatedFixture) signInWithCode(t *testing.T, email, code string) flowAnswer {
	t.Helper()

	f.server.SetAuthorizeFunc(func(string) (string, error) { return email, nil })

	step, err := common.InitiateAuthenticationFlow(f.appID, false, nil, "")
	require.NoError(t, err, "failed to start the federated flow")
	require.Equal(t, "REDIRECTION", step.Type, "the flow must redirect to the IdP")

	_, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	require.NoError(t, err, "failed to authorize at the IdP")

	return answerOf(postFlow(t, nil, "", map[string]interface{}{
		"executionId":    step.ExecutionID,
		"inputs":         map[string]string{"code": code, "state": state},
		"challengeToken": step.ChallengeToken,
	}))
}

// signedIn reports whether the federated flow, run as the identity with this email, issues an
// assertion.
func (f *federatedFixture) signedIn(t *testing.T, email string) bool {
	t.Helper()
	step := f.signIn(t, email)
	return step.FlowStatus == "COMPLETE" && step.Assertion != ""
}

// requireRefusedWithStatedHold checks that the federated sign-in is refused and states the hold.
func (f *federatedFixture) requireRefusedWithStatedHold(t *testing.T, email, code, label string) {
	t.Helper()
	requireStatedHoldStep(t, f.signIn(t, email), code, label)
}

// newLinkPickerApp adds an application whose sign-in asks which account to continue with when several
// users hold the federated identity, and binds the one picked.
func (f *federatedFixture) newLinkPickerApp(t *testing.T, name string) string {
	t.Helper()

	return f.newExtraApp(t, name, testutils.Flow{
		Name:     "Governance " + name + " link picker flow",
		FlowType: "AUTHENTICATION",
		Handle:   "governance_" + name + "_link_picker_flow",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "google_auth"},
			{
				"id":         "google_auth",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"idpId": f.idpID},
				"executor":   map[string]interface{}{"name": "GoogleOIDCAuthExecutor"},
				"onSuccess":  "link_resolver",
			},
			{
				"id":           "link_resolver",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "IdentifyingExecutor", "mode": "resolve"},
				"onSuccess":    "auth_assert",
				"onIncomplete": "link_picker",
			},
			{
				"id":   "link_picker",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{"action": map[string]interface{}{"ref": pickAction, "nextNode": "link_resolver"}},
				},
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}, false)
}

// newRegistrationApp adds an application that registers through the IdP and, like the shipped
// registration flow, ends at provisioning. An identity some user in the target organization unit
// already holds completes the registration as that user; holders elsewhere do not count.
func (f *federatedFixture) newRegistrationApp(t *testing.T, name string) string {
	t.Helper()

	return f.newExtraApp(t, name, testutils.Flow{
		Name:     "Governance " + name + " federated registration flow",
		FlowType: "REGISTRATION",
		Handle:   "governance_" + name + "_federated_registration_flow",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "user_type_resolver"},
			{
				"id":        "user_type_resolver",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "UserTypeResolver"},
				"onSuccess": "google_auth",
			},
			{
				"id":   "google_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                             f.idpID,
					"allowRegistrationWithExistingUser": true,
				},
				"executor":  map[string]interface{}{"name": "GoogleOIDCAuthExecutor"},
				"onSuccess": "provisioning",
			},
			{
				"id":         "provisioning",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"allowCrossOUProvisioning": true},
				"executor":   map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess":  "end",
			},
			{"id": "end", "type": "END"},
		},
	}, true)
}

// newExtraApp creates the flow and an application using it. A registration flow's application signs in
// through the fixture's own flow.
func (f *federatedFixture) newExtraApp(t *testing.T, name string, flow testutils.Flow, registration bool) string {
	t.Helper()

	flowID, err := testutils.CreateFlow(flow)
	require.NoError(t, err, "failed to create the %s flow", name)
	f.extraFlowIDs = append(f.extraFlowIDs, flowID)

	app := testutils.Application{
		Name:             "Governance " + name + " federated app",
		ClientID:         "governance_" + name + "_federated_client",
		ClientSecret:     "governance_" + name + "_federated_secret",
		RedirectURIs:     []string{federatedRedirectURI},
		AllowedUserTypes: []string{f.userType},
		AuthFlowID:       flowID,
		OUID:             f.ouID,
	}
	if registration {
		app.AuthFlowID = f.flowID
		app.IsRegistrationFlowEnabled = true
		app.RegistrationFlowID = flowID
	}
	appID, err := testutils.CreateApplication(app)
	require.NoError(t, err, "failed to create the %s application", name)
	f.extraAppIDs = append(f.extraAppIDs, appID)
	return appID
}

// signInAndPick runs the link picker application's sign-in as the identity with this email, requires
// it to ask which account to continue with, and picks the one with this username.
func (f *federatedFixture) signInAndPick(t *testing.T, appID, email, username string) *common.FlowStep {
	t.Helper()

	f.server.SetAuthorizeFunc(func(string) (string, error) { return email, nil })
	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	require.NoError(t, err, "failed to start the federated flow")
	require.Equal(t, "REDIRECTION", step.Type, "the flow must redirect to the IdP")
	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	require.NoError(t, err, "failed to authorize at the IdP")

	pick, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"code": code, "state": state}, "", step.ChallengeToken)
	require.NoError(t, err, "federated flow request failed")
	require.Equal(t, "INCOMPLETE", pick.FlowStatus, "several holders must be picked between, got %+v", pick)
	require.Empty(t, pick.Assertion, "no assertion may be issued before an account is picked")
	require.True(t, common.HasInput(pick.Data.Inputs, "username"),
		"the holders differ on username, so it must be offered, got %+v", pick.Data.Inputs)

	picked, err := common.CompleteFlow(pick.ExecutionID, map[string]string{"username": username}, pickAction,
		pick.ChallengeToken)
	require.NoError(t, err, "failed to complete the flow")
	return picked
}

// register runs the registration application's flow as the identity with this email.
func (f *federatedFixture) register(t *testing.T, appID, email string) *common.FlowStep {
	t.Helper()

	f.server.SetAuthorizeFunc(func(string) (string, error) { return email, nil })
	step, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	require.NoError(t, err, "failed to start the federated registration")
	require.Equal(t, "REDIRECTION", step.Type, "the registration must redirect to the IdP")
	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	require.NoError(t, err, "failed to authorize at the IdP")

	done, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"code": code, "state": state}, "", step.ChallengeToken)
	require.NoError(t, err, "failed to complete the flow")
	return done
}

// requireStatedHoldStep checks a flow step was refused, issued nothing and states the hold.
func requireStatedHoldStep(t *testing.T, step *common.FlowStep, code, label string) {
	t.Helper()
	require.NotEqual(t, "COMPLETE", step.FlowStatus, "%s: the flow must be refused", label)
	require.Empty(t, step.Assertion, "%s: a refused flow must carry no assertion", label)
	require.NotNil(t, step.Error, "%s: the refusal must carry an error", label)
	require.Equal(t, code, step.Error.Code, "%s: the refusal must state the hold", label)
}

// tearDown deletes what the fixture created and stops the mock IdP, logging rather than failing.
func (f *federatedFixture) tearDown(t *testing.T) {
	t.Helper()
	for _, appID := range f.extraAppIDs {
		if err := testutils.DeleteApplication(appID); err != nil {
			t.Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}
	for _, flowID := range f.extraFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}
	if f.appID != "" {
		if err := testutils.DeleteApplication(f.appID); err != nil {
			t.Logf("teardown: failed to delete the federated app: %v", err)
		}
	}
	if f.flowID != "" {
		if err := testutils.DeleteFlow(f.flowID); err != nil {
			t.Logf("teardown: failed to delete the federated flow: %v", err)
		}
	}
	if f.idpID != "" {
		if err := testutils.DeleteIDP(f.idpID); err != nil {
			t.Logf("teardown: failed to delete the IdP: %v", err)
		}
	}
	if err := f.server.Stop(); err != nil {
		t.Logf("teardown: failed to stop the mock IdP: %v", err)
	}
}

// federatedSub names an IdP subject nobody else in this package uses.
func federatedSub(name, suffix string) string {
	return fmt.Sprintf("gov-%s-%s", name, suffix)
}

// federatedEmail is the email the IdP asserts for sub, which a local account carries to be matched.
func federatedEmail(sub string) string {
	return sub + "@governance.example"
}
