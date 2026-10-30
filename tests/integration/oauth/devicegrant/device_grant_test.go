// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package devicegrant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	deviceGrantType           = "urn:ietf:params:oauth:grant-type:device_code"
	deviceAuthorizeEndpoint   = "/oauth2/device-authorize"
	deviceCallbackEndpoint    = "/oauth2/auth/callback"
	deviceTokenEndpoint       = "/oauth2/token"
	deviceGrantClientID       = "device_grant_integration_client"
	deviceGrantClientSecret   = "device_grant_integration_secret"
	deviceGrantUsername       = "device_grant_integration_user"
	deviceGrantPassword       = "device-grant-password"
	deviceGrantUserTypeHandle = "device-grant-integration-person"
)

type deviceAuthorizationResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
	Error                   string `json:"error"`
}

type deviceTokenResult struct {
	statusCode  int
	accessToken string
	idToken     string
	errorCode   string
}

type DeviceGrantTestSuite struct {
	suite.Suite
	client            *http.Client
	ouID              string
	userTypeID        string
	userID            string
	authFlowID        string
	deviceGrantFlowID string
	appID             string
}

func TestDeviceGrantTestSuite(t *testing.T) {
	suite.Run(t, new(DeviceGrantTestSuite))
}

func (ts *DeviceGrantTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "device-grant-integration-ou",
		Name:        "Device Grant Integration OU",
		Description: "Organization unit for Device Grant integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      deviceGrantUserTypeHandle,
		DisplayName: "Device Grant Integration Person",
		OUID:        ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err)
	ts.userTypeID = userTypeID

	userID, err := testutils.CreateUser(testutils.User{
		OUID: ts.ouID,
		Type: deviceGrantUserTypeHandle,
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": %q,
			"password": %q,
			"email": "device-grant-integration@example.com"
		}`, deviceGrantUsername, deviceGrantPassword)),
	})
	ts.Require().NoError(err)
	ts.userID = userID

	authFlowID, err := testutils.CreateFlow(deviceGrantAuthenticationFlow(ts.ouID))
	ts.Require().NoError(err)
	ts.authFlowID = authFlowID

	deviceFlowID, err := testutils.CreateFlow(deviceGrantApprovalFlow(ts.ouID, authFlowID))
	ts.Require().NoError(err)
	ts.deviceGrantFlowID = deviceFlowID

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:              ts.ouID,
		Name:              "Device Grant Integration Application",
		Description:       "Application for Device Grant integration tests",
		Type:              "fullstack",
		AuthFlowID:        ts.authFlowID,
		DeviceGrantFlowID: ts.deviceGrantFlowID,
		AllowedUserTypes:  []string{deviceGrantUserTypeHandle},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                deviceGrantClientID,
					"clientSecret":            deviceGrantClientSecret,
					"redirectUris":            []string{"https://localhost:3000/callback"},
					"grantTypes":              []string{deviceGrantType},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err)
	ts.appID = appID
}

func (ts *DeviceGrantTestSuite) TearDownSuite() {
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	if ts.deviceGrantFlowID != "" {
		_ = testutils.DeleteFlow(ts.deviceGrantFlowID)
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.userTypeID != "" {
		_ = testutils.DeleteUserType(ts.userTypeID)
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete Device Grant test organization unit: %v", err)
		}
	}
}

// TestDeviceGrantLifecycle exercises the complete browser and device sides of RFC 8628. It verifies
// request creation, pending polling, rate limiting, user-code verification, the application-assigned
// Authentication flow, callback binding, token issuance, and one-time request consumption.
func (ts *DeviceGrantTestSuite) TestDeviceGrantLifecycle() {
	authStatus, authResponse, headers := ts.deviceAuthorize("openid")
	ts.Require().Equal(http.StatusOK, authStatus)
	ts.Require().NotEmpty(authResponse.DeviceCode)
	ts.Require().NotEmpty(authResponse.UserCode)
	ts.Require().Positive(authResponse.ExpiresIn)
	ts.Require().Positive(authResponse.Interval)
	ts.Equal("no-store", headers.Get("Cache-Control"))
	ts.Equal("no-cache", headers.Get("Pragma"))

	verificationURI, err := url.Parse(authResponse.VerificationURI)
	ts.Require().NoError(err)
	ts.Equal(ts.appID, verificationURI.Query().Get("applicationId"))
	completeURI, err := url.Parse(authResponse.VerificationURIComplete)
	ts.Require().NoError(err)
	ts.Equal(ts.appID, completeURI.Query().Get("applicationId"))
	ts.Equal(authResponse.UserCode, completeURI.Query().Get("user_code"))

	pending := ts.pollToken(authResponse.DeviceCode)
	ts.Equal(http.StatusBadRequest, pending.statusCode)
	ts.Equal("authorization_pending", pending.errorCode)

	slowDown := ts.pollToken(authResponse.DeviceCode)
	ts.Equal(http.StatusBadRequest, slowDown.statusCode)
	ts.Equal("slow_down", slowDown.errorCode)

	codePrompt := ts.startDeviceGrantFlow()
	ts.Equal("INCOMPLETE", codePrompt.FlowStatus)
	ts.Require().NotNil(codePrompt.Data)
	ts.Require().NotEmpty(codePrompt.Data.Inputs)
	ts.Equal("user_code", codePrompt.Data.Inputs[0].Identifier)

	authPrompt, err := testutils.ExecuteAuthenticationFlow(codePrompt.ExecutionID,
		map[string]string{"user_code": authResponse.UserCode}, "device-code-submit", codePrompt.ChallengeToken)
	ts.Require().NoError(err)
	ts.Equal("INCOMPLETE", authPrompt.FlowStatus)

	completed, err := testutils.ExecuteAuthenticationFlow(authPrompt.ExecutionID, map[string]string{
		"username": deviceGrantUsername,
		"password": deviceGrantPassword,
	}, "action_credentials", authPrompt.ChallengeToken)
	ts.Require().NoError(err)
	ts.Equal("COMPLETE", completed.FlowStatus)
	ts.Require().NotEmpty(completed.Assertion)

	assertionClaims, err := testutils.DecodeJWT(completed.Assertion)
	ts.Require().NoError(err)
	authRequestID, ok := assertionClaims.Additional["authorization_request_id"].(string)
	ts.Require().True(ok)
	ts.Require().NotEmpty(authRequestID)

	callbackStatus, callbackError := ts.postCallback(authRequestID, completed.Assertion)
	ts.Equal(http.StatusOK, callbackStatus, callbackError)

	tokens := ts.pollToken(authResponse.DeviceCode)
	ts.Equal(http.StatusOK, tokens.statusCode, tokens.errorCode)
	ts.Require().NotEmpty(tokens.accessToken)
	ts.Require().NotEmpty(tokens.idToken)

	accessTokenClaims, err := testutils.DecodeJWT(tokens.accessToken)
	ts.Require().NoError(err)
	ts.Equal(ts.userID, accessTokenClaims.Sub)

	reuse := ts.pollToken(authResponse.DeviceCode)
	ts.Equal(http.StatusBadRequest, reuse.statusCode)
	ts.Equal("invalid_grant", reuse.errorCode)
}

func (ts *DeviceGrantTestSuite) deviceAuthorize(scope string) (int, deviceAuthorizationResponse, http.Header) {
	form := url.Values{"scope": []string{scope}}
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+deviceAuthorizeEndpoint,
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(deviceGrantClientID, deviceGrantClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var body deviceAuthorizationResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, body, resp.Header.Clone()
}

func (ts *DeviceGrantTestSuite) startDeviceGrantFlow() *testutils.FlowStep {
	payload, err := json.Marshal(map[string]interface{}{
		"applicationId": ts.appID,
		"flowType":      "DEVICE_GRANT",
		"verbose":       true,
	})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/flow/execute", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var step testutils.FlowStep
	ts.Require().NoError(json.Unmarshal(body, &step))
	return &step
}

func (ts *DeviceGrantTestSuite) postCallback(authID, assertion string) (int, string) {
	payload, err := json.Marshal(map[string]string{
		"authId":    authID,
		"assertion": assertion,
		"type":      deviceGrantType,
	})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+deviceCallbackEndpoint,
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	var body map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	errorCode, _ := body["error"].(string)
	return resp.StatusCode, errorCode
}

func (ts *DeviceGrantTestSuite) pollToken(deviceCode string) deviceTokenResult {
	form := url.Values{
		"grant_type":  []string{deviceGrantType},
		"device_code": []string{deviceCode},
	}
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+deviceTokenEndpoint,
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(deviceGrantClientID, deviceGrantClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	var body map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)

	result := deviceTokenResult{statusCode: resp.StatusCode}
	result.accessToken, _ = body["access_token"].(string)
	result.idToken, _ = body["id_token"].(string)
	result.errorCode, _ = body["error"].(string)
	return result
}

func deviceGrantAuthenticationFlow(ouID string) testutils.Flow {
	inputs := []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "Device Grant Integration Authentication Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "device_grant_integration_auth_flow",
		OUID:     ouID,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": inputs,
						"action": map[string]interface{}{
							"ref": "action_credentials", "nextNode": "credentials_auth",
						},
					},
				},
			},
			{
				"id": "credentials_auth", "type": "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": inputs},
				"onSuccess": "auth_assert", "onIncomplete": "prompt_credentials",
			},
			{
				"id": "auth_assert", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

func deviceGrantApprovalFlow(ouID, authFlowID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Device Grant Integration Approval Flow",
		FlowType: "DEVICE_GRANT",
		Handle:   "device_grant_integration_approval_flow",
		OUID:     ouID,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "verify_user_code"},
			{
				"id": "verify_user_code", "type": "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "DeviceCodeExecutor"},
				"onSuccess": "call_authentication", "onIncomplete": "prompt_user_code",
			},
			{
				"id":   "prompt_user_code",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref": "input_user_code", "identifier": "user_code",
								"type": "TEXT_INPUT", "required": true,
							},
						},
						"action": map[string]interface{}{
							"ref": "device-code-submit", "nextNode": "verify_user_code",
						},
					},
				},
			},
			{
				"id": "call_authentication", "type": "CALL",
				"flow":      map[string]interface{}{"ref": authFlowID},
				"onSuccess": "end", "onFailure": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}
