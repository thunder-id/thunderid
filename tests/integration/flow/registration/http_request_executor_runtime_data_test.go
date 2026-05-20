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

package registration

import (
	"fmt"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	mockGoogleRuntimeDataPort       = 8096
	mockNotificationRuntimeDataPort = 8097
	mockHTTPRuntimeDataPort         = 9092
)

var (
	httpRequestRuntimeDataFlow = testutils.Flow{
		Name:     "HTTP Request Runtime Data Registration Flow",
		FlowType: "REGISTRATION",
		Handle:   "registration_flow_http_runtime_data",
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
				"onSuccess":    "google_auth",
				"onIncomplete": "prompt_usertype",
			},
			{
				"id":   "prompt_usertype",
				"type": "PROMPT",
				"meta": map[string]interface{}{
					"components": []map[string]interface{}{
						{
							"type":    "TEXT",
							"id":      "heading_usertype",
							"label":   "Sign Up",
							"variant": "HEADING_2",
						},
						{
							"type": "BLOCK",
							"id":   "block_usertype",
							"components": []map[string]interface{}{
								{
									"type":        "SELECT",
									"id":          "usertype_input",
									"ref":         "userType",
									"label":       "User Type",
									"placeholder": "Select your user type",
									"required":    true,
									"options":     []interface{}{},
								},
								{
									"type":      "ACTION",
									"id":        "action_usertype",
									"label":     "Continue",
									"variant":   "PRIMARY",
									"eventType": "SUBMIT",
								},
							},
						},
					},
				},
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
				"id":   "google_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId": "placeholder-idp-id",
				},
				"executor": map[string]interface{}{
					"name": "GoogleOIDCAuthExecutor",
				},
				"onSuccess": "prompt_mobile",
			},
			{
				"id":   "prompt_mobile",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_mobile",
								"identifier": "mobileNumber",
								"type":       "string",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_mobile",
							"nextNode": "sms_otp_send",
						},
					},
				},
			},
			{
				"id":   "sms_otp_send",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"senderId": "placeholder-sender-id",
				},
				"executor": map[string]interface{}{
					"name": "SMSOTPAuthExecutor",
					"mode": "send",
				},
				"onSuccess": "prompt_otp",
			},
			{
				"id":   "prompt_otp",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_otp",
								"identifier": "otp",
								"type":       "string",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_otp",
							"nextNode": "sms_otp_verify",
						},
					},
				},
			},
			{
				"id":   "sms_otp_verify",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"senderId": "placeholder-sender-id",
				},
				"executor": map[string]interface{}{
					"name": "SMSOTPAuthExecutor",
					"mode": "verify",
				},
				"onSuccess": "http_request",
			},
			{
				"id":   "http_request",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"url":    "http://localhost:9092/api/notifications",
					"method": "POST",
					"headers": map[string]interface{}{
						"X-App-Id":    "{{ context.applicationId }}",
						"X-IDP-Id":    "{{ context.idpId }}",
						"X-Sender-Id": "{{ context.senderId }}",
					},
					"body": map[string]interface{}{
						"application": "{{ context.applicationId }}",
						"idp":         "{{ context.idpId }}",
						"sender":      "{{ context.senderId }}",
					},
				},
				"executor": map[string]interface{}{
					"name": "HTTPRequestExecutor",
				},
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
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

	httpRequestRuntimeDataOU = testutils.OrganizationUnit{
		Name:        "HTTP Request Runtime Data Registration Test OU",
		Handle:      "http-request-runtime-data-reg-ou",
		Description: "Organization unit for HTTP request runtime data registration flow",
	}

	httpRequestRuntimeDataEntityType = testutils.UserType{
		Name: "http_request_runtime_user",
		Schema: map[string]interface{}{
			"sub": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"email_verified": map[string]interface{}{
				"type": "string",
			},
			"name": map[string]interface{}{
				"type": "string",
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
			"family_name": map[string]interface{}{
				"type": "string",
			},
			"givenName": map[string]interface{}{
				"type": "string",
			},
			"familyName": map[string]interface{}{
				"type": "string",
			},
			"picture": map[string]interface{}{
				"type": "string",
			},
			"locale": map[string]interface{}{
				"type": "string",
			},
			"mobileNumber": map[string]interface{}{
				"type": "string",
			},
		},
	}

	httpRequestRuntimeDataApp = testutils.Application{
		Name:                      "HTTP Request Runtime Data Registration Test Application",
		Description:               "App to verify runtime data placeholders in HTTP request executor",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "http_runtime_data_reg_client",
		ClientSecret:              "http_runtime_data_reg_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{httpRequestRuntimeDataEntityType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}
)

var (
	httpRequestRuntimeDataAppID string
	httpRequestRuntimeDataOUID  string
	httpRequestRuntimeUserID    string
)

type HTTPRequestRuntimeDataRegistrationFlowTestSuite struct {
	suite.Suite
	config                 *common.TestSuiteConfig
	mockGoogleServer       *testutils.MockGoogleOIDCServer
	mockNotificationServer *testutils.MockNotificationServer
	mockHTTPServer         *testutils.MockHTTPServer
	idpID                  string
	senderID               string
	entityTypeID           string
}

func TestHTTPRequestRuntimeDataRegistrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPRequestRuntimeDataRegistrationFlowTestSuite))
}

func (ts *HTTPRequestRuntimeDataRegistrationFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ts.mockHTTPServer = testutils.NewMockHTTPServer(mockHTTPRuntimeDataPort)
	err := ts.mockHTTPServer.Start()
	ts.Require().NoError(err, "Failed to start mock HTTP server")
	time.Sleep(100 * time.Millisecond)

	ts.mockNotificationServer = testutils.NewMockNotificationServer(mockNotificationRuntimeDataPort)
	err = ts.mockNotificationServer.Start()
	ts.Require().NoError(err, "Failed to start mock notification server")
	time.Sleep(100 * time.Millisecond)

	mockGoogleServer, err := testutils.NewMockGoogleOIDCServer(mockGoogleRuntimeDataPort,
		"runtime_data_client", "runtime_data_secret")
	ts.Require().NoError(err, "Failed to create mock Google server")
	ts.mockGoogleServer = mockGoogleServer

	ts.mockGoogleServer.AddUser(&testutils.GoogleUserInfo{
		Sub:           "runtime-data-google-user-123",
		Email:         "runtime-data-user@example.com",
		EmailVerified: true,
		Name:          "Runtime Data User",
		GivenName:     "Runtime",
		FamilyName:    "User",
		Picture:       "https://example.com/runtime-data-picture.jpg",
		Locale:        "en",
	})

	err = ts.mockGoogleServer.Start()
	ts.Require().NoError(err, "Failed to start mock Google server")

	ouID, err := testutils.CreateOrganizationUnit(httpRequestRuntimeDataOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	httpRequestRuntimeDataOUID = ouID

	httpRequestRuntimeDataEntityType.OUID = httpRequestRuntimeDataOUID
	httpRequestRuntimeDataEntityType.AllowSelfRegistration = true
	schemaID, err := testutils.CreateUserType(httpRequestRuntimeDataEntityType)
	ts.Require().NoError(err, "Failed to create user type for runtime data flow")
	ts.entityTypeID = schemaID

	idp := testutils.IDP{
		Name:        "HTTP Request Runtime Data Google IDP",
		Description: "Google IDP for runtime data registration flow",
		Type:        "GOOGLE",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "runtime_data_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "runtime_data_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "http://localhost:3000/callback",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "openid email profile",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/o/oauth2/v2/auth",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/v1/userinfo",
				IsSecret: false,
			},
			{
				Name:     "jwks_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/oauth2/v3/certs",
				IsSecret: false,
			},
		},
	}

	idpID, err := testutils.CreateIDP(idp)
	ts.Require().NoError(err, "Failed to create Google IDP")
	ts.idpID = idpID
	ts.config.CreatedIdpIDs = append(ts.config.CreatedIdpIDs, idpID)

	notificationSender := testutils.NotificationSender{
		Name:        "Runtime Data SMS Sender",
		Description: "Sender for runtime data registration flow",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{
				Name:     "url",
				Value:    ts.mockNotificationServer.GetSendSMSURL(),
				IsSecret: false,
			},
			{
				Name:     "http_method",
				Value:    "POST",
				IsSecret: false,
			},
			{
				Name:     "content_type",
				Value:    "JSON",
				IsSecret: false,
			},
		},
	}

	senderID, err := testutils.CreateNotificationSender(notificationSender)
	ts.Require().NoError(err, "Failed to create notification sender")
	ts.senderID = senderID
	ts.config.CreatedSenderIDs = append(ts.config.CreatedSenderIDs, senderID)

	nodes := httpRequestRuntimeDataFlow.Nodes.([]map[string]interface{})
	nodes[3]["properties"].(map[string]interface{})["idpId"] = idpID
	nodes[5]["properties"].(map[string]interface{})["senderId"] = senderID
	nodes[7]["properties"].(map[string]interface{})["senderId"] = senderID
	nodes[8]["properties"].(map[string]interface{})["url"] = fmt.Sprintf("http://localhost:%d/api/notifications",
		mockHTTPRuntimeDataPort)
	httpRequestRuntimeDataFlow.Nodes = nodes

	flowID, err := testutils.CreateFlow(httpRequestRuntimeDataFlow)
	ts.Require().NoError(err, "Failed to create runtime data registration flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	httpRequestRuntimeDataApp.RegistrationFlowID = flowID

	httpRequestRuntimeDataApp.OUID = httpRequestRuntimeDataOUID
	appID, err := testutils.CreateApplication(httpRequestRuntimeDataApp)
	ts.Require().NoError(err, "Failed to create runtime data test application")
	httpRequestRuntimeDataAppID = appID
}

func (ts *HTTPRequestRuntimeDataRegistrationFlowTestSuite) TearDownSuite() {
	if ts.mockHTTPServer != nil {
		_ = ts.mockHTTPServer.Stop()
	}

	if ts.mockNotificationServer != nil {
		_ = ts.mockNotificationServer.Stop()
	}

	if ts.mockGoogleServer != nil {
		_ = ts.mockGoogleServer.Stop()
	}

	if len(ts.config.CreatedUserIDs) > 0 {
		if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
			ts.T().Logf("Failed to cleanup users during teardown: %v", err)
		}
	}

	if httpRequestRuntimeDataAppID != "" {
		if err := testutils.DeleteApplication(httpRequestRuntimeDataAppID); err != nil {
			ts.T().Logf("Failed to delete application during teardown: %v", err)
		}
	}

	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow during teardown: %v", err)
		}
	}

	for _, idpID := range ts.config.CreatedIdpIDs {
		if err := testutils.DeleteIDP(idpID); err != nil {
			ts.T().Logf("Failed to delete IDP during teardown: %v", err)
		}
	}

	for _, senderID := range ts.config.CreatedSenderIDs {
		if err := testutils.DeleteNotificationSender(senderID); err != nil {
			ts.T().Logf("Failed to delete notification sender during teardown: %v", err)
		}
	}

	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}

	if httpRequestRuntimeDataOUID != "" {
		if err := testutils.DeleteOrganizationUnit(httpRequestRuntimeDataOUID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
}

func (ts *HTTPRequestRuntimeDataRegistrationFlowTestSuite) SetupTest() {
	if ts.mockHTTPServer != nil {
		ts.mockHTTPServer.ClearRequests()
	}
	if ts.mockNotificationServer != nil {
		ts.mockNotificationServer.ClearMessages()
	}
}

func (ts *HTTPRequestRuntimeDataRegistrationFlowTestSuite) TestHTTPRequestRuntimeDataPropagation() {
	mobileNumber := generateUniqueMobileNumber()

	flowStep, err := common.InitiateRegistrationFlow(httpRequestRuntimeDataAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate runtime data registration flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().Equal("REDIRECTION", flowStep.Type)
	ts.Require().NotEmpty(flowStep.Data.RedirectURL)

	authCode, state, err := testutils.SimulateFederatedOAuthFlow(flowStep.Data.RedirectURL)
	ts.Require().NoError(err, "Failed to simulate Google authorization for runtime data flow")
	ts.Require().NotEmpty(authCode, "Authorization code should not be empty")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"code": authCode, "state": state,
	}, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete runtime data flow with authorization code")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().Equal("VIEW", flowStep.Type)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "mobileNumber"))

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"mobileNumber": mobileNumber,
	}, "action_mobile", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to continue runtime data flow with mobile number")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().Equal("VIEW", flowStep.Type)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "otp"))

	time.Sleep(500 * time.Millisecond)
	otpMessage := ts.mockNotificationServer.GetLastMessage()
	ts.Require().NotNil(otpMessage, "OTP message should be captured by mock notification server")
	ts.Require().NotEmpty(otpMessage.OTP, "OTP should be available for verification")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"otp": otpMessage.OTP,
	}, "action_otp", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete runtime data flow with OTP")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Flow should complete after HTTP request executor")
	ts.Require().NotEmpty(flowStep.Assertion, "Assertion should be present after flow completion")

	time.Sleep(200 * time.Millisecond)

	requests := ts.mockHTTPServer.GetCapturedRequests()
	ts.Require().NotEmpty(requests, "Mock HTTP server should capture at least one request")

	var notificationRequest *testutils.HTTPRequest
	for _, req := range requests {
		if req.Path == "/api/notifications" {
			notificationRequest = &req
			break
		}
	}

	ts.Require().NotNil(notificationRequest, "Notification request should be sent by HTTP request executor")
	ts.Require().Equal("POST", notificationRequest.Method)
	ts.Require().Equal(httpRequestRuntimeDataAppID, notificationRequest.Body["application"])
	ts.Require().Equal(ts.idpID, notificationRequest.Body["idp"])
	ts.Require().Equal(ts.senderID, notificationRequest.Body["sender"])
	ts.Require().Contains(notificationRequest.Headers["X-App-Id"], httpRequestRuntimeDataAppID)
	ts.Require().Contains(notificationRequest.Headers["X-Idp-Id"], ts.idpID)
	ts.Require().Contains(notificationRequest.Headers["X-Sender-Id"], ts.senderID)

	user, err := testutils.FindUserByAttribute("sub", "runtime-data-google-user-123")
	ts.Require().NoError(err, "User lookup should succeed after registration")
	ts.Require().NotNil(user, "User should be created after flow completion")
	if user != nil {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
		httpRequestRuntimeUserID = user.ID
	}
}
