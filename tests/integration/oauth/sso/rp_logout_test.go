/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package sso

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestRPInitiatedLogoutEndsSession drives a full OIDC RP-Initiated Logout: after establishing an SSO
// session it hits the end_session_endpoint with a valid id_token_hint and a registered
// post_logout_redirect_uri, runs the sign-out flow the gate would run, and confirms the completion
// callback returns the post-logout redirect. It then asserts the SSO cookie is cleared and a fresh
// authorize re-prompts for credentials, proving the session was terminated.
func (ts *SSOLogoutTestSuite) TestRPInitiatedLogoutEndsSession() {
	client := ts.newSessionClient()

	idToken := ts.login(client, logoutUsername, "logout_state_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after login")

	// Initiate RP-initiated logout; the endpoint redirects the browser to the gate sign-out page.
	executionID, logoutID := ts.initiateLogout(client, idToken, postLogoutRedirectURI, "logout_state_2")
	ts.Require().NotEmpty(executionID, "the sign-out flow execution id should be present")
	ts.Require().NotEmpty(logoutID, "the logout id should be present")

	// Run the sign-out flow (what the gate does): it terminates the session and clears the cookie.
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-out flow should complete")

	// The completion callback consumes the stored request and returns the validated post-logout
	// redirect with state appended.
	redirect := ts.completeLogout(client, logoutID)
	parsed, err := url.Parse(redirect)
	ts.Require().NoError(err, "failed to parse post-logout redirect")
	ts.Equal(postLogoutRedirectURI, parsed.Scheme+"://"+parsed.Host+parsed.Path,
		"post-logout redirect should match the registered URI")
	ts.Equal("logout_state_2", parsed.Query().Get("state"), "state should be echoed on the post-logout redirect")

	// The sign-out flow cleared the per-flow cookie.
	ts.Empty(ts.ssoCookieNames(client), "the SSO cookie should be cleared after sign-out")

	// A fresh authorize now presents the credential prompt again: the SSO session is gone, so the
	// flow can no longer be skipped. Assert the exact prompt step (an INCOMPLETE credential VIEW)
	// rather than merely "not COMPLETE", which a failed or unrelated incomplete state would also
	// satisfy without proving re-authentication was requested.
	_, reAuthExecutionID := ts.authorize(client, "openid", "logout_state_3")
	reAuthStep := ts.flowExecute(client, map[string]interface{}{"executionId": reAuthExecutionID})
	ts.Equal("INCOMPLETE", reAuthStep.FlowStatus, "after sign-out, authorize must re-prompt for credentials")
	ts.Equal("VIEW", reAuthStep.Type, "the re-prompt should render the credential input view")
	ts.NotNil(reAuthStep.Data, "the credential prompt should carry view data")
	ts.Empty(reAuthStep.Assertion, "no assertion should be issued when credentials are still required")
}

// TestRPInitiatedLogoutRevokesTokenFamily proves that signing out revokes the login's token family:
// after logout the access token issued for that session is rejected by introspection, even though it
// has not yet expired.
func (ts *SSOLogoutTestSuite) TestRPInitiatedLogoutRevokesTokenFamily() {
	client := ts.newSessionClient()

	tokens := ts.loginTokens(client, logoutUsername, "logout_revoke_state_1")
	ts.Require().NotEmpty(tokens.AccessToken, "login should issue an access token")
	ts.Require().True(ts.introspectActive(client, tokens.AccessToken),
		"the access token should be active right after login")

	executionID, logoutID := ts.initiateLogout(client, tokens.IDToken, postLogoutRedirectURI, "logout_revoke_state_2")
	ts.Require().NotEmpty(executionID)
	ts.Require().NotEmpty(logoutID)

	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-out flow should complete")
	ts.completeLogout(client, logoutID)

	ts.False(ts.introspectActive(client, tokens.AccessToken),
		"signing out must revoke the session's token family, so its access token is no longer active")
}

// initiateLogout posts to the end_session_endpoint and returns the sign-out flow executionId and the
// logoutId carried on the gate sign-out redirect.
func (ts *SSOLogoutTestSuite) initiateLogout(
	client *http.Client, idTokenHint, postLogoutRedirect, state string,
) (string, string) {
	form := url.Values{}
	if idTokenHint != "" {
		form.Set("id_token_hint", idTokenHint)
	}
	if postLogoutRedirect != "" {
		form.Set("post_logout_redirect_uri", postLogoutRedirect)
	}
	if state != "" {
		form.Set("state", state)
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/logout", strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	ts.Require().NoError(err, "logout request failed")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusFound, resp.StatusCode,
		"logout should redirect to the gate sign-out page: %s", string(body))

	parsed, err := url.Parse(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to parse gate sign-out redirect")
	query := parsed.Query()
	return query.Get("executionId"), query.Get("logoutId")
}

// completeLogout posts the logout id to the completion callback and returns the post-logout redirect URI.
func (ts *SSOLogoutTestSuite) completeLogout(client *http.Client, logoutID string) string {
	body := strings.NewReader(`{"logoutId":"` + logoutID + `"}`)
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/logout/callback", body)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	ts.Require().NoError(err, "logout callback request failed")
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "logout callback failed: %s", string(respBody))

	var out struct {
		RedirectURI string `json:"redirect_uri"`
	}
	ts.Require().NoError(json.Unmarshal(respBody, &out), "failed to decode logout callback response")
	return out.RedirectURI
}
