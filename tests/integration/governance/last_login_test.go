// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// LastLoginTestSuite checks that lastLoginAt is recorded where access is granted, and only
// when the category asks for it. The setting is switched through the writable accountAccess layer,
// which each test starts empty and the suite puts back as it found it.
type LastLoginTestSuite struct {
	suite.Suite
	smsServer        *testutils.MockNotificationServer
	smsSenderID      string
	ouID             string
	entityTypeID     string
	flowID           string
	appID            string
	ssoFlowID        string
	ssoAppID         string
	userIDs          []string
	suffix           string
	mobileSeq        int
	originalWritable json.RawMessage
}

func TestLastLoginTestSuite(t *testing.T) {
	suite.Run(t, new(LastLoginTestSuite))
}

var lastLoginOU = testutils.OrganizationUnit{
	Handle:      "governance_last_login_ou",
	Name:        "Governance Last Login Test OU",
	Description: "Organization unit for the last-login recording tests",
}

var lastLoginUserType = testutils.UserType{
	Handle:      "governance_last_login_user",
	DisplayName: "governance_last_login_user",
	Schema: map[string]interface{}{
		"username":      map[string]interface{}{"type": "string"},
		"password":      map[string]interface{}{"type": "string", "credential": true},
		"mobile_number": map[string]interface{}{"type": "string"},
	},
}

// stampGranularity is how far apart two sign-ins must be for their stamps to differ: the stored
// value has second precision.
const stampGranularity = 1100 * time.Millisecond

const (
	lastLoginSSOClientID     = "governance_last_login_sso_client"
	lastLoginSSOClientSecret = "governance_last_login_sso_secret" //nolint:gosec // test credential
	lastLoginSSORedirectURI  = "https://localhost:3000/callback"
)

func (ts *LastLoginTestSuite) SetupSuite() {
	var err error
	ts.originalWritable, err = testutils.GetWritableServerConfig(accountAccessSection)
	ts.Require().NoError(err, "failed to read the accountAccess section")

	ts.smsServer = testutils.NewMockNotificationServer(0)
	ts.Require().NoError(ts.smsServer.Start(), "failed to start the mock notification server")
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

	ouID, err := testutils.CreateOrganizationUnit(lastLoginOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	lastLoginUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(lastLoginUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	ts.flowID, err = testutils.CreateFlow(passwordFlow("last_login"))
	ts.Require().NoError(err, "failed to create the password flow")

	ts.appID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Governance last login app",
		ClientID:         "governance_last_login_client",
		ClientSecret:     "governance_last_login_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{lastLoginUserType.Handle},
		AuthFlowID:       ts.flowID,
		OUID:             ts.ouID,
	})
	ts.Require().NoError(err, "failed to create the application")

	ssoFlow := buildSSOAuthFlow()
	ssoFlow.Name = "Governance Last Login SSO Flow"
	ssoFlow.Handle = "governance-last-login-sso"
	ts.ssoFlowID, err = testutils.CreateFlow(ssoFlow)
	ts.Require().NoError(err, "failed to create the SSO flow")

	ts.ssoAppID, err = testutils.CreateApplication(testutils.Application{
		OUID:             ts.ouID,
		Name:             "Governance last login SSO app",
		Description:      "Application the session-reuse recording test signs in through",
		AuthFlowID:       ts.ssoFlowID,
		AllowedUserTypes: []string{lastLoginUserType.Handle},
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

	ts.suffix = uniqueSuffix()
}

// SetupTest starts each test with nothing in the writable layer, so recording is off.
func (ts *LastLoginTestSuite) SetupTest() {
	ts.Require().NoError(testutils.PutWritableServerConfig(accountAccessSection, []byte(`{}`)),
		"failed to clear the %s section", accountAccessSection)
	ts.smsServer.ClearMessages()
}

func (ts *LastLoginTestSuite) TearDownSuite() {
	if ts.originalWritable != nil {
		if err := testutils.PutWritableServerConfig(accountAccessSection, ts.originalWritable); err != nil {
			ts.T().Errorf("teardown: failed to restore the %s section: %v", accountAccessSection, err)
		}
	}
	for _, userID := range ts.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("teardown: failed to delete user %s: %v", userID, err)
		}
	}
	for _, appID := range []string{ts.appID, ts.ssoAppID} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}
	for _, flowID := range []string{ts.flowID, ts.ssoFlowID} {
		if flowID == "" {
			continue
		}
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}
	if ts.smsSenderID != "" {
		if err := testutils.DeleteNotificationSender(ts.smsSenderID); err != nil {
			ts.T().Logf("teardown: failed to delete the notification sender: %v", err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("teardown: failed to delete user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
	if ts.smsServer != nil {
		_ = ts.smsServer.Stop()
	}
}

// With recording off, the shipped default, a completed sign-in records nothing.
func (ts *LastLoginTestSuite) TestNothingIsRecordedWhenRecordingIsOff() {
	userID, username, password, _ := ts.newUser("off")

	ts.Require().True(ts.signIn(username, password), "the sign-in itself must succeed")

	ts.Empty(ts.readLastLoginAt(userID), "recording is off, so no lastLoginAt may appear")
}

// With recording on, a completed sign-in records when it happened, with no restart.
func (ts *LastLoginTestSuite) TestACompletedSignInIsRecorded() {
	userID, username, password, _ := ts.newUser("on")
	ts.recordLastLogin(0)

	started := time.Now().UTC().Truncate(time.Second)
	ts.Require().True(ts.signIn(username, password), "the sign-in itself must succeed")

	ts.requireRecordedSince(userID, started, "flow sign-in")
}

// For session reuse: a sign-in satisfied by a live SSO session grants access too, so it moves
// lastLoginAt: the value means access last granted, not credentials last verified.
func (ts *LastLoginTestSuite) TestASessionReuseIsRecorded() {
	userID, username, password, _ := ts.newUser("sso")
	ts.recordLastLogin(0)

	client := newSessionClient(ts.T())
	ts.Require().True(ts.ssoSignIn(client, map[string]string{"username": username, "password": password}),
		"the first sign-in must complete with credentials")
	first := ts.readLastLoginAt(userID)
	ts.Require().NotEmpty(first, "the first sign-in must record lastLoginAt")

	time.Sleep(stampGranularity)
	reusedFrom := time.Now().UTC().Truncate(time.Second)
	ts.Require().True(ts.ssoSignIn(client, nil), "the second sign-in must be satisfied by the session")

	ts.requireRecordedSince(userID, reusedFrom, "session reuse")
}

// A sign-in through an atomic authentication endpoint records lastLoginAt as a flow does.
func (ts *LastLoginTestSuite) TestAnAtomicSignInIsRecorded() {
	userID, username, password, _ := ts.newUser("atomic")
	ts.recordLastLogin(0)

	started := time.Now().UTC().Truncate(time.Second)
	_, err := testutils.ObtainAuthAssertion(username, password)
	ts.Require().NoError(err, "the atomic sign-in itself must succeed")

	ts.requireRecordedSince(userID, started, "atomic password")
}

// For the one-time password: the atomic verify endpoint is a completed sign-in, and records too.
func (ts *LastLoginTestSuite) TestAnAtomicOneTimePasswordSignInIsRecorded() {
	userID, _, _, mobile := ts.newUser("atomic_otp")
	ts.recordLastLogin(0)

	started := time.Now().UTC().Truncate(time.Second)
	sessionToken, code, status := sendDirectOTP(ts.T(), ts.smsServer, ts.smsSenderID, mobile)
	ts.Require().Equal(http.StatusOK, status, "the code must be dispatched")
	ts.Require().NotEmpty(code, "a code must be delivered")
	ts.Require().True(verifyDirectOTP(ts.T(), sessionToken, code).succeeded(), "the atomic one-time-password sign-in must succeed")

	ts.requireRecordedSince(userID, started, "atomic one-time password")
}

// A refused sign-in, by a wrong secret or by a suspension, does not move lastLoginAt.
func (ts *LastLoginTestSuite) TestARefusedSignInIsNotRecorded() {
	userID, username, password, _ := ts.newUser("refused")
	ts.recordLastLogin(0)

	ts.Require().True(ts.signIn(username, password), "the first sign-in must succeed")
	first := ts.readLastLoginAt(userID)
	ts.Require().NotEmpty(first)

	time.Sleep(stampGranularity)
	ts.Require().False(ts.signIn(username, "DefinitelyNotTheSecret!"), "a wrong secret must be refused")
	ts.Equal(first, ts.readLastLoginAt(userID), "a wrong secret must not move lastLoginAt")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.Require().False(ts.signIn(username, password), "a suspended account must be refused")
	ts.Equal(first, ts.readLastLoginAt(userID), "a refused grant must not move lastLoginAt")
}

// Inside the resolution window a later sign-in keeps the stamp; at zero every sign-in moves it.
func (ts *LastLoginTestSuite) TestTheResolutionWindowBoundsTheWrites() {
	windowed, windowedName, windowedPassword, _ := ts.newUser("windowed")
	everyTime, everyTimeName, everyTimePassword, _ := ts.newUser("every_time")

	ts.recordLastLogin(3600)
	ts.Require().True(ts.signIn(windowedName, windowedPassword))
	first := ts.readLastLoginAt(windowed)
	ts.Require().NotEmpty(first)
	time.Sleep(stampGranularity)
	ts.Require().True(ts.signIn(windowedName, windowedPassword))
	ts.Equal(first, ts.readLastLoginAt(windowed),
		"a sign-in inside the resolution window must leave lastLoginAt where it was")

	ts.recordLastLogin(0)
	ts.Require().True(ts.signIn(everyTimeName, everyTimePassword))
	first = ts.readLastLoginAt(everyTime)
	ts.Require().NotEmpty(first)
	time.Sleep(stampGranularity)
	ts.Require().True(ts.signIn(everyTimeName, everyTimePassword))
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

// requireRecordedSince requires lastLoginAt to be an RFC 3339 instant no earlier than since.
func (ts *LastLoginTestSuite) requireRecordedSince(userID string, since time.Time, label string) {
	ts.T().Helper()

	recorded := ts.readLastLoginAt(userID)
	ts.Require().NotEmpty(recorded, "%s: a completed sign-in must record lastLoginAt", label)
	at, err := time.Parse(time.RFC3339, recorded)
	ts.Require().NoError(err, "%s: lastLoginAt must be an RFC 3339 instant, got %q", label, recorded)
	ts.False(at.Before(since), "%s: lastLoginAt %s is earlier than the sign-in that set it (%s)",
		label, recorded, since.Format(time.RFC3339))
}

// signIn runs the password flow to the end and reports whether it completed.
func (ts *LastLoginTestSuite) signIn(username, password string) bool {
	ts.T().Helper()

	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "failed to start the password flow")

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username, "password": password}, "action_credentials", step.ChallengeToken)
	if err != nil {
		return false
	}
	return step.FlowStatus == "COMPLETE"
}

// ssoSignIn starts an authorization through the SSO application and reports whether the flow
// issued an assertion. With credentials it requires a prompt first and answers it; without, the
// flow must complete on its first step, satisfied by the client's session.
func (ts *LastLoginTestSuite) ssoSignIn(client *http.Client, credentials map[string]string) bool {
	ts.T().Helper()

	params := url.Values{}
	params.Set("client_id", lastLoginSSOClientID)
	params.Set("redirect_uri", lastLoginSSORedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", "last_login_"+ts.suffix)

	resp, err := client.Get(testServerURL + "/oauth2/authorize?" + params.Encode())
	ts.Require().NoError(err, "the authorize request failed")
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize must redirect to the gate")
	_, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to read the gate redirect")

	step := flowExecute(ts.T(), client, map[string]interface{}{"executionId": executionID})
	if credentials != nil {
		ts.Require().NotEqual("COMPLETE", step.FlowStatus, "a first sign-in must prompt for credentials")
		step = flowExecute(ts.T(), client, map[string]interface{}{
			"executionId":    executionID,
			"inputs":         credentials,
			"action":         "action_001",
			"challengeToken": step.ChallengeToken,
		})
	}
	return step.FlowStatus == "COMPLETE" && step.Assertion != ""
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
	username = fmt.Sprintf("last_login_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	mobile = fmt.Sprintf("+1720%07d", ts.mobileSeq)
	attributes, err := json.Marshal(map[string]interface{}{
		"username":      username,
		"password":      password,
		"mobile_number": mobile,
	})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       lastLoginUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, password, mobile
}

// passwordFlow is a username and password sign-in that ends in an assertion.
func passwordFlow(name string) testutils.Flow {
	credentialInputs := []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "Governance " + name + " password flow",
		FlowType: "AUTHENTICATION",
		Handle:   "governance_" + name + "_password_flow",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": credentialInputs,
					"action": map[string]interface{}{"ref": "action_credentials", "nextNode": "credentials_auth"},
				}},
			},
			{
				"id":           "credentials_auth",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_credentials",
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
