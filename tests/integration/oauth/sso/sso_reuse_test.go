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
