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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	existingAccessClientID     = "governance_existing_access_client"
	existingAccessClientSecret = "governance_existing_access_secret" //nolint:gosec // test credential
	existingAccessRedirectURI  = "https://localhost:3000/callback"

	// pausedClientID is the client whose flow prompts between the credentials and session nodes.
	pausedClientID = "governance_existing_access_paused_client"

	// continueAction submits that prompt.
	continueAction = "action_continue"
)

// revocationSwitches are the two deployment switches that decide where a suspension's revocation is
// enforced: OAuth (introspection and the token endpoint) and the native APIs.
type revocationSwitches struct {
	oauth  bool
	native bool
}

// switchesOn is the shipped deployment.
var switchesOn = revocationSwitches{oauth: true, native: true}

// switchesFor names the tests that run with a switch off. Every other test runs with both on.
var switchesFor = map[string]revocationSwitches{
	"TestWithOAuthRevocationOffIntrospectionMissesTheSuspension":     {oauth: false, native: true},
	"TestWithOAuthRevocationOffUserInfoRefusesASuspendedApplication": {oauth: false, native: true},
	"TestWithNativeRevocationOffTheRESTAPIMissesTheSuspension":       {oauth: true, native: false},
	"TestWithNativeRevocationOffTheSelfUpdateReportsTheSuspension":   {oauth: true, native: false},
}

// ExistingAccessTestSuite checks what each hold does to existing sessions and refresh grants: a
// suspension ends them; a lock leaves them alone, since anyone who knows a username can form one.
// Logins are browser-shaped, with a cookie jar carrying the SSO cookie between authorize calls.
type ExistingAccessTestSuite struct {
	suite.Suite
	users      *userFixture
	appID      string
	authFlowID string

	// The deployment's oauth and server sections as the suite found them, and the revocation switches
	// the running server has.
	pristineOAuth  map[string]interface{}
	pristineServer map[string]interface{}
	switches       revocationSwitches
}

func TestExistingAccessTestSuite(t *testing.T) {
	suite.Run(t, new(ExistingAccessTestSuite))
}

func (ts *ExistingAccessTestSuite) SetupSuite() {
	applyLockoutPolicy(ts.T())
	ts.switches = switchesOn
	ts.pristineOAuth = ts.deploymentSection("oauth")
	ts.pristineServer = ts.deploymentSection("server")

	ts.users = newUserFixture(ts.T(), "existing_access",
		testutils.UserType{Schema: credentialsSchema(map[string]interface{}{"email": stringAttribute})})

	authFlowID, err := testutils.CreateFlow(buildSSOAuthFlow("Governance Existing Access Test Auth Flow",
		"governance-existing-access-auth"))
	ts.Require().NoError(err, "failed to create the SSO authentication flow")
	ts.authFlowID = authFlowID

	appID, err := testutils.CreateApplication(ts.oauthApplication("Governance Existing Access Test App",
		existingAccessClientID, authFlowID))
	ts.Require().NoError(err, "failed to create the test application")
	ts.appID = appID
}

// BeforeTest puts the revocation switches as the test needs them. The server restarts only when that
// changes them, so the two switched tests cost one restart each and the restore one more.
func (ts *ExistingAccessTestSuite) BeforeTest(_, testName string) {
	want, listed := switchesFor[testName]
	if !listed {
		want = switchesOn
	}
	ts.Require().NoError(ts.useSwitches(want), "failed to set the revocation switches")
}

func (ts *ExistingAccessTestSuite) TearDownSuite() {
	if err := ts.useSwitches(switchesOn); err != nil {
		ts.T().Errorf("teardown: failed to restore the revocation switches: %v", err)
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("teardown: failed to delete application: %v", err)
		}
	}
	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", ts.authFlowID, err)
		}
	}
}

// A suspension ends both the SSO session and the refresh grant.
// Asserted separately, since either one left live is enough to regain access.
func (ts *ExistingAccessTestSuite) TestSuspendEndsExistingSessionsAndTokens() {
	userID, username, password := ts.newUser("suspend_existing")

	browser := ts.browser()
	token := browser.login(username, password, "existing_1")
	ts.Require().NotEmpty(token.RefreshToken, "the login must produce a refresh grant to revoke")
	ts.Require().NotEmpty(browser.ssoCookieNames(), "the login must establish an SSO session to end")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	ts.False(ts.refreshSucceeds(token.RefreshToken),
		"a suspension must revoke the refresh grant the account already held")
	ts.False(browser.sessionSatisfiesAuthorize("existing_2"),
		"a suspension must end the SSO session the account already held")
}

// A suspension placed after the credentials step but before the session node is refused there: the flow
// fails with the suspension stated, and no SSO session is saved for the account.
func (ts *ExistingAccessTestSuite) TestASuspensionBeforeTheSessionNodeIsRefusedThere() {
	flowID, err := testutils.CreateFlow(buildSSOAuthFlowPausedBeforeSession(
		"Governance Existing Access Paused Auth Flow", "governance-existing-access-paused-auth"))
	ts.Require().NoError(err, "failed to create the paused SSO authentication flow")
	cleanupFlow(ts.T(), flowID)
	appID, err := testutils.CreateApplication(ts.oauthApplication("Governance Existing Access Paused App",
		pausedClientID, flowID))
	ts.Require().NoError(err, "failed to create the paused application")
	cleanupApplication(ts.T(), appID)

	userID, username, password := ts.newUser("suspend_before_session")
	browser := newBrowser(ts.T(), pausedClientID, existingAccessClientSecret, existingAccessRedirectURI)
	_, paused := browser.signIn("before_session", map[string]string{"username": username, "password": password})
	ts.Require().Equal("INCOMPLETE", paused.FlowStatus, "precondition: the credentials must be accepted: %+v", paused)
	ts.Require().Nil(paused.Error, "precondition: the credentials must be accepted: %+v", paused.Error)
	ts.Require().Empty(browser.ssoCookieNames(), "precondition: no session before the session node")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	status, raw := postFlow(ts.T(), browser.client, "", map[string]interface{}{
		"executionId":    paused.ExecutionID,
		"inputs":         map[string]string{"nickname": "paused"},
		"action":         continueAction,
		"challengeToken": paused.ChallengeToken,
	})
	ts.Equal(flowAnswer{status: http.StatusOK, flowStatus: "ERROR", code: flowStatedSuspensionCode},
		answerOf(status, raw), "the session node must refuse with the suspension stated: %s", raw)
	ts.Empty(flowStepOf(ts.T(), status, raw).Assertion, "a refused session node must not sign the account in")
	ts.Empty(browser.ssoCookieNames(), "the session node must not hand the browser a session")
	ts.Zero(ssoSessionCount(ts.T(), userID), "the session node must not save a session for the account")
}

// Suspending again completes, keeps the first suspension's record, and leaves access revoked.
// A failed repeat revocation is unit-tested.
func (ts *ExistingAccessTestSuite) TestRepeatingASuspensionKeepsTheFirstRecord() {
	userID, username, password := ts.newUser("suspend_repeat")

	token := ts.browser().login(username, password, "existing_repeat")
	ts.Require().NotEmpty(token.RefreshToken, "the login must produce a refresh grant to revoke")

	runAdministrationFlowAs(ts.T(), adminToken(ts.T()), suspendFlowHandle, map[string]string{
		subjectInput:      userID,
		operatorNoteInput: "First note",
	})
	first := readStatus(ts.T(), userID)
	ts.Require().NotNil(first.Details)

	runAdministrationFlowAs(ts.T(), adminToken(ts.T()), suspendFlowHandle, map[string]string{
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

// A lock leaves the access token, the refresh grant and the SSO session alone.
// The lock is first checked to refuse fresh authentication, so this cannot pass with locking broken.
func (ts *ExistingAccessTestSuite) TestALockLeavesExistingSessionsAndTokensAlone() {
	userID, username, password := ts.newUser("lock_existing")

	browser := ts.browser()
	token := browser.login(username, password, "existing_3")
	ts.Require().NotEmpty(token.RefreshToken, "the login must produce a refresh grant")
	ts.Require().NotEmpty(browser.ssoCookieNames(), "the login must establish an SSO session")

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"the account must be locked before the criterion means anything")
	ts.Require().False(authenticate(ts.T(), "username", username, password).succeeded(),
		"the lock must actually be withholding a fresh authentication")

	ts.True(ts.tokenActive(token.AccessToken), "a lock must not revoke an existing access token")
	ts.True(ts.refreshSucceeds(token.RefreshToken),
		"a lock must not stop an existing refresh grant yielding tokens")
	ts.True(browser.sessionSatisfiesAuthorize("existing_4"),
		"a lock must not end an existing SSO session")
}

// Removing a restriction does not restore revoked sessions or tokens: after unsuspend the access
// token and the refresh grant stay dead and the SSO session does not come back.
func (ts *ExistingAccessTestSuite) TestReleasingTheSuspensionRestoresNothing() {
	userID, username, password := ts.newUser("restore_nothing")

	browser := ts.browser()
	token := browser.login(username, password, "existing_5")
	ts.Require().NotEmpty(token.RefreshToken, "the login must produce a refresh grant")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.Require().False(ts.tokenActive(token.AccessToken),
		"the access token must have been revoked before its non-restoration means anything")

	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	ts.Require().NotEqual("SUSPENDED", readStatus(ts.T(), userID).Value,
		"the suspension itself must have been released")

	ts.False(ts.tokenActive(token.AccessToken), "releasing a hold must not revive a revoked access token")
	ts.False(ts.refreshSucceeds(token.RefreshToken), "releasing a hold must not revive a revoked refresh grant")
	ts.False(browser.sessionSatisfiesAuthorize("existing_6"),
		"releasing a hold must not revive an ended SSO session")
}

// A second suspension after a release revokes the grants issued since: its cutoff is its own, not the
// first one's. The cutoff is the pre-suspend time plus jwt.leeway, so a grant from within the first
// window is revoked with the old ones, and the new one is issued once that has passed.
func (ts *ExistingAccessTestSuite) TestASuspensionAfterAReleaseRevokesTheGrantsIssuedSince() {
	userID, username, password := ts.newUser("suspend_again")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	cutoffBound := ts.suspendedSince(userID)
	runAdministrationFlow(ts.T(), unsuspendFlowHandle, userID)
	runAdministrationFlow(ts.T(), unlockFlowHandle, userID)
	waitPastTheCutoff(cutoffBound)

	fresh := ts.browser().login(username, password, "suspend_again")
	ts.Require().True(ts.tokenActive(fresh.AccessToken), "precondition: a grant past the cutoff is usable")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.False(ts.tokenActive(fresh.AccessToken), "a second suspension must revoke the newer access token")
	ts.False(ts.refreshSucceeds(fresh.RefreshToken), "a second suspension must revoke the newer refresh grant")
}

// Suspending the application a user signed in through revokes that user's tokens, as deleting it
// does. Once released and past the cutoff, a fresh sign-in through the application works again.
// Introspection authenticates as the same client, so the old tokens are checked after the release.
func (ts *ExistingAccessTestSuite) TestSuspendingTheApplicationRevokesTheUserTokensItIssued() {
	_, username, password := ts.newUser("app_suspend")
	token := ts.browser().login(username, password, "app_suspend_1")
	ts.Require().True(ts.tokenActive(token.AccessToken), "the token must be usable before the suspension")
	// Client credentials: the application's own token names its entity ID, so the entity criterion
	// matches it as well as the client ID.
	ownResult := clientCredentials(ts.T(), existingAccessClientID, existingAccessClientSecret, "")
	ts.Require().Equal(http.StatusOK, ownResult.StatusCode, "client credentials failed: %s", ownResult.Body)
	own := ownResult.Token
	ts.Equal(ts.appID, ts.claim(own.AccessToken, "sub"), "a client credentials token must name the application")
	ts.Require().True(ts.tokenActive(own.AccessToken))

	ts.T().Cleanup(ts.releaseTheApplicationIfSuspended)
	runAdministrationFlow(ts.T(), applicationSuspendFlowHandle, ts.appID)
	// An application read carries no placement time, so the flow's return bounds the pre-check instead.
	cutoffBound := time.Now()
	status, _ := ts.userInfo(token.AccessToken)
	ts.Equal(http.StatusUnauthorized, status, "UserInfo must refuse a token issued through a suspended application")
	runAdministrationFlow(ts.T(), applicationUnsuspendFlowHandle, ts.appID)

	ts.False(ts.tokenActive(token.AccessToken),
		"an application suspension must revoke the access tokens users obtained through it")
	ts.False(ts.refreshSucceeds(token.RefreshToken),
		"an application suspension must revoke the refresh grants users obtained through it")
	ts.False(ts.tokenActive(own.AccessToken),
		"an application suspension must revoke the application's own client credentials token")

	waitPastTheCutoff(cutoffBound)
	fresh := ts.browser().login(username, password, "app_suspend_2")
	ts.True(ts.tokenActive(fresh.AccessToken), "a token issued after the release must be usable")
	ts.True(ts.refreshSucceeds(fresh.RefreshToken), "a refresh grant issued after the release must be usable")
	again := clientCredentials(ts.T(), existingAccessClientID, existingAccessClientSecret, "")
	ts.Require().Equal(http.StatusOK, again.StatusCode, "client credentials failed: %s", again.Body)
	ts.True(ts.tokenActive(again.Token.AccessToken),
		"a client credentials token issued after the release must be usable")
}

// releaseTheApplicationIfSuspended unsuspends the suite application when a failure left it suspended,
// and waits past the cutoff, so the next test can sign in through it.
func (ts *ExistingAccessTestSuite) releaseTheApplicationIfSuspended() {
	status, err := testutils.GetAccessStatus(applicationCollection, ts.appID)
	if err != nil || status.Value != "SUSPENDED" {
		return
	}
	answer := attemptAdministrationFlow(ts.T(), applicationUnsuspendFlowHandle, ts.appID)
	if answer.status != http.StatusOK || answer.flowStatus != "COMPLETE" {
		ts.T().Errorf("cleanup: failed to unsuspend the application: %+v", answer)
		return
	}
	waitPastTheCutoff(time.Now())
}

// Every token path names the entity ID in `sub`, so a suspension rejects what each issued
// before it: code, refresh, token exchange, introspection, UserInfo and the REST API. Client
// credentials is covered in the application test above.
func (ts *ExistingAccessTestSuite) TestEveryTokenPathNamesTheEntityAndSuspensionRevokesIt() {
	userID, username, password := ts.newUser("token_paths")
	token := ts.browser().login(username, password, "token_paths")

	ts.Equal(userID, ts.claim(token.AccessToken, "sub"), "code: the access token must name the entity")
	ts.Equal(userID, ts.claim(token.IDToken, "sub"), "code: the ID token must name the entity")
	ts.Equal(userID, ts.claim(token.RefreshToken, "access_token_sub"),
		"code: the refresh token must carry the entity as the subject it issues for")

	refreshed, err := testutils.RefreshAccessTokenRaw(existingAccessClientID, existingAccessClientSecret,
		token.RefreshToken, "")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, refreshed.StatusCode, "refresh must succeed before the suspension")
	ts.Equal(userID, ts.claim(refreshed.Token.AccessToken, "sub"),
		"refresh: the new access token must name the entity")

	exchanged := exchangeAccessToken(ts.T(), existingAccessClientID, existingAccessClientSecret, token.AccessToken)
	ts.Require().Equal(http.StatusOK, exchanged.StatusCode, "token exchange must succeed before the suspension: %s",
		exchanged.Body)
	ts.Equal(userID, ts.claim(exchanged.Token.AccessToken, "sub"), "exchange: the issued token must name the entity")

	introspected, err := testutils.IntrospectTokenResult(token.AccessToken, existingAccessClientID,
		existingAccessClientSecret, false)
	ts.Require().NoError(err)
	ts.Require().True(introspected.Active, "the token must be active to report a subject")
	ts.Equal(userID, introspected.Sub, "introspection must report the entity")
	status, sub := ts.userInfo(token.AccessToken)
	ts.Require().Equal(http.StatusOK, status, "UserInfo must answer before the suspension")
	ts.Equal(userID, sub, "UserInfo must report the entity")
	ts.Equal(http.StatusOK, ts.usersMe(token.AccessToken), "the REST API must accept the token before the suspension")

	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)
	ts.Eventually(func() bool { return ts.usersMe(token.AccessToken) == http.StatusUnauthorized },
		denyListRefreshWait, 250*time.Millisecond, "the REST API must refuse a revoked token")

	for name, raw := range map[string]string{
		"code": token.AccessToken, "refresh": refreshed.Token.AccessToken, "exchange": exchanged.Token.AccessToken,
	} {
		ts.False(ts.tokenActive(raw), "%s: the access token must be revoked", name)
	}
	ts.False(ts.refreshSucceeds(refreshed.Token.RefreshToken), "refresh: the rotated grant must be revoked")
	refused := exchangeAccessToken(ts.T(), existingAccessClientID, existingAccessClientSecret, token.AccessToken)
	requireOAuthError(ts.T(), refused.StatusCode, string(refused.Body), http.StatusBadRequest, "invalid_request",
		"Invalid subject_token", "exchange: a revoked subject token must be refused as an invalid subject token")
	status, _ = ts.userInfo(token.AccessToken)
	ts.Equal(http.StatusUnauthorized, status, "UserInfo must refuse a revoked token")
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

	browser := ts.browser()
	code := browser.signInForCode(username, password, "inside_cutoff")
	// The pre-check ran after beforeSuspend, so the cutoff is no earlier than beforeSuspend plus the
	// leeway, less the precision the code's creation is compared at. A code issued later proves nothing.
	ts.Require().True(time.Now().Before(beforeSuspend.Add(suspensionCutoffLeeway-timestampPrecision)),
		"the release and sign-in must land inside the cutoff; a runner this slow needs a longer "+
			"suspensionCutoffLeeway")
	status, body := browser.redeem(code)
	requireOAuthError(ts.T(), status, body, http.StatusBadRequest, "invalid_grant",
		"The authorization grant has been revoked", "a code issued inside the cutoff must not redeem")

	waitPastTheCutoff(cutoffBound)
	fresh := ts.browser().login(username, password, "past_cutoff")
	ts.True(ts.tokenActive(fresh.AccessToken), "a sign-in past the cutoff must be usable")
}

// Governance adds no claims: the tokens carry no session or grant claim, and `sub` is the entity ID.
func (ts *ExistingAccessTestSuite) TestTokenClaimsStayStandard() {
	userID, username, password := ts.newUser("standard_claims")
	token := ts.browser().login(username, password, "standard_claims")
	for name, raw := range map[string]string{"access": token.AccessToken, "refresh": token.RefreshToken,
		"id": token.IDToken} {
		ts.Require().NotEmpty(raw, "the %s token is issued", name)
		claims, err := testutils.DecodeJWTPayloadMap(raw)
		ts.Require().NoError(err)
		ts.NotContains(claims, "seid", "the %s token carries no session claim", name)
		ts.NotContains(claims, "Grant", "the %s token carries no grant claim", name)
		ts.NotContains(claims, "grant", "the %s token carries no grant claim", name)
	}
	ts.Equal(userID, ts.claim(token.AccessToken, "sub"), "the access token's subject is the entity ID")
	ts.True(ts.tokenActive(token.AccessToken), "the access token introspects as active")
}

// With OAuth revocation off, an access token issued before a suspension still introspects as active,
// while refresh is still refused by its live check.
func (ts *ExistingAccessTestSuite) TestWithOAuthRevocationOffIntrospectionMissesTheSuspension() {
	userID, username, password := ts.newUser("oauth_switch_off")
	token := ts.browser().login(username, password, "oauth_switch_off")
	runAdministrationFlow(ts.T(), suspendFlowHandle, userID)

	ts.True(ts.tokenActive(token.AccessToken),
		"with OAuth revocation off, introspection does not consult the suspension's record")
	ts.False(ts.refreshSucceeds(token.RefreshToken), "refresh is still refused by its live check")
}

// With OAuth revocation off, only UserInfo's own client check can refuse the token.
func (ts *ExistingAccessTestSuite) TestWithOAuthRevocationOffUserInfoRefusesASuspendedApplication() {
	userID, username, password := ts.newUser("app_userinfo_switch_off")
	token := ts.browser().login(username, password, "app_userinfo_switch_off")
	status, _ := ts.userInfo(token.AccessToken)
	ts.Require().Equal(http.StatusOK, status, "precondition: UserInfo answers before the suspension")

	ts.T().Cleanup(ts.releaseTheApplicationIfSuspended)
	runAdministrationFlow(ts.T(), applicationSuspendFlowHandle, ts.appID)
	status, _ = ts.userInfo(token.AccessToken)
	ts.Equal(http.StatusUnauthorized, status, "UserInfo must refuse a token of a suspended application")

	runAdministrationFlow(ts.T(), applicationUnsuspendFlowHandle, ts.appID)
	status, sub := ts.userInfo(token.AccessToken)
	ts.Equal(http.StatusOK, status, "UserInfo must answer once the application is released")
	ts.Equal(userID, sub, "UserInfo must report the entity")
	waitPastTheCutoff(time.Now())
}

// With native API revocation off, `/users/me` still accepts a token issued before a suspension, and
// reports the suspension without the operator note, while OAuth introspection rejects the token.
func (ts *ExistingAccessTestSuite) TestWithNativeRevocationOffTheRESTAPIMissesTheSuspension() {
	userID, username, password := ts.newUser("native_switch_off")
	token := ts.browser().login(username, password, "native_switch_off")
	ts.Require().Equal(http.StatusOK, ts.usersMe(token.AccessToken))
	runAdministrationFlowAs(ts.T(), adminToken(ts.T()), suspendFlowHandle, map[string]string{
		subjectInput:      userID,
		operatorNoteInput: "Credential exposure reported by the security team",
	})
	// With the switch off the server builds no copy of the deny list, so no refresh exists to await;
	// the watch lasts as long as the enforced case is given to refuse.
	ts.Never(func() bool { return ts.usersMe(token.AccessToken) != http.StatusOK },
		denyListRefreshWait, 250*time.Millisecond,
		"with native API revocation off, /users/me does not consult the suspension's record")

	status, body := getWith(ts.T(), testutils.GetHTTPClientWithToken(token.AccessToken), "/users/me")
	ts.Require().Equal(http.StatusOK, status)
	ts.Contains(body, "SUSPENDED", "the holder's read reports the suspension")
	ts.NotContains(body, "operatorNote", "the holder's read never carries the operator note")
	held := readStatus(ts.T(), userID)
	ts.Require().NotNil(held.Details)
	ts.Equal("Credential exposure reported by the security team", held.Details.OperatorNote,
		"a management read still carries it")
	ts.False(ts.tokenActive(token.AccessToken), "OAuth introspection still rejects the token")
}

// Native API revocation is off so that the suspended holder's token still reaches /users/me.
func (ts *ExistingAccessTestSuite) TestWithNativeRevocationOffTheSelfUpdateReportsTheSuspension() {
	userID, username, password := ts.newUser("native_switch_off_update")
	token := ts.browser().login(username, password, "native_switch_off_update")
	runAdministrationFlowAs(ts.T(), adminToken(ts.T()), suspendFlowHandle, map[string]string{
		subjectInput:      userID,
		operatorNoteInput: "Credential exposure reported by the security team",
	})
	held := readStatus(ts.T(), userID)
	ts.Require().Equal("SUSPENDED", held.Value, "precondition: the suspension is in force")
	ts.Require().NotNil(held.Details)
	ts.Require().NotEmpty(held.Details.OperatorNote, "precondition: a management read carries the note")

	payload, err := json.Marshal(map[string]interface{}{
		"attributes": map[string]interface{}{"email": "native_switch_off_update@example.com"},
	})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPut, testServerURL+"/users/me", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClientWithToken(token.AccessToken).Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "the self update must answer: %s", string(body))

	ts.NotContains(string(body), "operatorNote", "the holder's update never carries the operator note")
	var updated struct {
		Status statusReport `json:"status"`
	}
	ts.Require().NoError(json.Unmarshal(body, &updated), string(body))
	withoutNote := *held.Details
	withoutNote.OperatorNote = ""
	ts.Equal(statusReport{Value: "SUSPENDED", Details: &withoutNote}, updated.Status,
		"the self update must report the suspension a read reports, without the note")
}

// useSwitches restarts the server with the revocation switches as wanted, unless it already has them.
func (ts *ExistingAccessTestSuite) useSwitches(want revocationSwitches) error {
	if want == ts.switches {
		return nil
	}
	oauth, server := ts.pristineOAuth, ts.pristineServer
	if !want.oauth {
		oauth = withKey(oauth, []string{"token_revocation", "enabled"}, false)
	}
	if !want.native {
		server = withKey(server, []string{"security", "token_revocation", "enabled"}, false)
	}
	if err := testutils.PatchDeploymentConfig(map[string]interface{}{"oauth": oauth, "server": server}); err != nil {
		return err
	}
	if err := testutils.RestartServer(); err != nil {
		return err
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		return err
	}
	ts.switches = want
	return nil
}

// withKey returns a copy of a deployment section with one nested key set, leaving the original intact.
func withKey(section map[string]interface{}, path []string, value interface{}) map[string]interface{} {
	copied := make(map[string]interface{}, len(section)+1)
	for key, existing := range section {
		copied[key] = existing
	}
	if len(path) == 1 {
		copied[path[0]] = value
		return copied
	}
	child, _ := copied[path[0]].(map[string]interface{})
	copied[path[0]] = withKey(child, path[1:], value)
	return copied
}

// deploymentSection reads one deployment section, for putting it back.
func (ts *ExistingAccessTestSuite) deploymentSection(section string) map[string]interface{} {
	ts.T().Helper()
	value, err := testutils.ReadDeploymentConfigKey(section)
	ts.Require().NoError(err)
	sectionMap, ok := value.(map[string]interface{})
	ts.Require().True(ok, "deployment.yaml has no %s section", section)
	return sectionMap
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

// timestampPrecision is the precision a token's iat, and a code's creation time, is compared with a
// cutoff at.
const timestampPrecision = time.Second

// waitPastTheCutoff sleeps until a suspension whose pre-check ran no later than bound has a cutoff
// in the past, with twice the timestamp precision to spare.
func waitPastTheCutoff(bound time.Time) {
	time.Sleep(time.Until(bound.Add(suspensionCutoffLeeway).Add(2 * timestampPrecision)))
}

// oauthApplication is an application in the suite's unit that signs in through authFlowID as the OAuth
// client clientID, with the suite's secret and redirect URI.
func (ts *ExistingAccessTestSuite) oauthApplication(name, clientID, authFlowID string) testutils.Application {
	return testutils.Application{
		OUID:             ts.users.ouID,
		Name:             name,
		Description:      "Application the existing-access acceptance tests sign in through",
		AuthFlowID:       authFlowID,
		AllowedUserTypes: []string{ts.users.userType},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":     clientID,
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
	}
}

// ssoSessionCount counts the SSO sessions saved for the subject, in any state.
func ssoSessionCount(t *testing.T, subjectID string) int {
	t.Helper()
	raw, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT COUNT(*) FROM "SSO_SESSION" WHERE SUBJECT_ID = '%s';`, subjectID))
	require.NoError(t, err, "failed to read the SSO sessions")
	count, err := strconv.Atoi(raw)
	require.NoError(t, err, "unreadable SSO session count %q", raw)
	return count
}

// newUser creates one account with an email address nobody else in this package touches.
func (ts *ExistingAccessTestSuite) newUser(name string) (userID, username, password string) {
	ts.T().Helper()
	userID, username, password = ts.users.newUser(ts.T(), name, nil)
	return userID, username, password
}

// browser is a fresh browser signing in through the suite application.
func (ts *ExistingAccessTestSuite) browser() *browser {
	return newBrowser(ts.T(), existingAccessClientID, existingAccessClientSecret, existingAccessRedirectURI)
}

// tokenActive reports whether introspection, as the suite application, finds the token active.
func (ts *ExistingAccessTestSuite) tokenActive(token string) bool {
	ts.T().Helper()
	return tokenActive(ts.T(), token, existingAccessClientID, existingAccessClientSecret)
}

// refreshSucceeds reports whether the refresh grant still yields tokens.
func (ts *ExistingAccessTestSuite) refreshSucceeds(refreshToken string) bool {
	ts.T().Helper()

	result, err := testutils.RefreshAccessTokenRaw(existingAccessClientID, existingAccessClientSecret,
		refreshToken, "")
	ts.Require().NoError(err, "the refresh request itself must reach the server")
	return result.StatusCode == http.StatusOK && result.Token != nil && result.Token.AccessToken != ""
}

// claim reads one claim from a JWT without verifying it.
func (ts *ExistingAccessTestSuite) claim(raw, name string) interface{} {
	ts.T().Helper()
	ts.Require().NotEmpty(raw)
	claims, err := testutils.DecodeJWTPayloadMap(raw)
	ts.Require().NoError(err)
	return claims[name]
}

// exchangeAccessToken exchanges a locally issued access token as the given client and returns the
// answer whatever it is.
func exchangeAccessToken(t *testing.T, clientID, clientSecret, subjectToken string) *testutils.TokenHTTPResult {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token", strings.NewReader(url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token":      {subjectToken},
		"subject_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
	}.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	result := &testutils.TokenHTTPResult{StatusCode: resp.StatusCode, Body: body}
	if resp.StatusCode == http.StatusOK {
		result.Token = &testutils.TokenResponse{}
		require.NoError(t, json.Unmarshal(body, result.Token))
	}
	return result
}

// userInfo calls UserInfo with a bearer token and returns the status and the subject it reports.
// /oauth2/ is public to the shared clients, so the token goes on by hand.
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
	status, _ := getWith(ts.T(), testutils.GetHTTPClientWithToken(token), "/users/me")
	return status
}

// buildSSOAuthFlow is an authentication flow with a reusable SSO session: SSO_CHECK short-circuits
// to the session node when one is live, and SessionExecutor establishes it on a first login.
func buildSSOAuthFlow(name, handle string) testutils.Flow {
	return testutils.Flow{
		Name:     name,
		Handle:   handle,
		FlowType: "AUTHENTICATION",
		Nodes: []map[string]interface{}{
			startNode("sso_check"),
			taskNode("sso_check", "SSOCheckExecutor", "", "session_main", map[string]interface{}{
				"properties": map[string]interface{}{"checkpointRef": "session_main"},
				"onFailure":  "prompt_credentials",
			}),
			promptNode("prompt_credentials", signInAction, "credentials_auth", usernameInput, passwordInput),
			withExecutorInputs(taskNode("credentials_auth", "CredentialsAuthExecutor", "", "session_main",
				map[string]interface{}{"onIncomplete": "prompt_credentials"}), usernameInput, passwordInput),
			taskNode("session_main", "SessionExecutor", "", "authorization_check", nil),
			taskNode("authorization_check", "AuthorizationExecutor", "", "auth_assert", nil),
			taskNode("auth_assert", "AuthAssertExecutor", "", "end", nil),
			endNode(),
		},
	}
}

// buildSSOAuthFlowPausedBeforeSession is buildSSOAuthFlow with a prompt between the credentials node
// and the session node, so a test can act after the credentials are accepted. The prompt checks nothing.
func buildSSOAuthFlowPausedBeforeSession(name, handle string) testutils.Flow {
	flow := buildSSOAuthFlow(name, handle)
	nodes, _ := flow.Nodes.([]map[string]interface{})
	for _, node := range nodes {
		if node["id"] == "credentials_auth" {
			node["onSuccess"] = "prompt_continue"
		}
	}
	flow.Nodes = append(nodes, promptNode("prompt_continue", continueAction, "session_main",
		flowInput("input_nickname", "nickname", "TEXT_INPUT")))
	return flow
}
