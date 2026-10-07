// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	rtGraceClientID     = "rt_grace_test_client"
	rtGraceClientSecret = "rt_grace_test_secret"
	rtGraceRedirectURI  = "https://localhost:3000"
	rtGraceUsername     = "rt_grace_test_user"
	rtGracePassword     = "RtGracePass1!"
	// rtGraceWindowSeconds is the grace period this suite configures. It is long enough to issue a
	// handful of sequential refresh requests inside it, and short enough that waiting it out does
	// not slow the suite down.
	rtGraceWindowSeconds = 3
	// rtGraceConfigSection is the server-config section carrying the deployment's rotation policy,
	// in its refreshToken block.
	rtGraceConfigSection = "oauth"
)

var rtGraceUserType = testutils.UserType{
	Name: "rt-grace-person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

var rtGraceAuthFlow = testutils.Flow{
	Name:     "Refresh Token Grace Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_rt_grace_test",
	Nodes: []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "prompt_credentials",
		},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
				},
			},
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

// RefreshTokenGraceTestSuite exercises graceful refresh token rotation end to end.
//
// The shared integration server runs with the grace period disabled so that the existing rotation
// and replay suites keep asserting immediate invalidation. This suite therefore turns the feature on
// through the refreshToken block of the oauth server-config section, which takes effect without a restart,
// and turns it off again in teardown, so it neither depends on nor leaks configuration to the other
// suites.
type RefreshTokenGraceTestSuite struct {
	suite.Suite
	client       *http.Client
	ouID         string
	entityTypeID string
	authFlowID   string
	appID        string
	userID       string
}

func TestRefreshTokenGraceTestSuite(t *testing.T) {
	suite.Run(t, new(RefreshTokenGraceTestSuite))
}

func (ts *RefreshTokenGraceTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ts.enableGracePeriod()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "rt-grace-test-ou",
		Name:        "Refresh Token Grace Test OU",
		Description: "Organization unit for graceful refresh token rotation integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	rtGraceUserType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(rtGraceUserType)
	ts.Require().NoError(err)
	ts.entityTypeID = entityTypeID

	flowID, err := testutils.CreateFlow(rtGraceAuthFlow)
	ts.Require().NoError(err)
	ts.authFlowID = flowID

	ts.appID = ts.createApplication()

	attributesJSON, err := json.Marshal(map[string]interface{}{
		"username": rtGraceUsername,
		"password": rtGracePassword,
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		OUID:       ouID,
		Type:       "rt-grace-person",
		Attributes: json.RawMessage(attributesJSON),
	})
	ts.Require().NoError(err)
	ts.userID = userID
}

func (ts *RefreshTokenGraceTestSuite) TearDownSuite() {
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}

	ts.disableGracePeriod()
}

// enableGracePeriod turns on graceful rotation through the server-config section. The section is
// runtime-mutable, so unlike a deployment.yaml change this needs no server restart.
func (ts *RefreshTokenGraceTestSuite) enableGracePeriod() {
	body := fmt.Sprintf(`{"refreshToken":{"graceEnabled":true,"graceCeilingSeconds":%d}}`, rtGraceWindowSeconds)
	ts.Require().NoError(
		testutils.PutWritableServerConfig(rtGraceConfigSection, []byte(body)),
		"failed to enable graceful refresh token rotation")
}

// disableGracePeriod puts the section back to disabled so later suites see the shared server
// asserting immediate invalidation again.
func (ts *RefreshTokenGraceTestSuite) disableGracePeriod() {
	// An explicit false with no ceiling is enough to switch the feature off.
	body := []byte(`{"refreshToken":{"graceEnabled":false}}`)
	if err := testutils.PutWritableServerConfig(rtGraceConfigSection, body); err != nil {
		ts.T().Logf("failed to disable graceful refresh token rotation: %v", err)
	}
}

func (ts *RefreshTokenGraceTestSuite) createApplication() string {
	app := map[string]interface{}{
		"name":                      "RefreshTokenGraceApp",
		"description":               "Application for graceful refresh token rotation integration testing",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"rt-grace-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{"type": "oauth2", "config": map[string]interface{}{
				"clientId":                rtGraceClientID,
				"clientSecret":            rtGraceClientSecret,
				"redirectUris":            []string{rtGraceRedirectURI},
				"grantTypes":              []string{"authorization_code", "refresh_token"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				// The window is opted into per application; the deployment ceiling only caps it.
				"token": map[string]interface{}{
					"refreshToken": map[string]interface{}{
						"rotationGracePeriod": rtGraceWindowSeconds,
					},
				},
			}},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(bodyBytes))

	var respData map[string]interface{}
	ts.Require().NoError(json.Unmarshal(bodyBytes, &respData))
	return respData["id"].(string)
}

// obtainRefreshToken runs the authorization_code flow and returns a fresh refresh token.
func (ts *RefreshTokenGraceTestSuite) obtainRefreshToken() string {
	resp, err := testutils.InitiateAuthorizationFlow(
		rtGraceClientID, rtGraceRedirectURI, "code", "openid", "test-state")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": rtGraceUsername,
		"password": rtGracePassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	tokenResult, err := testutils.RequestToken(
		rtGraceClientID, rtGraceClientSecret, code, rtGraceRedirectURI, "authorization_code")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token)
	ts.Require().NotEmpty(tokenResult.Token.RefreshToken)

	return tokenResult.Token.RefreshToken
}

// refresh submits a refresh_token grant request and returns the status and parsed body.
func (ts *RefreshTokenGraceTestSuite) refresh(refreshToken string) (int, map[string]interface{}) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
		bytes.NewBufferString(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(rtGraceClientID, rtGraceClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var body map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().NoError(json.Unmarshal(bodyBytes, &body), "body: %s", string(bodyBytes))
	return resp.StatusCode, body
}

// introspect submits a token to the introspection endpoint and reports whether it is active.
func (ts *RefreshTokenGraceTestSuite) introspect(token string) bool {
	form := url.Values{}
	form.Set("token", token)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/introspect",
		bytes.NewBufferString(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(rtGraceClientID, rtGraceClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var body map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().NoError(json.Unmarshal(bodyBytes, &body), "body: %s", string(bodyBytes))
	active, _ := body["active"].(bool)
	return active
}

// AC1.1/AC1.2/AC1.3: a refresh token presented again inside its grace window is served, and both
// the original rotation's token and the graced rotation's token are usable afterwards. This is the
// concurrent-refresh scenario the feature exists for: without grace, the second request would fail
// and take the whole token family with it.
func (ts *RefreshTokenGraceTestSuite) TestGracePeriod_ConcurrentRefreshSucceeds() {
	original := ts.obtainRefreshToken()

	status, body := ts.refresh(original)
	ts.Require().Equal(http.StatusOK, status, "%v", body)
	firstRotated, ok := body["refresh_token"].(string)
	ts.Require().True(ok, "rotation should issue a new refresh token")
	ts.NotEqual(original, firstRotated)

	// The racing request: the same original token, presented again inside the window.
	status, body = ts.refresh(original)
	ts.Require().Equal(http.StatusOK, status,
		"a refresh inside the grace window must succeed: %v", body)
	secondRotated, ok := body["refresh_token"].(string)
	ts.Require().True(ok, "a graced refresh should also issue a new refresh token")
	ts.NotEqual(original, secondRotated)
	ts.NotEqual(firstRotated, secondRotated,
		"each graced redemption mints its own sibling refresh token")
	ts.NotEmpty(body["access_token"], "a graced refresh must return an access token")
}

// AC3.1/AC3.2/AC3.3: several redemptions inside one window all succeed, and the sibling tokens they
// produce share a token family, so one replay after the window ends all of them together.
func (ts *RefreshTokenGraceTestSuite) TestGracePeriod_SiblingsShareOneRevocableFamily() {
	original := ts.obtainRefreshToken()

	siblings := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		status, body := ts.refresh(original)
		ts.Require().Equal(http.StatusOK, status,
			"redemption %d inside the grace window should succeed: %v", i, body)
		sibling, ok := body["refresh_token"].(string)
		ts.Require().True(ok)
		siblings = append(siblings, sibling)
	}

	// Wait for the window to close, then replay the original. That is a genuine replay, so it
	// revokes the token family.
	time.Sleep((rtGraceWindowSeconds + 1) * time.Second)

	status, body := ts.refresh(original)
	ts.Require().Equal(http.StatusBadRequest, status,
		"the original token must be rejected once its window has closed: %v", body)
	ts.Equal("invalid_grant", body["error"])

	// AC3.3: one family revocation ends every sibling the race produced.
	for i, sibling := range siblings {
		status, body := ts.refresh(sibling)
		ts.Require().Equal(http.StatusBadRequest, status,
			"sibling %d should die with its token family: %v", i, body)
		ts.Equal("invalid_grant", body["error"])
	}
}

// AC2.2/AC2.3: the window is anchored to the rotation that opened it. Redeeming a token repeatedly
// inside its window does not push the deadline out, so it still closes on schedule.
func (ts *RefreshTokenGraceTestSuite) TestGracePeriod_RepeatedUseDoesNotExtendWindow() {
	original := ts.obtainRefreshToken()

	// The window opens here. Everything below is measured against this instant, not against the
	// last use, which is the property under test.
	rotatedAt := time.Now()
	status, body := ts.refresh(original)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	// Keep using the token while comfortably inside the window. The loop stops at the window's
	// halfway point so a slow request near the boundary cannot turn a timing margin into a
	// spurious failure; a sliding window would still be open long after this point.
	stopUsing := rotatedAt.Add(rtGraceWindowSeconds * time.Second / 2)
	uses := 0
	for time.Now().Before(stopUsing) {
		status, body = ts.refresh(original)
		ts.Require().Equal(http.StatusOK, status,
			"redemptions inside the window should keep succeeding: %v", body)
		uses++
		time.Sleep(200 * time.Millisecond)
	}
	ts.Require().Positive(uses, "the test must exercise at least one redemption inside the window")

	// Wait until the window has closed, measured from the rotation that opened it. Had each use
	// pushed the deadline out, the token would still be redeemable here.
	time.Sleep(time.Until(rotatedAt.Add((rtGraceWindowSeconds + 2) * time.Second)))

	status, body = ts.refresh(original)
	ts.Require().Equal(http.StatusBadRequest, status,
		"repeated use must not extend the window: %v", body)
	ts.Equal("invalid_grant", body["error"])
}

// AC2.4: only the immediately previous generation is graced. A token two rotations back has its own
// window, which closed at its own rotation, so it can never come back.
func (ts *RefreshTokenGraceTestSuite) TestGracePeriod_OlderGenerationIsNotGraced() {
	original := ts.obtainRefreshToken()

	status, body := ts.refresh(original)
	ts.Require().Equal(http.StatusOK, status, "%v", body)
	firstRotated, _ := body["refresh_token"].(string)
	ts.Require().NotEmpty(firstRotated)

	// Let the original's window close before rotating again, so the chain is unambiguous.
	time.Sleep((rtGraceWindowSeconds + 1) * time.Second)

	status, body = ts.refresh(firstRotated)
	ts.Require().Equal(http.StatusOK, status,
		"the current token should still refresh normally: %v", body)
	ts.Require().NotEmpty(body["refresh_token"])

	// The original is now two generations back and long past its own window.
	status, body = ts.refresh(original)
	ts.Require().Equal(http.StatusBadRequest, status,
		"an older generation must not be redeemable: %v", body)
	ts.Equal("invalid_grant", body["error"])
}

// AC4.3: a refresh token inside its grace window is redeemable at the token endpoint and nowhere
// else. Introspection reports it inactive, so the window never widens what the token appears to
// authorize.
func (ts *RefreshTokenGraceTestSuite) TestGracePeriod_GracedTokenIntrospectsInactive() {
	original := ts.obtainRefreshToken()

	ts.Require().True(ts.introspect(original),
		"a current refresh token should introspect active")

	status, body := ts.refresh(original)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	// The token is now inside its grace window: still redeemable on the refresh grant, but not a
	// valid token for any other purpose.
	status, body = ts.refresh(original)
	ts.Require().Equal(http.StatusOK, status,
		"the token should still be redeemable inside its window: %v", body)

	ts.False(ts.introspect(original),
		"a graced refresh token must introspect inactive")
}

// AC4.2: a revoked token family outranks an open grace window. The criteria deny list is consulted
// before the single-token list, so a compromise response can never be softened by grace.
func (ts *RefreshTokenGraceTestSuite) TestGracePeriod_FamilyRevocationOutranksGrace() {
	original := ts.obtainRefreshToken()

	status, body := ts.refresh(original)
	ts.Require().Equal(http.StatusOK, status, "%v", body)
	rotated, _ := body["refresh_token"].(string)
	ts.Require().NotEmpty(rotated)

	// Explicitly revoke the rotated token. With token-family revocation on explicit revoke enabled,
	// this ends the whole family, including the token still inside its grace window.
	form := url.Values{}
	form.Set("token", rotated)
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/revoke",
		bytes.NewBufferString(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(rtGraceClientID, rtGraceClientSecret)
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	status, body = ts.refresh(original)
	ts.Require().Equal(http.StatusBadRequest, status,
		"a revoked family must outrank an open grace window: %v", body)
	ts.Equal("invalid_grant", body["error"])
}

// The deployment ceiling caps an application that asks for more. The application is reconfigured to
// request a window far beyond the ceiling; the ceiling must still bound it.
func (ts *RefreshTokenGraceTestSuite) TestGracePeriod_CeilingCapsApplicationWindow() {
	ts.setApplicationGracePeriod(rtGraceWindowSeconds * 20)
	defer ts.setApplicationGracePeriod(rtGraceWindowSeconds)

	original := ts.obtainRefreshToken()
	status, body := ts.refresh(original)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	// Past the deployment ceiling, but well inside what the application asked for.
	time.Sleep(time.Duration(rtGraceWindowSeconds+1) * time.Second)

	status, body = ts.refresh(original)
	ts.Require().Equal(http.StatusBadRequest, status,
		"the deployment ceiling, not the application's request, must bound the window: %v", body)
	ts.Equal("invalid_grant", body["error"])
}

// Turning the feature off at the deployment closes open windows at once, without touching any
// application's own configuration.
func (ts *RefreshTokenGraceTestSuite) TestGracePeriod_DeploymentKillSwitchClosesWindows() {
	original := ts.obtainRefreshToken()
	status, body := ts.refresh(original)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	// The application still carries its window; the deployment withdraws the feature.
	ts.disableGracePeriod()
	defer ts.enableGracePeriod()

	status, body = ts.refresh(original)
	ts.Require().Equal(http.StatusBadRequest, status,
		"disabling the feature must close windows already open: %v", body)
	ts.Equal("invalid_grant", body["error"])
}

// An application that configures no window of its own is not graced, even while the deployment has
// the feature enabled: the window is opted into per application.
func (ts *RefreshTokenGraceTestSuite) TestGracePeriod_ApplicationWithoutWindowIsNotGraced() {
	ts.setApplicationGracePeriod(0)
	defer ts.setApplicationGracePeriod(rtGraceWindowSeconds)

	original := ts.obtainRefreshToken()
	status, body := ts.refresh(original)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	status, body = ts.refresh(original)
	ts.Require().Equal(http.StatusBadRequest, status,
		"an application that opted into no window must not be graced: %v", body)
	ts.Equal("invalid_grant", body["error"])
}

// setApplicationGracePeriod rewrites the test application's own rotation grace period.
func (ts *RefreshTokenGraceTestSuite) setApplicationGracePeriod(seconds int) {
	req, err := http.NewRequest("GET", testutils.TestServerURL+"/applications/"+ts.appID, nil)
	ts.Require().NoError(err)
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	appBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(appBytes))

	var app map[string]interface{}
	ts.Require().NoError(json.Unmarshal(appBytes, &app))

	inbound, ok := app["inboundAuthConfig"].([]interface{})
	ts.Require().True(ok && len(inbound) > 0, "expected an inbound auth config: %s", string(appBytes))
	entry, ok := inbound[0].(map[string]interface{})
	ts.Require().True(ok)
	cfg, ok := entry["config"].(map[string]interface{})
	ts.Require().True(ok)

	token, ok := cfg["token"].(map[string]interface{})
	if !ok {
		token = map[string]interface{}{}
		cfg["token"] = token
	}
	refreshToken, ok := token["refreshToken"].(map[string]interface{})
	if !ok {
		refreshToken = map[string]interface{}{}
		token["refreshToken"] = refreshToken
	}
	refreshToken["rotationGracePeriod"] = seconds

	updated, err := json.Marshal(app)
	ts.Require().NoError(err)
	putReq, err := http.NewRequest("PUT", testutils.TestServerURL+"/applications/"+ts.appID,
		bytes.NewBuffer(updated))
	ts.Require().NoError(err)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := ts.client.Do(putReq)
	ts.Require().NoError(err)
	putBytes, _ := io.ReadAll(putResp.Body)
	putResp.Body.Close()
	ts.Require().Equal(http.StatusOK, putResp.StatusCode, string(putBytes))
}
