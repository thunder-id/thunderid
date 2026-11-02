// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	lastLoginSSOClientID     = "governance_last_login_sso_client"
	lastLoginSSOClientSecret = "governance_last_login_sso_secret" //nolint:gosec // test credential
	lastLoginSSORedirectURI  = "https://localhost:3000/callback"
)

// LastLoginTestSuite checks that lastLoginAt is recorded where access is granted, and only
// when the category asks for it. The setting is switched through the writable accountAccess layer,
// which each test starts empty and the suite puts back as it found it.
type LastLoginTestSuite struct {
	suite.Suite
	smsServer   *testutils.MockNotificationServer
	smsSenderID string
	users       *userFixture
	appID       string
	mobileSeq   int
}

func TestLastLoginTestSuite(t *testing.T) {
	suite.Run(t, new(LastLoginTestSuite))
}

func (ts *LastLoginTestSuite) SetupSuite() {
	t := ts.T()
	original, err := testutils.GetWritableServerConfig(accountAccessSection)
	ts.Require().NoError(err, "failed to read the accountAccess section")
	t.Cleanup(func() {
		if err := testutils.PutWritableServerConfig(accountAccessSection, original); err != nil {
			t.Errorf("teardown: failed to restore the %s section: %v", accountAccessSection, err)
		}
	})

	ts.smsServer = testutils.NewMockNotificationServer(0)
	ts.Require().NoError(ts.smsServer.Start(), "failed to start the mock notification server")
	t.Cleanup(func() {
		if err := ts.smsServer.Stop(); err != nil {
			t.Logf("teardown: failed to stop the mock SMS server: %v", err)
		}
	})
	ts.smsSenderID, err = testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:        "Governance Last Login SMS Sender",
		Description: "Sender for the last-login recording tests",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: ts.smsServer.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	ts.Require().NoError(err, "failed to create the notification sender")
	t.Cleanup(func() {
		if err := testutils.DeleteNotificationSender(ts.smsSenderID); err != nil {
			t.Logf("teardown: failed to delete the notification sender: %v", err)
		}
	})

	ts.users = newUserFixture(t, "last_login",
		testutils.UserType{Schema: credentialsSchema(map[string]interface{}{"mobile_number": stringAttribute})})

	flowID, err := testutils.CreateFlow(passwordSignInFlow("Governance last_login password flow",
		"governance_last_login_password_flow", "AUTHENTICATION"))
	ts.Require().NoError(err, "failed to create the password flow")
	cleanupFlow(t, flowID)

	ts.appID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Governance last login app",
		ClientID:         "governance_last_login_client",
		ClientSecret:     "governance_last_login_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{ts.users.userType},
		AuthFlowID:       flowID,
		OUID:             ts.users.ouID,
	})
	ts.Require().NoError(err, "failed to create the application")
	cleanupApplication(t, ts.appID)

	ssoFlowID, err := testutils.CreateFlow(buildSSOAuthFlow("Governance Last Login SSO Flow",
		"governance-last-login-sso"))
	ts.Require().NoError(err, "failed to create the SSO flow")
	cleanupFlow(t, ssoFlowID)

	ssoAppID, err := testutils.CreateApplication(testutils.Application{
		OUID:             ts.users.ouID,
		Name:             "Governance last login SSO app",
		Description:      "Application the session-reuse recording test signs in through",
		AuthFlowID:       ssoFlowID,
		AllowedUserTypes: []string{ts.users.userType},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                lastLoginSSOClientID,
				"clientSecret":            lastLoginSSOClientSecret,
				"redirectUris":            []string{lastLoginSSORedirectURI},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"scopes":                  []string{"openid"},
			},
		}},
	})
	ts.Require().NoError(err, "failed to create the SSO application")
	cleanupApplication(t, ssoAppID)
}

// SetupTest starts each test with nothing in the writable layer, so recording is off.
func (ts *LastLoginTestSuite) SetupTest() {
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection, []byte(`{}`)),
		"failed to clear the %s section", accountAccessSection)
	ts.smsServer.ClearMessages()
}

// With recording off, the shipped default, a completed sign-in records nothing.
func (ts *LastLoginTestSuite) TestNothingIsRecordedWhenRecordingIsOff() {
	userID, username, password, _ := ts.newUser("off")

	ts.requireSignedIn(username, password, "the sign-in itself must succeed")

	ts.Empty(ts.readLastLoginAt(userID), "recording is off, so no lastLoginAt may appear")
}

// With recording on, a completed sign-in records when it happened, with no restart.
func (ts *LastLoginTestSuite) TestACompletedSignInIsRecorded() {
	userID, username, password, _ := ts.newUser("on")
	ts.recordLastLogin(0)

	started := time.Now().UTC().Truncate(time.Second)
	ts.requireSignedIn(username, password, "the sign-in itself must succeed")

	requireRecordedSince(ts.T(), userCollection, userID, started, "flow sign-in")
}

// For session reuse: a sign-in satisfied by a live SSO session grants access too, so it moves
// lastLoginAt: the value means access last granted, not credentials last verified.
func (ts *LastLoginTestSuite) TestASessionReuseIsRecorded() {
	userID, username, password, _ := ts.newUser("sso")
	ts.recordLastLogin(0)

	browser := newBrowser(ts.T(), lastLoginSSOClientID, lastLoginSSOClientSecret, lastLoginSSORedirectURI)
	_, step := browser.signIn("last_login_"+ts.users.suffix,
		map[string]string{"username": username, "password": password})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the first sign-in must complete with credentials")
	first := ts.readLastLoginAt(userID)
	ts.Require().NotEmpty(first, "the first sign-in must record lastLoginAt")

	reusedFrom := awaitTheSecondAfter(ts.T(), first)
	ts.Require().True(browser.sessionSatisfiesAuthorize("last_login_reuse_"+ts.users.suffix),
		"the second sign-in must be satisfied by the session")

	requireRecordedSince(ts.T(), userCollection, userID, reusedFrom, "session reuse")
}

// A sign-in through an atomic authentication endpoint records lastLoginAt as a flow does.
func (ts *LastLoginTestSuite) TestAnAtomicSignInIsRecorded() {
	userID, username, password, _ := ts.newUser("atomic")
	ts.recordLastLogin(0)

	started := time.Now().UTC().Truncate(time.Second)
	_, err := testutils.ObtainAuthAssertion(username, password)
	ts.Require().NoError(err, "the atomic sign-in itself must succeed")

	requireRecordedSince(ts.T(), userCollection, userID, started, "atomic password")
}

// For the one-time password: the atomic verify endpoint is a completed sign-in, and records too.
func (ts *LastLoginTestSuite) TestAnAtomicOneTimePasswordSignInIsRecorded() {
	userID, _, _, mobile := ts.newUser("atomic_otp")
	ts.recordLastLogin(0)

	started := time.Now().UTC().Truncate(time.Second)
	sessionToken, code := sendDirectOTP(ts.T(), ts.smsServer, ts.smsSenderID, mobile)
	ts.Require().True(verifyDirectOTP(ts.T(), sessionToken, code).succeeded(),
		"the atomic one-time-password sign-in must succeed")

	requireRecordedSince(ts.T(), userCollection, userID, started, "atomic one-time password")
}

// A refused sign-in, by a wrong secret or by a suspension, does not move lastLoginAt.
func (ts *LastLoginTestSuite) TestARefusedSignInIsNotRecorded() {
	userID, username, password, _ := ts.newUser("refused")
	ts.recordLastLogin(0)

	ts.requireSignedIn(username, password, "the first sign-in must succeed")
	first := ts.readLastLoginAt(userID)
	ts.Require().NotEmpty(first)

	awaitTheSecondAfter(ts.T(), first)
	ts.requireRefused(username, "DefinitelyNotTheSecret!", "a wrong secret must be refused")
	ts.Equal(first, ts.readLastLoginAt(userID), "a wrong secret must not move lastLoginAt")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.requireRefused(username, password, "a suspended account must be refused")
	ts.Equal(first, ts.readLastLoginAt(userID), "a refused grant must not move lastLoginAt")
}

// Inside the resolution window a later sign-in keeps the stamp; at zero every sign-in moves it.
func (ts *LastLoginTestSuite) TestTheResolutionWindowBoundsTheWrites() {
	windowed, windowedName, windowedPassword, _ := ts.newUser("windowed")
	everyTime, everyTimeName, everyTimePassword, _ := ts.newUser("every_time")

	ts.recordLastLogin(3600)
	ts.requireSignedIn(windowedName, windowedPassword, "the first windowed sign-in must succeed")
	first := ts.readLastLoginAt(windowed)
	ts.Require().NotEmpty(first)
	awaitTheSecondAfter(ts.T(), first)
	ts.requireSignedIn(windowedName, windowedPassword, "the second windowed sign-in must succeed")
	ts.Equal(first, ts.readLastLoginAt(windowed),
		"a sign-in inside the resolution window must leave lastLoginAt where it was")

	ts.recordLastLogin(0)
	ts.requireSignedIn(everyTimeName, everyTimePassword, "the first sign-in must succeed")
	first = ts.readLastLoginAt(everyTime)
	ts.Require().NotEmpty(first)
	awaitTheSecondAfter(ts.T(), first)
	ts.requireSignedIn(everyTimeName, everyTimePassword, "the second sign-in must succeed")
	ts.NotEqual(first, ts.readLastLoginAt(everyTime),
		"with a resolution of zero every completed sign-in must move lastLoginAt")
}

// recordLastLogin turns user recording on through the writable layer, with the given resolution.
func (ts *LastLoginTestSuite) recordLastLogin(resolutionSeconds int) {
	ts.T().Helper()
	body := []byte(fmt.Sprintf(`{"user":{"recordLastLogin":true,"lastLoginResolutionSeconds":%d}}`,
		resolutionSeconds))
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection, body),
		"the section must accept the last-login settings")
}

// requireRecordedSince requires the entity's lastLoginAt to be an RFC 3339 instant no earlier than
// since, and returns it.
func requireRecordedSince(t *testing.T, collection, entityID string, since time.Time, label string) string {
	t.Helper()

	recorded := readLastLoginAt(t, collection, entityID)
	require.NotEmpty(t, recorded, "%s must record lastLoginAt", label)
	at, err := time.Parse(time.RFC3339, recorded)
	require.NoError(t, err, "%s: lastLoginAt must be an RFC 3339 instant, got %q", label, recorded)
	assert.False(t, at.Before(since), "%s: lastLoginAt %s is earlier than the access that set it (%s)",
		label, recorded, since.Format(time.RFC3339))
	return recorded
}

// signIn runs the password flow with the credentials and returns the step it ends on. A refusal is
// still a flow answer, so a failed request fails the test.
func (ts *LastLoginTestSuite) signIn(username, password string) *common.FlowStep {
	ts.T().Helper()

	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "failed to start the password flow")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username, "password": password}, signInAction, step.ChallengeToken)
	ts.Require().NoError(err, "the sign-in must be answered")
	return step
}

// requireSignedIn requires the password flow to complete with an assertion.
func (ts *LastLoginTestSuite) requireSignedIn(username, password, msg string) {
	ts.T().Helper()
	step := ts.signIn(username, password)
	ts.Require().Equal("COMPLETE", step.FlowStatus, msg)
	ts.Require().NotEmpty(step.Assertion, msg)
}

// requireRefused requires the password flow to keep the sign-in on the prompt with an error and
// issue nothing.
func (ts *LastLoginTestSuite) requireRefused(username, password, msg string) {
	ts.T().Helper()
	step := ts.signIn(username, password)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, msg)
	ts.Require().NotNil(step.Error, msg)
	ts.Require().Empty(step.Assertion, msg)
}

// readLastLoginAt returns the lastLoginAt a management read of the user reports, empty when absent.
func (ts *LastLoginTestSuite) readLastLoginAt(userID string) string {
	ts.T().Helper()
	return readLastLoginAt(ts.T(), userCollection, userID)
}

// newUser creates one account with a mobile number, nobody else in this package touches.
func (ts *LastLoginTestSuite) newUser(name string) (userID, username, password, mobile string) {
	ts.T().Helper()

	ts.mobileSeq++
	mobile = fmt.Sprintf("+1720%07d", ts.mobileSeq)
	userID, username, password = ts.users.newUser(ts.T(), name, map[string]interface{}{"mobile_number": mobile})
	return userID, username, password, mobile
}
