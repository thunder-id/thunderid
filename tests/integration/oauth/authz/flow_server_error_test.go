// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	serverErrClientID     = "authz_server_err_client_123"
	serverErrClientSecret = "authz_server_err_secret_123"
	serverErrRedirect     = "https://localhost:3000"
	// A port with nothing listening, so the notification send fails with a connection error. This
	// stands in for any infrastructure dependency being unreachable mid-flow.
	serverErrDeadSenderURL = "http://127.0.0.1:65533/send"
)

// ServerErrorTestSuite covers the engine-failure channel: an infrastructure dependency that goes down
// mid-authentication surfaces as a server-side error rather than an in-band flowStatus=ERROR, so the
// error assertion travels in the 4xx/5xx body instead of the flow response. It must still reach the
// waiting OAuth request, as server_error rather than access_denied.
//
// The dependency broken here is the notification gateway, because SMSExecutor returns a bare Go error
// on any send failure in an AUTHENTICATION flow (sms_executor.go), which the task node converts to
// InternalServerError. That is the same path a userdb outage takes out of CredentialsAuthExecutor.
type ServerErrorTestSuite struct {
	suite.Suite
	ouID     string
	senderID string
	flowID   string
	appID    string
	client   *http.Client
}

func TestServerErrorTestSuite(t *testing.T) {
	suite.Run(t, new(ServerErrorTestSuite))
}

func (ts *ServerErrorTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "authz-server-err-ou",
		Name:        "Authz Server Error OU",
		Description: "Organization unit for the engine-failure error assertion test",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	senderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:        "Unreachable Test Sender",
		Description: "Sender pointed at a dead port to force a server-side send failure",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: serverErrDeadSenderURL},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	ts.Require().NoError(err, "Failed to create notification sender")
	ts.senderID = senderID

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Authz Server Error Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_authz_server_error",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_mobile"},
			{
				"id":   "prompt_mobile",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_001",
								"identifier": "mobile_number",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "send_sms",
						},
					},
				},
			},
			{
				"id":   "send_sms",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"senderId":    senderID,
					"smsTemplate": "ciba-notification",
				},
				"executor":  map[string]interface{}{"name": "SMSExecutor"},
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
	})
	ts.Require().NoError(err, "Failed to create server-error flow")
	ts.flowID = flowID

	ts.appID = ts.createServerErrorApplication(flowID)
}

func (ts *ServerErrorTestSuite) TearDownSuite() {
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	if ts.flowID != "" {
		_ = testutils.DeleteFlow(ts.flowID)
	}
	if ts.senderID != "" {
		_ = testutils.DeleteNotificationSender(ts.senderID)
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// TestServerErrorPropagatesToClientRedirect verifies that a server-side failure during authentication
// still reaches the client, as server_error. The assertion arrives in the 4xx/5xx error body rather
// than a flow response, which is a separate serialization path from the in-band ERROR channel.
func (ts *ServerErrorTestSuite) TestServerErrorPropagatesToClientRedirect() {
	resp, err := testutils.InitiateAuthorizationFlow(serverErrClientID, serverErrRedirect,
		"code", "openid", "server_err_state")
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode, "Expected redirect status")

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "Failed to extract auth data from redirect")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	// Submitting the recipient advances into the SMS node, whose gateway is unreachable.
	status, errBody, err := testutils.ExecuteAuthenticationFlowExpectingError(executionID,
		map[string]string{"mobile_number": "+1987654321"}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to execute the flow")
	ts.Require().GreaterOrEqual(status, http.StatusInternalServerError,
		"An infrastructure failure should be reported as a 5xx, not an in-band flow response")
	ts.Require().NotNil(errBody)
	ts.Require().NotEmpty(errBody.ErrorAssertion,
		"The server-error body must carry the signed error assertion")

	authzResponse, err := testutils.CompleteAuthorization(authID, errBody.ErrorAssertion)
	ts.Require().NoError(err, "Callback should return a redirect (200), not an HTTP error")

	parsed, err := url.Parse(authzResponse.RedirectURI)
	ts.Require().NoError(err, "Failed to parse client redirect URI")
	ts.Equal("server_error", parsed.Query().Get("error"),
		"A server-side failure must reach the client as server_error, not access_denied")
	ts.Equal("server_err_state", parsed.Query().Get("state"),
		"The client's state must be echoed back on the error redirect")
	ts.NotEmpty(parsed.Query().Get("iss"), "The error redirect must carry the issuer")
	ts.Empty(parsed.Query().Get("code"), "No authorization code may be issued for a failed flow")
}

// createServerErrorApplication creates an OAuth application bound to the given authentication flow.
func (ts *ServerErrorTestSuite) createServerErrorApplication(authFlowID string) string {
	app := map[string]interface{}{
		"name":                      "AuthzServerErrorApp",
		"description":               "Application for the server-error assertion test",
		"ouId":                      ts.ouID,
		"type":                      "browser",
		"authFlowId":                authFlowID,
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                serverErrClientID,
					"clientSecret":            serverErrClientSecret,
					"redirectUris":            []string{serverErrRedirect},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
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

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Failed to create application. Status: %d, Response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respData))
	return respData["id"].(string)
}
