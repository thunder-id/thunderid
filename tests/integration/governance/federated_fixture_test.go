// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	federatedClientID     = "governance_google_client"
	federatedClientSecret = "governance_google_secret"
	federatedRedirectURI  = "http://localhost:3000/callback"
)

// federatedFixture is a Google IdP, a sign-in flow through it, and an application using that flow.
// An account is linked by recording the IdP identity against it, matched on email.
type federatedFixture struct {
	server   *testutils.MockGoogleOIDCServer
	idpID    string
	flowID   string
	appID    string
	ouID     string
	userType string
}

// federatedIdPPort is where the server reaches the Google endpoints in the test deployment.
const federatedIdPPort = 8093

// newFederatedFixture stands the fixture up for users of userType in ouID.
func newFederatedFixture(t *testing.T, name, ouID, userType string) *federatedFixture {
	t.Helper()

	server, err := testutils.NewMockGoogleOIDCServer(federatedIdPPort, federatedClientID, federatedClientSecret)
	require.NoError(t, err, "failed to create the mock IdP")
	require.NoError(t, server.Start(), "failed to start the mock IdP")
	f := &federatedFixture{server: server, ouID: ouID, userType: userType}

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

// signedInWithCode runs the federated flow as the identity with this email, but answers the IdP
// callback with code instead of the one the IdP issued, and reports whether the sign-in completed.
func (f *federatedFixture) signedInWithCode(t *testing.T, email, code string) bool {
	t.Helper()

	f.server.SetAuthorizeFunc(func(string) (string, error) { return email, nil })

	step, err := common.InitiateAuthenticationFlow(f.appID, false, nil, "")
	require.NoError(t, err, "failed to start the federated flow")
	require.Equal(t, "REDIRECTION", step.Type, "the flow must redirect to the IdP")

	_, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	require.NoError(t, err, "failed to authorize at the IdP")

	done, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"code": code, "state": state}, "", step.ChallengeToken)
	if err != nil {
		return false
	}
	return done.FlowStatus == "COMPLETE" && done.Assertion != ""
}

func (f *federatedFixture) signedIn(t *testing.T, email string) bool {
	t.Helper()
	step := f.signIn(t, email)
	return step.FlowStatus == "COMPLETE" && step.Assertion != ""
}

// requireRefusedWithStatedHold checks that the federated sign-in is refused and states the hold.
func (f *federatedFixture) requireRefusedWithStatedHold(t *testing.T, email, code, label string) {
	t.Helper()
	step := f.signIn(t, email)
	require.NotEqual(t, "COMPLETE", step.FlowStatus, "%s: the federated sign-in must be refused", label)
	require.Empty(t, step.Assertion, "%s: a refused sign-in must carry no assertion", label)
	require.NotNil(t, step.Error, "%s: the refusal must carry an error", label)
	require.Equal(t, code, step.Error.Code, "%s: the refusal must state the hold", label)
}

func (f *federatedFixture) tearDown(t *testing.T) {
	t.Helper()
	if f == nil {
		return
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
	if f.server != nil {
		_ = f.server.Stop()
		time.Sleep(200 * time.Millisecond)
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
