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

package authz

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestIssuerIdentifier_GET_ErrorRedirect_IssPresent tests that the iss parameter is present in
// GET /oauth2/authorize error redirects per RFC 9207.
func (ts *AuthzTestSuite) TestIssuerIdentifier_GET_ErrorRedirect_IssPresent() {
	resp, err := testutils.InitiateAuthorizationFlow(clientID, redirectURI, "invalid_type", "openid", "iss_test_state")
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.NotEmpty(location, "Location header must be set on error redirect")

	parsedURL, err := url.Parse(location)
	ts.Require().NoError(err)

	params := parsedURL.Query()
	ts.Equal("unsupported_response_type", params.Get("error"), "error param must match")
	ts.NotEmpty(params.Get("iss"), "iss must be present in GET error redirect")
	ts.Equal("iss_test_state", params.Get("state"), "state must be echoed back")
}

// TestIssuerIdentifier_GET_ErrorRedirect_IssUnconditional tests that the iss parameter is present in
// GET /oauth2/authorize error redirects even when no state parameter is provided.
func (ts *AuthzTestSuite) TestIssuerIdentifier_GET_ErrorRedirect_IssUnconditional() {
	resp, err := testutils.InitiateAuthorizationFlow(clientID, redirectURI, "invalid_type", "openid", "")
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.NotEmpty(location)

	parsedURL, err := url.Parse(location)
	ts.Require().NoError(err)

	params := parsedURL.Query()
	ts.NotEmpty(params.Get("iss"), "iss must be present even when state is absent")
	ts.Empty(params.Get("state"), "state must be absent when not sent in request")
}

// TestIssuerIdentifier_POST_CallbackSuccess_IssPresent tests that the iss parameter is present in
// the POST /oauth2/auth/callback success redirect URI per RFC 9207.
func (ts *AuthzTestSuite) TestIssuerIdentifier_POST_CallbackSuccess_IssPresent() {
	username := "iss_test_user_success"
	password := "testpass123"

	user := testutils.User{
		OUID: testOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "%s@example.com",
			"given_name": "Iss",
			"family_name": "Test"
		}`, username, password, username)),
	}

	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err)
	defer func() {
		_ = testutils.DeleteUser(userID)
	}()

	authzResponse := completeFullAuthorizationFlow(ts, username, password, "iss_success_state")

	parsedURL, err := url.Parse(authzResponse.RedirectURI)
	ts.Require().NoError(err)

	params := parsedURL.Query()
	ts.NotEmpty(params.Get("code"), "code must be present in success redirect")
	ts.NotEmpty(params.Get("iss"), "iss must be present in POST callback success redirect")
	ts.Equal("iss_success_state", params.Get("state"), "state must be echoed back")
}

// TestIssuerIdentifier_POST_CallbackSuccess_NoState_IssPresent tests that the iss parameter is present
// in the POST /oauth2/auth/callback success redirect even when no state parameter is provided.
func (ts *AuthzTestSuite) TestIssuerIdentifier_POST_CallbackSuccess_NoState_IssPresent() {
	username := "iss_test_user_nostate"
	password := "testpass123"

	user := testutils.User{
		OUID: testOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "%s@example.com",
			"given_name": "Iss",
			"family_name": "NoState"
		}`, username, password, username)),
	}

	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err)
	defer func() {
		_ = testutils.DeleteUser(userID)
	}()

	authzResponse := completeFullAuthorizationFlow(ts, username, password, "")

	parsedURL, err := url.Parse(authzResponse.RedirectURI)
	ts.Require().NoError(err)

	params := parsedURL.Query()
	ts.NotEmpty(params.Get("code"), "code must be present in success redirect")
	ts.NotEmpty(params.Get("iss"), "iss must be present even when state is absent")
	ts.Empty(params.Get("state"), "state must be absent when not sent in request")
}

// completeFullAuthorizationFlow completes the full authorization code flow and returns the
// authorization response containing the redirect URI.
func completeFullAuthorizationFlow(ts *AuthzTestSuite, username, password, state string) *testutils.AuthorizationResponse {
	resp, err := testutils.InitiateAuthorizationFlow(clientID, redirectURI, "code", "openid", state)
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode, "Expected redirect status")

	location := resp.Header.Get("Location")
	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract auth data")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow step")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": username,
		"password": password,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to execute authentication flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Flow must complete successfully")

	authzResponse, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err, "Failed to complete authorization")

	return authzResponse
}
