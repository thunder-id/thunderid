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

package authentication

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var magicLinkAuthFlow = testutils.Flow{
	Name:     "Magic Link Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_magic_link_test",
	Nodes: []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "prompt_email",
		},
		{
			"id":   "prompt_email",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "email",
							"type":       "EMAIL_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "magic_link_action",
						"nextNode": "send_magic_link",
					},
				},
			},
		},
		{
			"id":   "send_magic_link",
			"type": "TASK_EXECUTION",
			"properties": map[string]interface{}{
				"magicLinkURL": "https://localhost:8095/gate/signin",
			},
			"executor": map[string]interface{}{
				"name": "MagicLinkAuthExecutor",
				"mode": "generate",
			},
			"onSuccess": "email_magic_link",
		},
		{
			"id":   "email_magic_link",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "EmailExecutor",
				"mode": "send",
				"inputs": []map[string]interface{}{
					{
						"ref":        "input_001",
						"identifier": "email",
						"type":       "EMAIL_INPUT",
						"required":   true,
					},
				},
			},
			"properties": map[string]interface{}{
				"emailTemplate": "MAGIC_LINK",
			},
			"onSuccess": "verify_magic_link",
		},
		{
			"id":   "verify_magic_link",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "MagicLinkAuthExecutor",
				"mode": "verify",
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

var magicLinkTestApp = testutils.Application{
	Name:                      "Magic Link Test Application",
	Description:               "Application for testing magic link authentication",
	IsRegistrationFlowEnabled: false,
	ClientID:                  "magic_link_test_client",
	ClientSecret:              "magic_link_test_secret",
	RedirectURIs:              []string{"http://localhost:3000/callback"},
	AllowedUserTypes:          []string{"magic_link_test_user"},
	AssertionConfig: map[string]interface{}{
		"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
	},
}

var magicLinkTestUserSchema = testutils.UserType{
	Name: "magic_link_test_user",
	Schema: map[string]interface{}{
		"email": map[string]interface{}{
			"type": "string",
		},
		"given_name": map[string]interface{}{
			"type": "string",
		},
		"family_name": map[string]interface{}{
			"type": "string",
		},
	},
}

var magicLinkTestUser = testutils.User{
	Type: magicLinkTestUserSchema.Name,
	Attributes: json.RawMessage(`{
		"email": "userA@example.com",
		"given_name": "user",
		"family_name": "A"
	}`),
}

var magicLinkTestOU = testutils.OrganizationUnit{
	Handle:      "magic-link-test-ou",
	Name:        "Magic Link Test OU",
	Description: "Organization unit for magic link authentication tests",
}

type magicLinkAuthFlowTestSuite struct {
	suite.Suite
	config           *common.TestSuiteConfig
	mockSMTPServer   *mockSMTPServer
	ouID             string
	appID            string
	userSchemaID     string
	originalPatchSet bool
}

func TestMagicLinkAuthFlowTestSuite(t *testing.T) {
	suite.Run(t, new(magicLinkAuthFlowTestSuite))
}

func (ts *magicLinkAuthFlowTestSuite) SetupSuite() {
	if os.Getenv("THUNDER_EXTRACTED_HOME") == "" {
		ts.T().Skip("Skipping magic link integration test - integration harness not initialized")
	}

	ts.config = &common.TestSuiteConfig{}

	server, err := newMockSMTPServer()
	ts.Require().NoError(err, "Failed to start mock SMTP server")
	ts.mockSMTPServer = server

	patch := map[string]interface{}{
		"email": map[string]interface{}{
			"smtp": map[string]interface{}{
				"host":                  server.Host(),
				"port":                  server.Port(),
				"username":              "",
				"password":              "",
				"from_address":          "no-reply@example.com",
				"enable_start_tls":      false,
				"enable_authentication": false,
			},
		},
	}

	if err := testutils.PatchDeploymentConfig(patch); err != nil {
		ts.T().Fatalf("Failed to patch deployment config: %v", err)
	}
	ts.originalPatchSet = true

	if err := testutils.RestartServer(); err != nil {
		ts.T().Fatalf("Failed to restart server with SMTP configuration: %v", err)
	}

	ouID, err := testutils.CreateOrganizationUnit(magicLinkTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	magicLinkTestUserSchema.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(magicLinkTestUserSchema)
	ts.Require().NoError(err, "Failed to create test user schema")
	ts.userSchemaID = schemaID

	magicLinkTestUser.OUID = ts.ouID
	userIDs, err := testutils.CreateMultipleUsers(magicLinkTestUser)
	ts.Require().NoError(err, "Failed to create test user")
	ts.config.CreatedUserIDs = userIDs

	flowID, err := testutils.CreateFlow(magicLinkAuthFlow)
	ts.Require().NoError(err, "Failed to create magic link flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	magicLinkTestApp.AuthFlowID = flowID
	magicLinkTestApp.OUID = ts.ouID

	appID, err := testutils.CreateApplication(magicLinkTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *magicLinkAuthFlowTestSuite) TearDownSuite() {
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		_ = testutils.DeleteFlow(flowID)
	}
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to clean up users: %v", err)
	}
	if ts.userSchemaID != "" {
		_ = testutils.DeleteUserType(ts.userSchemaID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
	if ts.mockSMTPServer != nil {
		_ = ts.mockSMTPServer.Stop()
	}
	if ts.originalPatchSet {
		_ = testutils.UpdateDeploymentConfig("../../resources/deployment.yaml")
		_ = testutils.RestartServer()
	}
}

func (ts *magicLinkAuthFlowTestSuite) TestMagicLinkLoginFlow() {
	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate magic link flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().Equal("VIEW", flowStep.Type)
	ts.Require().NotEmpty(flowStep.ExecutionID)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "email"), "Email input should be required")
	ts.Require().True(common.HasAction(flowStep.Data.Actions, "magic_link_action"),
		"Magic link action should be available")

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": "userB@example.com"},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit magic link email")
	ts.Require().Equal("INCOMPLETE", step2.FlowStatus)
	ts.Require().Equal("VIEW", step2.Type)
	ts.Require().NotEmpty(step2.ChallengeToken)
	ts.Require().True(common.HasInput(step2.Data.Inputs, "token"), "Magic link token input should be required")
	ts.Require().Equal("true", step2.Data.AdditionalData["emailSent"], "Magic link email should be sent")

	emailBody := ts.mockSMTPServer.LastMessage()
	ts.Require().NotEmpty(emailBody, "Expected magic link email to be captured")

	token := extractMagicLinkToken(emailBody)
	ts.Require().NotEmpty(token, "Expected magic link token to be present in the email body")

	completeStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"token": token}, "",
		step2.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete magic link login")
	ts.Require().Equal("COMPLETE", completeStep.FlowStatus)
	ts.Require().NotEmpty(completeStep.Assertion)

	claims, err := testutils.ValidateJWTAssertionFields(completeStep.Assertion, ts.appID,
		magicLinkTestUserSchema.Name, ts.ouID, magicLinkTestOU.Name, magicLinkTestOU.Handle)
	ts.Require().NoError(err, "Failed to validate JWT assertion")
	ts.Require().Equal(magicLinkTestUserSchema.Name, claims.UserType)
	ts.Require().Equal(ts.ouID, claims.OUID)
}

type mockSMTPServer struct {
	listener net.Listener
	mu       sync.Mutex
	message  string
	port     int
}

func newMockSMTPServer() (*mockSMTPServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server := &mockSMTPServer{
		listener: listener,
		port:     listener.Addr().(*net.TCPAddr).Port,
	}
	go server.serve()
	return server, nil
}

func (s *mockSMTPServer) Host() string {
	return "127.0.0.1"
}

func (s *mockSMTPServer) Port() int {
	return s.port
}

func (s *mockSMTPServer) Stop() error {
	return s.listener.Close()
}

func (s *mockSMTPServer) LastMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.message
}

func (s *mockSMTPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *mockSMTPServer) handleConn(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Println("Error closing connection:", err)
		}
	}(conn)

	_, _ = conn.Write([]byte("220 localhost ESMTP ready\r\n"))
	reader := bufio.NewReader(conn)
	var dataMode bool
	var body strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		trimmed := strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(trimmed)

		if dataMode {
			if trimmed == "." {
				s.mu.Lock()
				s.message = body.String()
				s.mu.Unlock()
				_, _ = conn.Write([]byte("250 OK\r\n"))
				dataMode = false
				body.Reset()
				continue
			}
			body.WriteString(trimmed)
			body.WriteString("\n")
			continue
		}

		switch {
		case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
			_, _ = conn.Write([]byte("250-localhost\r\n250 OK\r\n"))
		case strings.HasPrefix(upper, "MAIL FROM:"):
			_, _ = conn.Write([]byte("250 OK\r\n"))
		case strings.HasPrefix(upper, "RCPT TO:"):
			_, _ = conn.Write([]byte("250 OK\r\n"))
		case upper == "DATA":
			_, _ = conn.Write([]byte("354 End data with <CR><LF>.<CR><LF>\r\n"))
			dataMode = true
		case upper == "RSET" || upper == "NOOP":
			_, _ = conn.Write([]byte("250 OK\r\n"))
		case upper == "QUIT":
			_, _ = conn.Write([]byte("221 Bye\r\n"))
			return
		default:
			_, _ = conn.Write([]byte("250 OK\r\n"))
		}
	}
}

func extractMagicLinkToken(message string) string {
	re := regexp.MustCompile(`token=([^"&<\s]+)`)
	match := re.FindStringSubmatch(message)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}
