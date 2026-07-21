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

import "github.com/thunder-id/thunderid/tests/integration/testutils"

// TestSSOSessionReuseSkipsAuthentication verifies the core SSO promise: once a per-flow session is
// established, a subsequent authorize on the same flow (carrying the SSO cookie) is satisfied without
// re-prompting for credentials. The initial /flow/execute step completes immediately, whereas a
// first-time login would return a credential prompt.
func (ts *SSOLogoutTestSuite) TestSSOSessionReuseSkipsAuthentication() {
	client := ts.newSessionClient()

	// First login establishes the SSO session and sets the per-flow cookie.
	ts.login(client, ssoReuseUsername, "reuse_state_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	// Second authorize with the SSO cookie present: SSO_CHECK finds the live session and the flow
	// completes on its initial step, skipping the credential prompt.
	_, executionID := ts.authorize(client, "openid", "reuse_state_2")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})

	ts.Equal("COMPLETE", step.FlowStatus, "second authorize should skip authentication via SSO")
	ts.NotEmpty(step.Assertion, "SSO-skipped flow should still yield an assertion")
}

// TestSSOReuse_ScopesPermissionsToRequestedResourceServer verifies that an
// SSO checkpoint established for resource server A must not carry A's resource-server binding into a
// later SSO-satisfied request targeting resource server B. The scope user is granted "read" on A only,
// so a second request bound to B (which defines the same "read") must not receive it.
func (ts *SSOLogoutTestSuite) TestSSOReuse_ScopesPermissionsToRequestedResourceServer() {
	client := ts.newSessionClient()

	// First login binds to resource server A, where the scope user holds "read".
	tokenA := ts.loginWithResource(client, ssoScopeUsername, "scope_state_1", "openid read", rsAIdentifier)
	ts.Require().Equal(rsAIdentifier, ts.tokenAudience(tokenA.AccessToken), "first token audience should be rs-A")
	ts.Require().Contains(ts.tokenScopes(tokenA.AccessToken), "read", "read is granted on rs-A")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	// Second authorize with the SSO cookie present, bound to resource server B. SSO_CHECK satisfies the
	// flow without a credential prompt; exchange the resulting code for a token bound to B.
	authID, executionID := ts.authorizeWithResource(client, "openid read", "scope_state_2", rsBIdentifier)
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "second authorize should skip authentication via SSO")
	ts.Require().NotEmpty(step.Assertion, "SSO-skipped flow should still yield an assertion")

	clientRedirect := ts.completeAuthorization(client, authID, step.Assertion)
	code, err := testutils.ExtractAuthorizationCode(clientRedirect)
	ts.Require().NoError(err, "failed to extract authorization code")
	tokenB := ts.exchangeCodeWithResource(client, code, rsBIdentifier)

	// rs-B defines the same "read" but the user has no grant there; the checkpoint from rs-A must not
	// leak it across the SSO-satisfied request.
	ts.Require().Equal(rsBIdentifier, ts.tokenAudience(tokenB.AccessToken), "second token audience should be rs-B")
	ts.Require().NotContains(ts.tokenScopes(tokenB.AccessToken), "read",
		"read must not leak from resource server A to B across SSO reuse")
}
