// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ExistingAccessTestSuite checks what each hold does to existing sessions and refresh grants: a
// suspension ends them; a lock leaves them alone, since anyone who knows a username can form one.
// Logins are browser-shaped, with a cookie jar carrying the SSO cookie between authorize calls.
type ExistingAccessTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	appID        string
	authFlowID   string
	userIDs      []string
	suffix       string
}

func TestExistingAccessTestSuite(t *testing.T) {
	suite.Run(t, new(ExistingAccessTestSuite))
}

const (
	existingAccessClientID     = "governance_existing_access_client"
	existingAccessClientSecret = "governance_existing_access_secret" //nolint:gosec // test credential
	existingAccessRedirectURI  = "https://localhost:3000/callback"

	// ssoCookiePrefix is the per-flow SSO handle cookie the session transport mints. Its presence is
	// how a test tells "the session was established" from "the flow happened to complete".
	ssoCookiePrefix = "tid_sso_"
)

var existingAccessOU = testutils.OrganizationUnit{
	Handle:      "governance_existing_access_ou",
	Name:        "Governance Existing Access Test OU",
	Description: "Organization unit for the existing-access acceptance tests",
}

var existingAccessUserType = testutils.UserType{
	Handle:      "governance_existing_access_user",
	DisplayName: "governance_existing_access_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

func (ts *ExistingAccessTestSuite) SetupSuite() {
	applyLockoutPolicy(ts.T())

	ouID, err := testutils.CreateOrganizationUnit(existingAccessOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	existingAccessUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(existingAccessUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	authFlowID, err := testutils.CreateFlow(buildSSOAuthFlow())
	ts.Require().NoError(err, "failed to create the SSO authentication flow")
	ts.authFlowID = authFlowID

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:             ts.ouID,
		Name:             "Governance Existing Access Test App",
		Description:      "Application the existing-access acceptance tests sign in through",
		AuthFlowID:       authFlowID,
		AllowedUserTypes: []string{existingAccessUserType.Handle},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":     existingAccessClientID,
					"clientSecret": existingAccessClientSecret,
					"redirectUris": []string{existingAccessRedirectURI},
					"grantTypes": []string{"authorization_code", "refresh_token",
						"urn:ietf:params:oauth:grant-type:token-exchange", "client_credentials"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid"},
				},
			},
		},
	})
	ts.Require().NoError(err, "failed to create the test application")
	ts.appID = appID

	ts.suffix = uniqueSuffix()
}

func (ts *ExistingAccessTestSuite) TearDownSuite() {
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("teardown: failed to delete application: %v", err)
		}
	}
	for _, userID := range ts.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("teardown: failed to delete user %s: %v", userID, err)
		}
	}
	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", ts.authFlowID, err)
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
}

// A suspension ends both the SSO session and the refresh grant.
// Asserted separately, since either one left live is enough to regain access.
func (ts *ExistingAccessTestSuite) TestSuspendEndsExistingSessionsAndTokens() {
	userID, username, password := ts.newUser("suspend_existing")

	client := newSessionClient(ts.T())
	token := ts.login(client, username, password, "existing_1")
	ts.Require().NotEmpty(token.RefreshToken, "the login must produce a refresh grant to revoke")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "the login must establish an SSO session to end")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	ts.False(ts.refreshSucceeds(token.RefreshToken),
		"a suspension must revoke the refresh grant the account already held")
	ts.False(ts.sessionStillSatisfiesAuthorize(client, "existing_2"),
		"a suspension must end the SSO session the account already held")
}

// Suspending again completes, keeps the first suspension's record, and leaves access revoked.
// A failed revocation step cannot be forced here, so the repeat's revocation is unit-tested.
func (ts *ExistingAccessTestSuite) TestRepeatingASuspensionKeepsTheFirstRecord() {
	userID, username, password := ts.newUser("suspend_repeat")

	client := newSessionClient(ts.T())
	token := ts.login(client, username, password, "existing_repeat")
	ts.Require().NotEmpty(token.RefreshToken, "the login must produce a refresh grant to revoke")

	runAdministrationFlowWithInputs(ts.T(), suspendFlowHandle, map[string]string{
		subjectInput:      userID,
		operatorNoteInput: "First note",
	})
	first := readStatus(ts.T(), userID)
	ts.Require().NotNil(first.Details)

	runAdministrationFlowWithInputs(ts.T(), suspendFlowHandle, map[string]string{
		subjectInput:      userID,
		operatorNoteInput: "Second note",
	})
	repeat := readStatus(ts.T(), userID)

	ts.Equal("SUSPENDED", repeat.Value, "a repeat must leave the identity suspended")
	ts.Require().NotNil(repeat.Details)
	ts.Equal(first.Details.Since, repeat.Details.Since, "a repeat must not move the placement time")
	ts.Equal("First note", repeat.Details.OperatorNote, "a repeat must not replace the first note")
	ts.False(ts.refreshSucceeds(token.RefreshToken), "access must stay revoked after a repeat")
}

// A lock leaves the SSO session and the refresh grant alone.
// The lock is first checked to refuse fresh authentication, so this cannot pass with locking broken.
func (ts *ExistingAccessTestSuite) TestALockLeavesExistingSessionsAndTokensAlone() {
	userID, username, password := ts.newUser("lock_existing")

	client := newSessionClient(ts.T())
	token := ts.login(client, username, password, "existing_3")
	ts.Require().NotEmpty(token.RefreshToken, "the login must produce a refresh grant")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "the login must establish an SSO session")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"the account must be locked before the criterion means anything")
	ts.Require().False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the lock must actually be withholding a fresh authentication")

	ts.True(ts.refreshSucceeds(token.RefreshToken),
		"a lock must not stop an existing refresh grant yielding tokens")
	ts.True(ts.sessionStillSatisfiesAuthorize(client, "existing_4"),
		"a lock must not end an existing SSO session")
}

// Removing a restriction does not restore revoked sessions or tokens: after unsuspend the
// refresh grant stays dead and the SSO session does not come back.
func (ts *ExistingAccessTestSuite) TestReleasingTheSuspensionRestoresNothing() {
	userID, username, password := ts.newUser("restore_nothing")

	client := newSessionClient(ts.T())
	token := ts.login(client, username, password, "existing_5")
	ts.Require().NotEmpty(token.RefreshToken, "the login must produce a refresh grant")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	cutoffBound := ts.suspendedSince(userID)
	ts.Require().False(ts.refreshSucceeds(token.RefreshToken),
		"the grant must have been revoked before its non-restoration means anything")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	ts.Require().NotEqual("SUSPENDED", readStatus(ts.T(), userID).Value,
		"the suspension itself must have been released")

	ts.False(ts.refreshSucceeds(token.RefreshToken),
		"releasing a hold must not revive a revoked refresh grant")
	ts.False(ts.sessionStillSatisfiesAuthorize(client, "existing_6"),
		"releasing a hold must not revive an ended SSO session")
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	// The cutoff is the pre-suspend time plus jwt.leeway, so a grant from within that window is revoked
	// with the old ones. Sign in again once the cutoff has passed.
	waitPastTheCutoff(cutoffBound)
	fresh := ts.login(newSessionClient(ts.T()), username, password, "fresh_after_release")
	ts.True(ts.tokenActive(fresh.AccessToken), "fresh access token must be usable after release")
	ts.True(ts.refreshSucceeds(fresh.RefreshToken), "fresh refresh grant must be usable after release")
	ts.False(ts.tokenActive(token.AccessToken), "old access token remains revoked")
	ts.False(ts.refreshSucceeds(token.RefreshToken), "old refresh remains revoked")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.False(ts.tokenActive(fresh.AccessToken), "re-suspension revokes the new grant")
	ts.False(ts.refreshSucceeds(fresh.RefreshToken))

}

// Suspending the application a user signed in through revokes that user's tokens, as deleting it
// does. Once released and past the cutoff, a fresh sign-in through the application works again.
// Introspection authenticates as the same client, so the old tokens are checked after the release.
func (ts *ExistingAccessTestSuite) TestSuspendingTheApplicationRevokesTheUserTokensItIssued() {
	_, username, password := ts.newUser("app_suspend")
	token := ts.login(newSessionClient(ts.T()), username, password, "app_suspend_1")
	ts.Require().True(ts.tokenActive(token.AccessToken), "the token must be usable before the suspension")
	// Client credentials: the application's own token names its entity ID, so the entity criterion
	// matches it as well as the client ID.
	ownResult := clientCredentials(ts.T(), existingAccessClientID, existingAccessClientSecret)
	ts.Require().Equal(http.StatusOK, ownResult.StatusCode, "client credentials failed: %s", ownResult.Body)
	own := ownResult.Token
	ts.Equal(ts.appID, ts.claim(own.AccessToken, "sub"), "a client credentials token must name the application")
	ts.Require().True(ts.tokenActive(own.AccessToken))

	runAdministrationFlow(ts.T(), applicationSuspendFlowHandle, ts.appID)
	// An application read carries no placement time, so the flow's return bounds the pre-check instead.
	cutoffBound := time.Now()
	released := false
	defer func() {
		if !released {
			runAdministrationFlow(ts.T(), applicationUnsuspendFlowHandle, ts.appID)
		}
	}()
	runAdministrationFlow(ts.T(), applicationUnsuspendFlowHandle, ts.appID)
	released = true

	ts.False(ts.tokenActive(token.AccessToken),
		"an application suspension must revoke the access tokens users obtained through it")
	ts.False(ts.refreshSucceeds(token.RefreshToken),
		"an application suspension must revoke the refresh grants users obtained through it")
	ts.False(ts.tokenActive(own.AccessToken),
		"an application suspension must revoke the application's own client credentials token")

	waitPastTheCutoff(cutoffBound)
	fresh := ts.login(newSessionClient(ts.T()), username, password, "app_suspend_2")
	ts.True(ts.tokenActive(fresh.AccessToken), "a token issued after the release must be usable")
	ts.True(ts.refreshSucceeds(fresh.RefreshToken), "a refresh grant issued after the release must be usable")
	again := clientCredentials(ts.T(), existingAccessClientID, existingAccessClientSecret)
	ts.Require().Equal(http.StatusOK, again.StatusCode, "client credentials failed: %s", again.Body)
	ts.True(ts.tokenActive(again.Token.AccessToken),
		"a client credentials token issued after the release must be usable")
}

// Every token path names the entity ID in `sub`, so a suspension rejects what each issued
// before it: code, refresh, token exchange, introspection, UserInfo and the REST API. Client
// credentials is covered in the application test above.
func (ts *ExistingAccessTestSuite) TestEveryTokenPathNamesTheEntityAndSuspensionRevokesIt() {
	userID, username, password := ts.newUser("token_paths")
	token := ts.login(newSessionClient(ts.T()), username, password, "token_paths")

	ts.Equal(userID, ts.claim(token.AccessToken, "sub"), "code: the access token must name the entity")
	ts.Equal(userID, ts.claim(token.IDToken, "sub"), "code: the ID token must name the entity")
	ts.Equal(userID, ts.claim(token.RefreshToken, "access_token_sub"),
		"code: the refresh token must carry the entity as the subject it issues for")

	refreshed, err := testutils.RefreshAccessTokenRaw(existingAccessClientID, existingAccessClientSecret,
		token.RefreshToken, "")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, refreshed.StatusCode, "refresh must succeed before the suspension")
	ts.Equal(userID, ts.claim(refreshed.Token.AccessToken, "sub"), "refresh: the new access token must name the entity")

	status, exchanged := ts.exchangeAccessToken(token.AccessToken)
	ts.Require().Equal(http.StatusOK, status, "token exchange must succeed before the suspension")
	ts.Equal(userID, ts.claim(exchanged.AccessToken, "sub"), "exchange: the issued token must name the entity")

	ts.Equal(userID, ts.introspectedSubject(token.AccessToken), "introspection must report the entity")
	status, sub := ts.userInfo(token.AccessToken)
	ts.Require().Equal(http.StatusOK, status, "UserInfo must answer before the suspension")
	ts.Equal(userID, sub, "UserInfo must report the entity")
	ts.Equal(http.StatusOK, ts.usersMe(token.AccessToken), "the REST API must accept the token before the suspension")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	// The REST API checks a copy of the deny list this deployment refreshes every two seconds.
	ts.Eventually(func() bool { return ts.usersMe(token.AccessToken) == http.StatusUnauthorized },
		10*time.Second, 250*time.Millisecond, "the REST API must refuse a revoked token")

	for name, raw := range map[string]string{
		"code": token.AccessToken, "refresh": refreshed.Token.AccessToken, "exchange": exchanged.AccessToken,
	} {
		ts.False(ts.tokenActive(raw), "%s: the access token must be revoked", name)
	}
	ts.False(ts.refreshSucceeds(refreshed.Token.RefreshToken), "refresh: the rotated grant must be revoked")
	status, _ = ts.exchangeAccessToken(token.AccessToken)
	ts.NotEqual(http.StatusOK, status, "exchange: a revoked subject token must not be exchanged")
	status, _ = ts.userInfo(token.AccessToken)
	ts.NotEqual(http.StatusOK, status, "UserInfo must refuse a revoked token")
}

// The cutoff is the pre-check plus jwt.leeway, so it outlives an immediate release: a
// sign-in completed inside that window, after an unsuspend and an unlock, is refused at code
// redemption, and the same sign-in once the cutoff has passed is not.
func (ts *ExistingAccessTestSuite) TestASignInInsideTheCutoffIsRefusedAfterRelease() {
	userID, username, password := ts.newUser("inside_cutoff")

	beforeSuspend := time.Now()
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	cutoffBound := ts.suspendedSince(userID)
	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	ts.Require().Equal("ACTIVE", readStatus(ts.T(), userID).Value, "the account must be fully released")

	client := newSessionClient(ts.T())
	code := ts.signInForCode(client, username, password, "inside_cutoff")
	// The pre-check ran after beforeSuspend, so the cutoff is at least beforeSuspend plus the leeway.
	// A sign-in that finished later than that, with a second to spare for iat's precision, proves nothing.
	if !time.Now().Before(beforeSuspend.Add(suspensionCutoffLeeway - time.Second)) {
		ts.T().Skip("the release and sign-in took longer than the leeway, so it did not land inside the cutoff")
	}
	status, body := ts.redeemCode(client, code)
	ts.NotEqual(http.StatusOK, status, "a sign-in inside the cutoff must not redeem: %s", body)
	ts.Contains(body, "invalid_grant", "the refused redemption must be an ordinary invalid grant")

	waitPastTheCutoff(cutoffBound)
	fresh := ts.login(newSessionClient(ts.T()), username, password, "past_cutoff")
	ts.True(ts.tokenActive(fresh.AccessToken), "a sign-in past the cutoff must be usable")
}

// suspendedSince returns when the user's suspension was placed. The pre-check, and so the cutoff's
// base, comes before it, which makes it a safe upper bound for the cutoff.
func (ts *ExistingAccessTestSuite) suspendedSince(userID string) time.Time {
	ts.T().Helper()
	held := readStatus(ts.T(), userID)
	ts.Require().Equal("SUSPENDED", held.Value)
	ts.Require().NotNil(held.Details)
	since, err := time.Parse(time.RFC3339Nano, held.Details.Since)
	ts.Require().NoError(err, "the suspension must report a readable placement time")
	return since
}

// waitPastTheCutoff sleeps until a suspension whose pre-check ran no later than bound has a cutoff
// in the past, with two seconds for iat's second precision.
func waitPastTheCutoff(bound time.Time) {
	time.Sleep(time.Until(bound.Add(suspensionCutoffLeeway).Add(2 * time.Second)))
}

// signInForCode drives a first-time sign-in up to the authorization code, without redeeming it.
func (ts *ExistingAccessTestSuite) signInForCode(client *http.Client, username, password,
	state string) string {
	ts.T().Helper()

	authID, executionID := ts.authorize(client, state)
	initial := flowExecute(ts.T(), client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "a first login must prompt for credentials")
	step := flowExecute(ts.T(), client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": username, "password": password},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "a released account's sign-in must complete")
	code, err := testutils.ExtractAuthorizationCode(ts.completeAuthorization(client, authID, step.Assertion))
	ts.Require().NoError(err, "failed to extract the authorization code")
	return code
}

// redeemCode swaps an authorization code for tokens and returns the status and body as they came.
func (ts *ExistingAccessTestSuite) redeemCode(client *http.Client, code string) (int, string) {
	ts.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", existingAccessRedirectURI)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(existingAccessClientID, existingAccessClientSecret)
	resp, err := client.Do(req)
	ts.Require().NoError(err, "the token request failed")
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(body)
}

// newUser creates one account nobody else in this package touches.
func (ts *ExistingAccessTestSuite) newUser(name string) (userID, username, password string) {
	ts.T().Helper()

	username = fmt.Sprintf("access_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username,
		"password": password,
		"email":    username + "@example.com",
	})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       existingAccessUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, password
}

// ssoCookieNames reports the per-flow SSO cookies the client currently holds.
func (ts *ExistingAccessTestSuite) ssoCookieNames(client *http.Client) []string {
	parsed, err := url.Parse(testServerURL)
	ts.Require().NoError(err)

	var names []string
	for _, cookie := range client.Jar.Cookies(parsed) {
		if strings.HasPrefix(cookie.Name, ssoCookiePrefix) {
			names = append(names, cookie.Name)
		}
	}
	return names
}

// login drives a first-time sign-in to completion and returns its tokens. The first step must
// prompt, proving the session was established by this login.
func (ts *ExistingAccessTestSuite) login(client *http.Client, username, password,
	state string) *testutils.TokenResponse {
	ts.T().Helper()

	authID, executionID := ts.authorize(client, state)

	initial := flowExecute(ts.T(), client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "a first login must prompt for credentials")

	step := flowExecute(ts.T(), client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": username, "password": password},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the credential login must complete the flow")
	ts.Require().NotEmpty(step.Assertion, "the login must yield an assertion")

	code, err := testutils.ExtractAuthorizationCode(ts.completeAuthorization(client, authID, step.Assertion))
	ts.Require().NoError(err, "failed to extract the authorization code")

	return ts.exchangeCode(client, code)
}

// sessionStillSatisfiesAuthorize reports whether the client's SSO session is still live: a second
// authorize that completes on its first step was satisfied by it; a prompt means it has ended.
func (ts *ExistingAccessTestSuite) sessionStillSatisfiesAuthorize(client *http.Client,
	state string) bool {
	ts.T().Helper()

	_, executionID := ts.authorize(client, state)
	step := flowExecute(ts.T(), client, map[string]interface{}{"executionId": executionID})
	return step.FlowStatus == "COMPLETE" && step.Assertion != ""
}

// refreshSucceeds reports whether the refresh grant still yields tokens.
func (ts *ExistingAccessTestSuite) refreshSucceeds(refreshToken string) bool {
	ts.T().Helper()

	result, err := testutils.RefreshAccessTokenRaw(existingAccessClientID, existingAccessClientSecret,
		refreshToken, "")
	ts.Require().NoError(err, "the refresh request itself must reach the server")
	return result.StatusCode == http.StatusOK && result.Token != nil && result.Token.AccessToken != ""
}

// authorize starts an authorization code flow and returns the authId and executionId the gate
// redirect carries.
func (ts *ExistingAccessTestSuite) authorize(client *http.Client, state string) (string, string) {
	ts.T().Helper()

	params := url.Values{}
	params.Set("client_id", existingAccessClientID)
	params.Set("redirect_uri", existingAccessRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", state)

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/oauth2/authorize?"+params.Encode(), nil)
	ts.Require().NoError(err)

	resp, err := client.Do(req)
	ts.Require().NoError(err, "the authorize request failed")
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize must redirect to the gate")
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to read the gate redirect")
	return authID, executionID
}

// completeAuthorization hands the assertion back to the authorize callback and returns the client
// redirect it answers with.
func (ts *ExistingAccessTestSuite) completeAuthorization(client *http.Client, authID,
	assertion string) string {
	ts.T().Helper()

	payload, err := json.Marshal(map[string]string{"authId": authID, "assertion": assertion})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/auth/callback",
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	ts.Require().NoError(err, "the auth callback request failed")
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "the auth callback failed: %s", string(respBody))

	var authz testutils.AuthorizationResponse
	ts.Require().NoError(json.Unmarshal(respBody, &authz), "unreadable authorization response")
	return authz.RedirectURI
}

// exchangeCode swaps the authorization code for tokens.
func (ts *ExistingAccessTestSuite) exchangeCode(client *http.Client, code string) *testutils.TokenResponse {
	ts.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", existingAccessRedirectURI)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(existingAccessClientID, existingAccessClientSecret)

	resp, err := client.Do(req)
	ts.Require().NoError(err, "the token request failed")
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "the token request failed: %s", string(respBody))

	var token testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal(respBody, &token), "unreadable token response")
	return &token
}

// buildSSOAuthFlow is an authentication flow with a reusable SSO session: SSO_CHECK short-circuits
// to the session node when one is live, and SessionExecutor establishes it on a first login.
func buildSSOAuthFlow() testutils.Flow {
	return testutils.Flow{
		Name:     "Governance Existing Access Test Auth Flow",
		Handle:   "governance-existing-access-auth",
		FlowType: "AUTHENTICATION",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "sso_check",
			},
			{
				"id":   "sso_check",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "SSOCheckExecutor",
				},
				"properties": map[string]interface{}{
					"checkpointRef": "session_main",
				},
				"onSuccess": "session_main",
				"onFailure": "prompt_credentials",
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
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "credentials_auth",
						},
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
				"onSuccess":    "session_main",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":   "session_main",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "SessionExecutor",
				},
				"onSuccess": "authorization_check",
			},
			{
				"id":   "authorization_check",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthorizationExecutor",
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
}

// Governance adds no claims: the tokens carry no session or grant claim, and `sub` is the entity ID.
func (ts *ExistingAccessTestSuite) TestTokenClaimsStayStandard() {
	userID, username, password := ts.newUser("standard_claims")
	token := ts.login(newSessionClient(ts.T()), username, password, "standard_claims")
	for name, raw := range map[string]string{"access": token.AccessToken, "refresh": token.RefreshToken,
		"id": token.IDToken} {
		ts.Require().NotEmpty(raw, "the %s token is issued", name)
		claims, err := testutils.DecodeJWTPayloadMap(raw)
		ts.Require().NoError(err)
		ts.NotContains(claims, "seid", "the %s token carries no session claim", name)
		ts.NotContains(claims, "Grant", "the %s token carries no grant claim", name)
		ts.NotContains(claims, "grant", "the %s token carries no grant claim", name)
	}
	claims, err := testutils.DecodeJWTPayloadMap(token.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(userID, claims["sub"], "the access token's subject is the entity ID")
	ts.True(ts.tokenActive(token.AccessToken), "the access token introspects as active")
}

// The revocation switches bound what a suspension withdraws. With OAuth enforcement off, an old
// access token still introspects as active but refresh is refused; with native API enforcement off,
// `/users/me` still accepts it, without the operator note, while introspection rejects it.
func (ts *ExistingAccessTestSuite) TestTheRevocationSwitchesBoundWhatASuspensionWithdraws() {
	oauth := ts.deploymentSection("oauth")
	server := ts.deploymentSection("server")
	// Puts both sections back with a restart, even if a check fails.
	defer func() {
		ts.Assert().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{"oauth": oauth, "server": server}))
		ts.Assert().NoError(testutils.RestartServer())
		ts.Assert().NoError(testutils.ObtainAdminAccessToken())
	}()

	ts.switchDeployment("oauth", []string{"token_revocation", "enabled"}, false)
	{
		userID, username, password := ts.newUser("oauth_switch_off")
		token := ts.login(newSessionClient(ts.T()), username, password, "oauth_switch_off")
		runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

		ts.True(ts.tokenActive(token.AccessToken),
			"with OAuth revocation off, introspection does not consult the suspension's record")
		ts.False(ts.refreshSucceeds(token.RefreshToken), "refresh is still refused by its live check")
	}

	// Only the native API switch is off from here: the OAuth section goes back before the restart.
	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{"oauth": oauth}))
	ts.switchDeployment("server", []string{"security", "token_revocation", "enabled"}, false)
	{
		userID, username, password := ts.newUser("native_switch_off")
		token := ts.login(newSessionClient(ts.T()), username, password, "native_switch_off")
		ts.Require().Equal(http.StatusOK, ts.usersMe(token.AccessToken))
		runAdministrationFlowWithInputs(ts.T(), suspendFlowHandle, map[string]string{
			subjectInput:      userID,
			operatorNoteInput: "Credential exposure reported by the security team",
		})
		// Watch past the deny list refresh the positive case waits on.
		ts.Never(func() bool { return ts.usersMe(token.AccessToken) != http.StatusOK },
			10*time.Second, 250*time.Millisecond,
			"with native API revocation off, /users/me does not consult the suspension's record")

		status, body := ts.usersMeBody(token.AccessToken)
		ts.Require().Equal(http.StatusOK, status)
		ts.Contains(body, "SUSPENDED", "the holder's read reports the suspension")
		ts.NotContains(body, "operatorNote", "the holder's read never carries the operator note")
		held := readStatus(ts.T(), userID)
		ts.Require().NotNil(held.Details)
		ts.Equal("Credential exposure reported by the security team", held.Details.OperatorNote,
			"a management read still carries it")
		ts.False(ts.tokenActive(token.AccessToken), "OAuth introspection still rejects the token")
	}
}

// switchDeployment sets one nested key in a deployment section and restarts, so the key is in force.
func (ts *ExistingAccessTestSuite) switchDeployment(section string, path []string, value interface{}) {
	ts.T().Helper()
	ts.patchDeploymentKey(section, path, value)
	ts.Require().NoError(testutils.RestartServer())
	ts.Require().NoError(testutils.ObtainAdminAccessToken())
}

// patchDeploymentKey sets one nested key in a deployment section without restarting.
func (ts *ExistingAccessTestSuite) patchDeploymentKey(section string, path []string, value interface{}) {
	ts.T().Helper()
	current, err := testutils.ReadDeploymentConfigKey(section)
	ts.Require().NoError(err)
	currentMap, ok := current.(map[string]interface{})
	ts.Require().True(ok, "deployment.yaml has no %s section", section)

	switched := copySection(currentMap)
	node := switched
	for _, key := range path[:len(path)-1] {
		child, _ := node[key].(map[string]interface{})
		child = copySection(child)
		node[key] = child
		node = child
	}
	node[path[len(path)-1]] = value
	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{section: switched}))
}

// deploymentSection reads one deployment section, for putting it back.
func (ts *ExistingAccessTestSuite) deploymentSection(section string) interface{} {
	ts.T().Helper()
	value, err := testutils.ReadDeploymentConfigKey(section)
	ts.Require().NoError(err)
	return value
}

// copySection copies one level of a deployment section, so a nested change leaves the original intact.
func copySection(section map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(section))
	for key, value := range section {
		out[key] = value
	}
	return out
}

// claim reads one claim from a JWT without verifying it.
func (ts *ExistingAccessTestSuite) claim(raw, name string) interface{} {
	ts.T().Helper()
	ts.Require().NotEmpty(raw)
	claims, err := testutils.DecodeJWTPayloadMap(raw)
	ts.Require().NoError(err)
	return claims[name]
}

// exchangeAccessToken exchanges a locally issued access token through the suite application.
func (ts *ExistingAccessTestSuite) exchangeAccessToken(subjectToken string) (int, *testutils.TokenResponse) {
	ts.T().Helper()
	status, body := ts.tokenRequest(url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token":      {subjectToken},
		"subject_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
	})
	var token testutils.TokenResponse
	if status == http.StatusOK {
		ts.Require().NoError(json.Unmarshal(body, &token))
	}
	return status, &token
}

// tokenRequest posts one grant to the token endpoint as the suite application.
func (ts *ExistingAccessTestSuite) tokenRequest(form url.Values) (int, []byte) {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(existingAccessClientID, existingAccessClientSecret)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, body
}

// userInfo calls UserInfo with a bearer token and returns the status and the subject it reports.
func (ts *ExistingAccessTestSuite) userInfo(token string) (int, string) {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodGet, testServerURL+"/oauth2/userinfo", nil)
	ts.Require().NoError(err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	var result struct {
		Sub string `json:"sub"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return resp.StatusCode, result.Sub
}

// usersMe calls the REST API's self endpoint with a bearer token and returns the status.
func (ts *ExistingAccessTestSuite) usersMe(token string) int {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodGet, testServerURL+"/users/me", nil)
	ts.Require().NoError(err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// usersMeBody calls the REST API's self endpoint with a bearer token and returns the status and body.
func (ts *ExistingAccessTestSuite) usersMeBody(token string) (int, string) {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodGet, testServerURL+"/users/me", nil)
	ts.Require().NoError(err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(body)
}

// introspectedSubject returns the subject introspection reports for an active token.
func (ts *ExistingAccessTestSuite) introspectedSubject(token string) string {
	ts.T().Helper()
	values := url.Values{"token": {token}}
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/introspect", strings.NewReader(values.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(existingAccessClientID, existingAccessClientSecret)
	response, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer response.Body.Close()
	var result struct {
		Active bool   `json:"active"`
		Sub    string `json:"sub"`
	}
	ts.Require().NoError(json.NewDecoder(response.Body).Decode(&result))
	ts.Require().True(result.Active, "the token must be active to report a subject")
	return result.Sub
}

func (ts *ExistingAccessTestSuite) tokenActive(token string) bool {
	ts.T().Helper()
	values := url.Values{"token": {token}}
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/introspect", strings.NewReader(values.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(existingAccessClientID, existingAccessClientSecret)
	response, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer response.Body.Close()
	ts.Require().Equal(http.StatusOK, response.StatusCode)
	var result struct {
		Active bool `json:"active"`
	}
	ts.Require().NoError(json.NewDecoder(response.Body).Decode(&result))
	return result.Active
}
