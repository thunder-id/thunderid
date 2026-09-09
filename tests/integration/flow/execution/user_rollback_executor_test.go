// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const userRollbackRedirectURI = "https://localhost:3000"

var (
	userRollbackOU = testutils.OrganizationUnit{
		Handle:      "user-rollback-test-ou",
		Name:        "User Rollback Test Organization Unit",
		Description: "Organization unit for UserRollbackExecutor rollback testing",
		Parent:      nil,
	}

	// userRollbackUserType marks username and email required (not just unique) so
	// ProvisioningExecutor's HasRequiredInputs check correctly pauses at prompt_credentials when
	// they are absent, the same fix TestProvisioningFailureRollsBackSilently needed.
	userRollbackUserType = testutils.UserType{
		Name: "user-rollback-test-user-type",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type":     "string",
				"unique":   true,
				"required": true,
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type":     "string",
				"unique":   true,
				"required": true,
			},
		},
	}

	// userRollbackAuthFlow lets the user this test provisions log in with real credentials, so the
	// test can obtain a genuine access token before triggering the rollback and confirm afterward
	// that the token no longer works. Mirrors createTestAuthenticationFlow in
	// tests/integration/oauth/introspect/introspect_test.go: action_001 is hardcoded inside
	// testutils.ObtainAccessTokenWithPassword, so this ref must match exactly.
	userRollbackAuthFlow = testutils.Flow{
		Name:     "User Rollback Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "user_rollback_test_auth_flow",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
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

	// userRollbackFlow provisions a user, pauses at a plain continuation prompt (giving this test a
	// chance to log the new user in and obtain a real access token before anything fails), then runs
	// an HTTP call that is deliberately misconfigured so it fails fast and deterministically without
	// any real network attempt. Its onFailure routes straight into UserRollbackExecutor: no PROMPT
	// node in between, since UserRollbackExecutor is one of the executors exempt from the general
	// "onFailure must target a PROMPT node" rule, the same way OUDeleteExecutor is.
	userRollbackFlow = testutils.Flow{
		Name:     "User Rollback Executor Test",
		FlowType: "REGISTRATION",
		Handle:   "user_rollback_executor_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "user_type_resolver",
			},
			{
				"id":   "user_type_resolver",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "UserTypeResolver",
				},
				"onSuccess":    "provisioning",
				"onIncomplete": "prompt_usertype",
			},
			{
				"id":   "prompt_usertype",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "usertype_input",
								"identifier": "userType",
								"type":       "SELECT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_usertype",
							"nextNode": "user_type_resolver",
						},
					},
				},
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
				},
				"onSuccess":    "continue_prompt",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_username",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
							{
								"ref":        "input_email",
								"identifier": "email",
								"type":       "EMAIL_INPUT",
								"required":   true,
							},
							{
								"ref":        "input_password",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_credentials",
							"nextNode": "provisioning",
						},
					},
				},
			},
			{
				"id":   "continue_prompt",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{},
						"action": map[string]interface{}{
							"ref":      "action_continue",
							"nextNode": "http_task",
						},
					},
				},
			},
			{
				"id":   "http_task",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"url":     "not-a-valid-url",
					"method":  "POST",
					"timeout": 3,
					"errorHandling": map[string]interface{}{
						"failOnError": true,
						"retryCount":  0,
						"retryDelay":  0,
					},
				},
				"executor": map[string]interface{}{
					"name": "HTTPRequestExecutor",
				},
				"onSuccess": "end",
				"onFailure": "user_rollback",
			},
			{
				"id":   "user_rollback",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "UserRollbackExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
)

// introspectResult is the parsed outcome of one call to the introspection endpoint. Mirrors
// tests/integration/oauth/introspect/introspect_test.go's own helper of the same shape.
type introspectResult struct {
	StatusCode int
	Body       map[string]any
}

// introspectPost introspects a token as a client_secret_post client.
func introspectPost(token, clientID, clientSecret string) (introspectResult, error) {
	form := url.Values{}
	form.Set("token", token)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/oauth2/introspect",
		strings.NewReader(form.Encode()))
	if err != nil {
		return introspectResult{}, fmt.Errorf("failed to create introspection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := testutils.GetRawHTTPClient().Do(req)
	if err != nil {
		return introspectResult{}, fmt.Errorf("failed to send introspection request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return introspectResult{}, fmt.Errorf("failed to read introspection response: %w", err)
	}

	result := introspectResult{StatusCode: resp.StatusCode}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &result.Body); err != nil {
			return introspectResult{}, fmt.Errorf("introspection body is not JSON: %s", string(body))
		}
	}
	return result, nil
}

func (r introspectResult) active() bool {
	value, _ := r.Body["active"].(bool)
	return value
}

// UserRollbackExecutorTestSuite verifies that a UserRollbackExecutor node, wired into a flow's own
// onFailure branch, actually rolls back a user provisioned earlier in the same execution: the user
// is deleted, and an access token obtained for that user before the rollback stops working
// immediately afterward.
type UserRollbackExecutorTestSuite struct {
	suite.Suite
	config         *common.TestSuiteConfig
	ouID           string
	userTypeID     string
	authFlowID     string
	registrationID string
	appID          string
	clientID       string
	clientSecret   string
}

func TestUserRollbackExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(UserRollbackExecutorTestSuite))
}

func (ts *UserRollbackExecutorTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	ts.clientID = fmt.Sprintf("user_rollback_test_client_%s", suffix)
	ts.clientSecret = "user_rollback_test_secret"

	// Suffix every unique fixture identifier so a repeat SetupSuite invocation against a shared,
	// non-reset service (rather than the throwaway per-run database the integration harness
	// normally provisions) does not collide with fixtures a prior run left behind.
	userRollbackOU.Name = fmt.Sprintf("User Rollback Test Organization Unit %s", suffix)
	userRollbackOU.Handle = fmt.Sprintf("user-rollback-test-ou-%s", suffix)
	ouID, err := testutils.CreateOrganizationUnit(userRollbackOU)
	ts.Require().NoError(err, "Failed to create test organization unit during setup")
	ts.ouID = ouID

	userRollbackUserType.Name = fmt.Sprintf("user-rollback-test-user-type-%s", suffix)
	userRollbackUserType.OUID = ts.ouID
	userRollbackUserType.AllowSelfRegistration = true
	userTypeID, err := testutils.CreateUserType(userRollbackUserType)
	ts.Require().NoError(err, "Failed to create user type during setup")
	ts.userTypeID = userTypeID

	userRollbackAuthFlow.Handle = fmt.Sprintf("user_rollback_test_auth_flow_%s", suffix)
	authFlowID, err := testutils.CreateFlow(userRollbackAuthFlow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	ts.authFlowID = authFlowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, authFlowID)

	userRollbackFlow.Handle = fmt.Sprintf("user_rollback_executor_test_%s", suffix)
	registrationID, err := testutils.CreateFlow(userRollbackFlow)
	ts.Require().NoError(err, "Failed to create user rollback test flow")
	ts.registrationID = registrationID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, registrationID)

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:                      "User Rollback Executor Test Application",
		Description:               "Application for testing UserRollbackExecutor rollback",
		OUID:                      ts.ouID,
		IsRegistrationFlowEnabled: true,
		AuthFlowID:                ts.authFlowID,
		RegistrationFlowID:        ts.registrationID,
		AllowedUserTypes:          []string{userRollbackUserType.Name},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                ts.clientID,
					"clientSecret":            ts.clientSecret,
					"redirectUris":            []string{userRollbackRedirectURI},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_post",
					"pkceRequired":            true,
				},
			},
		},
	})
	ts.Require().NoError(err, "Failed to create test application during setup")
	ts.appID = appID
}

func (ts *UserRollbackExecutorTestSuite) TearDownSuite() {
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// TestProvisioningSucceedsThenLaterFailureRollsBackUser reproduces the scenario in issue #5277: a
// user is provisioned successfully, then a later node in the same execution fails. It asserts the
// user is fully rolled back — deleted, and any access token they already obtained is revoked —
// rather than left behind as a live, loginable account with no compensating action.
func (ts *UserRollbackExecutorTestSuite) TestProvisioningSucceedsThenLaterFailureRollsBackUser() {
	username := common.GenerateUniqueUsername("userrollback")
	email := fmt.Sprintf("%s@example.com", username)
	password := "RollbackTest123!"

	flowStep, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"the flow should pause at prompt_credentials, since only one user type is allowed")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}, "action_submit_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"the flow should pause at continue_prompt right after provisioning succeeds")

	// Confirm provisioning genuinely succeeded before anything else runs.
	provisionedUser, findErr := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(findErr)
	ts.Require().NotNil(provisionedUser, "provisioning should have created the user before the later failure")

	// Log the newly provisioned user in through a completely separate authentication flow, exactly as
	// an end user abandoning the registration flow at this point could do, and obtain a real access
	// token for them.
	tokens, tokenErr := testutils.ObtainAccessTokenWithPassword(
		ts.clientID, userRollbackRedirectURI, "openid", username, password, true, ts.clientSecret)
	ts.Require().NoError(tokenErr, "the provisioned user should already be able to log in")
	ts.Require().NotEmpty(tokens.AccessToken)

	// Sanity check: the token is genuinely active before the rollback.
	before, introspectErr := introspectPost(tokens.AccessToken, ts.clientID, ts.clientSecret)
	ts.Require().NoError(introspectErr)
	ts.Require().True(before.active(), "the token must be active before the rollback runs")

	// Continue the original registration execution: the HTTP task fails deterministically, its
	// onFailure routes straight into UserRollbackExecutor, and the whole failure-and-rollback
	// sequence resolves within this single response, since there is no PROMPT node in between.
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, nil, "action_continue", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"onFailure routes straight into UserRollbackExecutor with no PROMPT in between")

	// The whole point of UserRollbackExecutor: the user provisioned above must now be gone.
	rolledBackUser, findErr := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(findErr)
	ts.Require().Nil(rolledBackUser, "UserRollbackExecutor should have deleted the provisioned user")

	// And the access token obtained before the rollback must no longer work, proving
	// RevokeByCriteria actually ran rather than the delete alone leaving old tokens usable.
	after, introspectErr := introspectPost(tokens.AccessToken, ts.clientID, ts.clientSecret)
	ts.Require().NoError(introspectErr)
	ts.Require().False(after.active(), "the token must be revoked once the user is rolled back")
}
