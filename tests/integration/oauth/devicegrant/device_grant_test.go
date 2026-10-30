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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"gopkg.in/yaml.v3"
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
	deviceGrantSecondClientID = "device_grant_integration_second_client"
	deviceGrantSecondSecret   = "device_grant_integration_second_secret"
	deviceGrantNoFlowClientID = "device_grant_integration_no_flow_client"
	deviceGrantNoFlowSecret   = "device_grant_integration_no_flow_secret"
	deviceGrantDenyClientID   = "device_grant_integration_deny_client"
	deviceGrantDenySecret     = "device_grant_integration_deny_secret"
)

type deviceAuthorizationResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
	Error                   string `json:"error"`
	ErrorDescription        string `json:"error_description"`
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
	secondAppID       string
	noFlowAppID       string
	denyAuthFlowID    string
	denyFlowID        string
	denyAppID         string
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

	deviceFlowID, err := testutils.CreateFlow(deviceGrantApprovalFlow(ts.ouID, authFlowID,
		"Device Grant Integration Approval Flow", "device_grant_integration_approval_flow", true))
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

	secondAppID, err := testutils.CreateApplication(deviceGrantApplication(
		ts.ouID, "Device Grant Second Integration Application", deviceGrantSecondClientID,
		deviceGrantSecondSecret, ts.authFlowID, ts.deviceGrantFlowID,
		[]string{deviceGrantType},
	))
	ts.Require().NoError(err)
	ts.secondAppID = secondAppID

	noFlowAppID, err := testutils.CreateApplication(deviceGrantApplication(
		ts.ouID, "Device Grant No Flow Integration Application", deviceGrantNoFlowClientID,
		deviceGrantNoFlowSecret, ts.authFlowID, "", []string{"client_credentials"},
	))
	ts.Require().NoError(err)
	ts.noFlowAppID = noFlowAppID

	ts.denyAuthFlowID, err = testutils.CreateFlow(deviceGrantDenialAuthenticationFlow(ts.ouID))
	ts.Require().NoError(err)
	ts.denyFlowID, err = testutils.CreateFlow(deviceGrantApprovalFlow(ts.ouID, ts.denyAuthFlowID,
		"Device Grant Denial Integration Flow", "device_grant_denial_integration_flow", false))
	ts.Require().NoError(err)
	ts.denyAppID, err = testutils.CreateApplication(deviceGrantApplication(
		ts.ouID, "Device Grant Denial Integration Application", deviceGrantDenyClientID,
		deviceGrantDenySecret, ts.denyAuthFlowID, ts.denyFlowID, []string{deviceGrantType},
	))
	ts.Require().NoError(err)
}

func (ts *DeviceGrantTestSuite) TearDownSuite() {
	if ts.denyAppID != "" {
		_ = testutils.DeleteApplication(ts.denyAppID)
	}
	if ts.denyFlowID != "" {
		_ = testutils.DeleteFlow(ts.denyFlowID)
	}
	if ts.denyAuthFlowID != "" {
		_ = testutils.DeleteFlow(ts.denyAuthFlowID)
	}
	if ts.noFlowAppID != "" {
		_ = testutils.DeleteApplication(ts.noFlowAppID)
	}
	if ts.secondAppID != "" {
		_ = testutils.DeleteApplication(ts.secondAppID)
	}
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

func deviceGrantApplication(ouID, name, clientID, clientSecret, authFlowID, deviceFlowID string,
	grantTypes []string) testutils.Application {
	return testutils.Application{
		OUID:              ouID,
		Name:              name,
		Description:       name,
		Type:              "fullstack",
		AuthFlowID:        authFlowID,
		DeviceGrantFlowID: deviceFlowID,
		AllowedUserTypes:  []string{deviceGrantUserTypeHandle},
		AssertionConfig:   map[string]interface{}{"userAttributes": []string{"email"}},
		LoginConsent:      map[string]interface{}{"validityPeriod": 3600},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId": clientID, "clientSecret": clientSecret,
				"redirectUris": []string{"https://localhost:3000/callback"},
				"grantTypes":   grantTypes, "tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	}
}

// TestDeviceGrantLifecycle exercises the complete browser and device sides of RFC 8628. It verifies
// request creation, pending polling, rate limiting, user-code verification, the application-assigned
// Authentication flow, callback binding, token issuance, and one-time request consumption.
func (ts *DeviceGrantTestSuite) TestDeviceGrantLifecycle() {
	authStatus, authResponse, headers := ts.deviceAuthorize("openid")
	ts.Require().Equal(http.StatusOK, authStatus, "device authorization error: %s: %s",
		authResponse.Error, authResponse.ErrorDescription)
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

func (ts *DeviceGrantTestSuite) TestDeviceGrantDeniedByUser() {
	status, authorization, _ := ts.deviceAuthorizeAs("openid email", deviceGrantDenyClientID, deviceGrantDenySecret)
	ts.Require().Equal(http.StatusOK, status)

	codePrompt := ts.startDeviceGrantFlowFor(ts.denyAppID)
	authPrompt, err := testutils.ExecuteAuthenticationFlow(codePrompt.ExecutionID,
		map[string]string{"user_code": authorization.UserCode}, "device-code-submit", codePrompt.ChallengeToken)
	ts.Require().NoError(err)
	consentPrompt, err := testutils.ExecuteAuthenticationFlow(authPrompt.ExecutionID, map[string]string{
		"username": deviceGrantUsername, "password": deviceGrantPassword,
	}, "action_credentials", authPrompt.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", consentPrompt.FlowStatus)
	ts.Require().NotNil(consentPrompt.Data)
	rawPrompt, ok := consentPrompt.Data.AdditionalData["consentPrompt"].(string)
	ts.Require().True(ok, "expected the Authentication flow to request consent")
	var promptPurposes []struct {
		PurposeName string `json:"purposeName"`
		Essential   []struct {
			Name string `json:"name"`
		} `json:"essential"`
		Optional []struct {
			Name string `json:"name"`
		} `json:"optional"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(rawPrompt), &promptPurposes))
	type elementDecision struct {
		Name     string `json:"name"`
		Approved bool   `json:"approved"`
	}
	type purposeDecision struct {
		PurposeName string            `json:"purposeName"`
		Approved    bool              `json:"approved"`
		Elements    []elementDecision `json:"elements"`
	}
	decisions := struct {
		Approved bool              `json:"approved"`
		Purposes []purposeDecision `json:"purposes"`
	}{Approved: false}
	for _, purpose := range promptPurposes {
		decision := purposeDecision{PurposeName: purpose.PurposeName, Approved: false}
		for _, element := range append(purpose.Essential, purpose.Optional...) {
			decision.Elements = append(decision.Elements, elementDecision{Name: element.Name, Approved: false})
		}
		decisions.Purposes = append(decisions.Purposes, decision)
	}
	decisionsJSON, err := json.Marshal(decisions)
	ts.Require().NoError(err)

	denied, err := testutils.ExecuteAuthenticationFlow(consentPrompt.ExecutionID,
		map[string]string{"consent_decisions": string(decisionsJSON)},
		"consent_action_deny", consentPrompt.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("ERROR", denied.FlowStatus)
	ts.Require().NotEmpty(denied.ErrorAssertion)
	claims, err := testutils.DecodeJWT(denied.ErrorAssertion)
	ts.Require().NoError(err)
	authID, ok := claims.Additional["authorization_request_id"].(string)
	ts.Require().True(ok)

	callbackStatus, callbackError := ts.postCallback(authID, denied.ErrorAssertion)
	ts.Require().Equal(http.StatusOK, callbackStatus, callbackError)
	result := ts.pollTokenAs(authorization.DeviceCode, deviceGrantDenyClientID, deviceGrantDenySecret)
	ts.Equal(http.StatusBadRequest, result.statusCode)
	ts.Equal("access_denied", result.errorCode)
	ts.Empty(result.accessToken)
}

func (ts *DeviceGrantTestSuite) TestDeviceGrantExpiredRequest() {
	configPath := filepath.Join(testutils.GetExtractedProductHome(), "deployment.yaml")
	originalConfig, err := os.ReadFile(configPath)
	ts.Require().NoError(err)
	defer func() {
		if writeErr := os.WriteFile(configPath, originalConfig, 0o644); writeErr != nil {
			ts.T().Errorf("restore deployment config: %v", writeErr)
			return
		}
		if restartErr := testutils.RestartServerWithTimeout(60 * time.Second); restartErr != nil {
			ts.T().Errorf("restart server after restoring deployment config: %v", restartErr)
			return
		}
		if tokenErr := testutils.ObtainAdminAccessToken(); tokenErr != nil {
			ts.T().Errorf("refresh admin token after restoring deployment config: %v", tokenErr)
		}
	}()

	var config map[string]interface{}
	ts.Require().NoError(yaml.Unmarshal(originalConfig, &config))
	oauthConfig, ok := config["oauth"].(map[string]interface{})
	ts.Require().True(ok, "deployment config must contain OAuth settings")
	deviceConfig, ok := oauthConfig["device_grant"].(map[string]interface{})
	if !ok {
		deviceConfig = make(map[string]interface{})
		oauthConfig["device_grant"] = deviceConfig
	}
	deviceConfig["expires_in"] = 1
	shortConfig, err := yaml.Marshal(config)
	ts.Require().NoError(err)
	ts.Require().NoError(os.WriteFile(configPath, shortConfig, 0o644))
	ts.Require().NoError(testutils.RestartServerWithTimeout(60 * time.Second))

	status, authorization, _ := ts.deviceAuthorize("openid")
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal(int64(1), authorization.ExpiresIn)
	time.Sleep(2 * time.Second)

	expired := ts.pollToken(authorization.DeviceCode)
	ts.Equal(http.StatusBadRequest, expired.statusCode)
	ts.Equal("expired_token", expired.errorCode)
}

func (ts *DeviceGrantTestSuite) TestDeviceGrantRejectsInvalidUserCode() {
	codePrompt := ts.startDeviceGrantFlow()
	step, err := testutils.ExecuteAuthenticationFlow(codePrompt.ExecutionID,
		map[string]string{"user_code": "ZZZZ-ZZZZ"}, "device-code-submit", codePrompt.ChallengeToken)
	ts.Require().NoError(err)
	ts.Equal("INCOMPLETE", step.FlowStatus)
	ts.Require().NotNil(step.Error)
	ts.Equal("FET-1131", step.Error.Code)
}

func (ts *DeviceGrantTestSuite) TestDeviceGrantCodeCannotBeUsedForAnotherApplication() {
	status, authorization, _ := ts.deviceAuthorize("openid")
	ts.Require().Equal(http.StatusOK, status)

	otherAppPrompt := ts.startDeviceGrantFlowFor(ts.secondAppID)
	wrongAppStep, err := testutils.ExecuteAuthenticationFlow(otherAppPrompt.ExecutionID,
		map[string]string{"user_code": authorization.UserCode}, "device-code-submit", otherAppPrompt.ChallengeToken)
	ts.Require().NoError(err)
	ts.Equal("INCOMPLETE", wrongAppStep.FlowStatus)
	ts.Require().NotNil(wrongAppStep.Error)
	ts.Equal("FET-1131", wrongAppStep.Error.Code)

	correctAppPrompt := ts.startDeviceGrantFlow()
	correctAppStep, err := testutils.ExecuteAuthenticationFlow(correctAppPrompt.ExecutionID,
		map[string]string{"user_code": authorization.UserCode}, "device-code-submit", correctAppPrompt.ChallengeToken)
	ts.Require().NoError(err)
	ts.Equal("INCOMPLETE", correctAppStep.FlowStatus,
		"the cross-application attempt must not claim the request")
	ts.Empty(correctAppStep.Error)
}

func (ts *DeviceGrantTestSuite) TestDeviceGrantCodeCannotBePolledByAnotherClient() {
	status, authorization, _ := ts.deviceAuthorize("openid")
	ts.Require().Equal(http.StatusOK, status)

	result := ts.pollTokenAs(authorization.DeviceCode, deviceGrantSecondClientID, deviceGrantSecondSecret)
	ts.Equal(http.StatusBadRequest, result.statusCode)
	ts.Equal("invalid_grant", result.errorCode)
}

func (ts *DeviceGrantTestSuite) TestDeviceGrantFlowMustBeConfigured() {
	status, _, body := ts.startDeviceGrantFlowResult(ts.noFlowAppID)
	ts.Equal(http.StatusBadRequest, status)
	ts.Require().NotNil(body)
	ts.Equal("FES-1024", body.Code)
}

func (ts *DeviceGrantTestSuite) deviceAuthorize(scope string) (int, deviceAuthorizationResponse, http.Header) {
	return ts.deviceAuthorizeAs(scope, deviceGrantClientID, deviceGrantClientSecret)
}

func (ts *DeviceGrantTestSuite) deviceAuthorizeAs(
	scope, clientID, clientSecret string,
) (int, deviceAuthorizationResponse, http.Header) {
	form := url.Values{"scope": []string{scope}}
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+deviceAuthorizeEndpoint,
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var body deviceAuthorizationResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, body, resp.Header.Clone()
}

func (ts *DeviceGrantTestSuite) startDeviceGrantFlow() *testutils.FlowStep {
	status, step, body := ts.startDeviceGrantFlowResult(ts.appID)
	ts.Require().Equal(http.StatusOK, status, "%+v", body)
	ts.Require().NotNil(step)
	return step
}

func (ts *DeviceGrantTestSuite) startDeviceGrantFlowFor(applicationID string) *testutils.FlowStep {
	status, step, body := ts.startDeviceGrantFlowResult(applicationID)
	ts.Require().Equal(http.StatusOK, status, "%+v", body)
	ts.Require().NotNil(step)
	return step
}

func (ts *DeviceGrantTestSuite) startDeviceGrantFlowResult(
	applicationID string,
) (int, *testutils.FlowStep, *testutils.FlowErrorResponse) {
	payload, err := json.Marshal(map[string]interface{}{
		"applicationId": applicationID,
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
	var step testutils.FlowStep
	if resp.StatusCode == http.StatusOK {
		ts.Require().NoError(json.Unmarshal(body, &step))
		return resp.StatusCode, &step, nil
	}
	var errorBody testutils.FlowErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errorBody), string(body))
	return resp.StatusCode, nil, &errorBody
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
	return ts.pollTokenAs(deviceCode, deviceGrantClientID, deviceGrantClientSecret)
}

func (ts *DeviceGrantTestSuite) pollTokenAs(deviceCode, clientID, clientSecret string) deviceTokenResult {
	form := url.Values{
		"grant_type":  []string{deviceGrantType},
		"device_code": []string{deviceCode},
	}
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+deviceTokenEndpoint,
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

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

func deviceGrantApprovalFlow(ouID, authFlowID, name, handle string,
	handleAuthenticationFailure bool) testutils.Flow {
	callAuthentication := map[string]interface{}{
		"id": "call_authentication", "type": "CALL",
		"flow": map[string]interface{}{"ref": authFlowID}, "onSuccess": "end",
	}
	nodes := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "verify_user_code"},
		{
			"id": "verify_user_code", "type": "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "DeviceCodeExecutor"},
			"onSuccess": "call_authentication", "onIncomplete": "prompt_user_code",
		},
		{
			"id":   "prompt_user_code",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{{
					"ref": "input_user_code", "identifier": "user_code",
					"type": "TEXT_INPUT", "required": true,
				}},
				"action": map[string]interface{}{
					"ref": "device-code-submit", "nextNode": "verify_user_code",
				},
			}},
		},
	}
	if handleAuthenticationFailure {
		callAuthentication["onFailure"] = "authorization_failed"
	}
	nodes = append(nodes, callAuthentication)
	if handleAuthenticationFailure {
		nodes = append(nodes, map[string]interface{}{
			"id": "authorization_failed", "type": "PROMPT", "message": "Device is not authorized",
			"next": "end", "meta": map[string]interface{}{"components": []map[string]interface{}{{
				"type": "TEXT", "id": "authorization-failed-title", "label": "Device is not authorized",
			}}},
		})
	}
	nodes = append(nodes, map[string]interface{}{"id": "end", "type": "END"})

	return testutils.Flow{
		Name:     name,
		FlowType: "DEVICE_GRANT",
		Handle:   handle,
		OUID:     ouID,
		Nodes:    nodes,
	}
}

func deviceGrantDenialAuthenticationFlow(ouID string) testutils.Flow {
	inputs := []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	consentInput := map[string]interface{}{
		"ref": "input_consent", "identifier": "consent_decisions", "type": "CONSENT_INPUT", "required": true,
	}
	return testutils.Flow{
		Name:     "Device Grant Denial Integration Authentication Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "device_grant_denial_integration_auth_flow",
		OUID:     ouID,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{"id": "prompt_credentials", "type": "PROMPT", "prompts": []map[string]interface{}{{
				"inputs": inputs,
				"action": map[string]interface{}{"ref": "action_credentials", "nextNode": "credentials_auth"},
			}}},
			{"id": "credentials_auth", "type": "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": inputs},
				"onSuccess": "authorization_check", "onIncomplete": "prompt_credentials"},
			{"id": "authorization_check", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "AuthorizationExecutor"}, "onSuccess": "consent_check"},
			{"id": "consent_check", "type": "TASK_EXECUTION", "executor": map[string]interface{}{
				"name": "ConsentExecutor", "inputs": []map[string]interface{}{consentInput},
			}, "properties": map[string]interface{}{"failOnDeny": true},
				"onSuccess": "auth_assert", "onIncomplete": "prompt_consent"},
			{"id": "prompt_consent", "type": "PROMPT", "prompts": []map[string]interface{}{
				{"inputs": []map[string]interface{}{consentInput},
					"action": map[string]interface{}{"ref": "consent_action_allow", "nextNode": "consent_check"}},
				{"inputs": []map[string]interface{}{consentInput},
					"action": map[string]interface{}{"ref": "consent_action_deny", "nextNode": "consent_check"}},
			}},
			{"id": "auth_assert", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
			{"id": "end", "type": "END"},
		},
	}
}
