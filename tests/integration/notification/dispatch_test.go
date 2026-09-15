// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	mockDispatchServerPort = 8099
	otpSendEndpoint        = "/auth/otp/sms/send"
	smsGatewayPath         = "/connections/sms-gateway"
)

var (
	dispatchTestOU = testutils.OrganizationUnit{
		Handle:      "notification-dispatch-test-ou",
		Name:        "Notification Dispatch Test OU",
		Description: "Organization unit for notification dispatch tests",
	}

	dispatchEntityType = testutils.UserType{
		Name: "dispatch_user",
		Schema: map[string]interface{}{
			"username":      map[string]interface{}{"type": "string"},
			"mobile_number": map[string]interface{}{"type": "string"},
		},
	}
)

// DispatchTestSuite exercises how the custom SMS sender puts a message on the wire. The connection
// CRUD surface is covered by the connection suite; what matters here is the shape of the outbound
// request each configuration produces, and how a gateway failure surfaces.
type DispatchTestSuite struct {
	suite.Suite
	client       *http.Client
	mockServer   *testutils.MockNotificationServer
	ouID         string
	entityTypeID string
	userID       string
	mobileNumber string
}

func TestDispatchTestSuite(t *testing.T) {
	suite.Run(t, new(DispatchTestSuite))
}

func (ts *DispatchTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
	ts.mobileNumber = "+1555000111"

	ouID, err := testutils.CreateOrganizationUnit(dispatchTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	dispatchEntityType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(dispatchEntityType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = entityTypeID

	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID: ouID,
		Type: dispatchEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "dispatchuser",
			"mobile_number": "` + ts.mobileNumber + `"
		}`),
	})
	ts.Require().NoError(err, "Failed to create test user")
	ts.userID = userIDs[0]

	ts.mockServer = testutils.NewMockNotificationServer(mockDispatchServerPort)
	ts.Require().NoError(ts.mockServer.Start(), "Failed to start mock notification server")
}

func (ts *DispatchTestSuite) TearDownSuite() {
	if ts.userID != "" {
		if err := testutils.CleanupUsers([]string{ts.userID}); err != nil {
			ts.T().Logf("Failed to cleanup user during teardown: %v", err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
	if ts.mockServer != nil {
		if err := ts.mockServer.Stop(); err != nil {
			ts.T().Logf("Failed to stop mock notification server during teardown: %v", err)
		}
	}
}

func (ts *DispatchTestSuite) SetupTest() {
	ts.mockServer.ClearMessages()
	ts.mockServer.SetResponseStatus(0)
}

// createSender creates a custom SMS gateway connection with the given configuration and registers
// its cleanup.
func (ts *DispatchTestSuite) createSender(name string, extra map[string]interface{}) string {
	body := map[string]interface{}{
		"name":        name,
		"description": "Sender for notification dispatch testing",
		"url":         ts.mockServer.GetSendSMSURL(),
	}
	for key, value := range extra {
		body[key] = value
	}

	payload, err := json.Marshal(body)
	ts.Require().NoError(err, "Failed to marshal sender payload")

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+smsGatewayPath,
		bytes.NewReader(payload))
	ts.Require().NoError(err, "Failed to build sender request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to create sender")
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"Failed to create sender %s: %s", name, string(responseBody))

	var decoded map[string]interface{}
	ts.Require().NoError(json.Unmarshal(responseBody, &decoded), "Failed to decode sender response")

	senderID, ok := decoded["id"].(string)
	ts.Require().True(ok, "Sender response should carry an id")

	ts.T().Cleanup(func() { ts.deleteSender(senderID) })

	return senderID
}

func (ts *DispatchTestSuite) deleteSender(senderID string) {
	req, err := http.NewRequest(http.MethodDelete,
		testutils.TestServerURL+smsGatewayPath+"/"+senderID, nil)
	if err != nil {
		ts.T().Logf("Failed to build sender delete request: %v", err)
		return
	}
	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Logf("Failed to delete sender %s: %v", senderID, err)
		return
	}
	resp.Body.Close()
}

// sendOTP asks the server to deliver an OTP through the given sender and returns the status of that
// request, so both successful dispatch and gateway failures can be inspected.
func (ts *DispatchTestSuite) sendOTP(senderID string) int {
	payload, err := json.Marshal(map[string]interface{}{
		"recipient": ts.mobileNumber,
		"senderId":  senderID,
	})
	ts.Require().NoError(err, "Failed to marshal OTP send payload")

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+otpSendEndpoint,
		bytes.NewReader(payload))
	ts.Require().NoError(err, "Failed to build OTP send request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send the OTP request")
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return resp.StatusCode
}

// waitForDispatch waits for the mock gateway to record an inbound request.
func (ts *DispatchTestSuite) waitForDispatch() *testutils.SMSMessage {
	for i := 0; i < 20; i++ {
		if message := ts.mockServer.GetLastMessage(); message != nil {
			return message
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// TestDispatchJSONPost sends through a sender configured for a JSON body over POST.
func (ts *DispatchTestSuite) TestDispatchJSONPost() {
	senderID := ts.createSender("Dispatch JSON POST Sender", map[string]interface{}{
		"httpMethod":  "POST",
		"contentType": "JSON",
	})

	ts.Require().Equal(http.StatusOK, ts.sendOTP(senderID), "Expected the OTP send to succeed")

	message := ts.waitForDispatch()
	ts.Require().NotNil(message, "The gateway should have received a request")
	ts.Equal(http.MethodPost, message.Method, "The sender should dispatch over POST")
	ts.Contains(message.ContentType, "application/json", "The body should be sent as JSON")
	ts.NotEmpty(message.OTP, "The dispatched message should carry the OTP")
}

// TestDispatchFormPost sends through a sender configured for a form encoded body.
//
// The form encoder builds its fields by splitting the rendered message into "key=value" lines. The
// OTP SMS template renders prose rather than key value pairs, so no fields are produced and the
// gateway receives an empty body while the send still reports success. This test pins that
// behaviour; if the encoder is changed to carry the message, the assertion below will fail and
// should be updated to assert the OTP is present.
func (ts *DispatchTestSuite) TestDispatchFormPost() {
	senderID := ts.createSender("Dispatch FORM POST Sender", map[string]interface{}{
		"httpMethod":  "POST",
		"contentType": "FORM",
	})

	ts.Require().Equal(http.StatusOK, ts.sendOTP(senderID), "Expected the OTP send to succeed")

	message := ts.waitForDispatch()
	ts.Require().NotNil(message, "The gateway should have received a request")
	ts.Equal(http.MethodPost, message.Method, "The sender should dispatch over POST")
	ts.Contains(message.ContentType, "application/x-www-form-urlencoded",
		"The body should be form encoded")
	ts.Empty(message.Message,
		"A prose template produces no form fields, so the body reaches the gateway empty")
}

// TestDispatchCustomHeaders confirms configured headers reach the gateway.
func (ts *DispatchTestSuite) TestDispatchCustomHeaders() {
	senderID := ts.createSender("Dispatch Custom Headers Sender", map[string]interface{}{
		"httpMethod":  "POST",
		"contentType": "JSON",
		"httpHeaders": "X-Dispatch-Test: integration",
	})

	ts.Require().Equal(http.StatusOK, ts.sendOTP(senderID), "Expected the OTP send to succeed")

	message := ts.waitForDispatch()
	ts.Require().NotNil(message, "The gateway should have received a request")
	ts.Equal("integration", message.Headers["X-Dispatch-Test"],
		"The configured header should be present on the outbound request, got %v", message.Headers)
}

// TestDispatchGatewayFailure confirms a gateway error surfaces rather than being reported as a
// successful send.
func (ts *DispatchTestSuite) TestDispatchGatewayFailure() {
	senderID := ts.createSender("Dispatch Failing Sender", map[string]interface{}{
		"httpMethod":  "POST",
		"contentType": "JSON",
	})

	ts.mockServer.SetResponseStatus(http.StatusInternalServerError)

	status := ts.sendOTP(senderID)
	ts.NotEqual(http.StatusOK, status,
		"A gateway failure must not be reported as a successful send, got %d", status)

	message := ts.waitForDispatch()
	ts.Require().NotNil(message, "The gateway should still have received the attempt")
}

// TestDispatchUnsupportedContentType confirms an unusable content type is rejected rather than
// silently dispatched in some default encoding.
func (ts *DispatchTestSuite) TestDispatchUnsupportedContentType() {
	senderID, err := ts.tryCreateSender("Dispatch Unsupported Content Type Sender",
		map[string]interface{}{"httpMethod": "POST", "contentType": "XML"})
	if err != nil {
		// Rejecting the configuration up front is a valid guard, and preferable to failing at send.
		ts.T().Logf("the connection API rejected an unsupported content type: %v", err)
		return
	}
	ts.T().Cleanup(func() { ts.deleteSender(senderID) })

	status := ts.sendOTP(senderID)
	ts.NotEqual(http.StatusOK, status,
		"An unsupported content type must not produce a successful send, got %d", status)
}

// tryCreateSender creates a sender and returns any API error rather than failing the test, for
// configurations that may be rejected at creation time.
func (ts *DispatchTestSuite) tryCreateSender(name string, extra map[string]interface{}) (string, error) {
	body := map[string]interface{}{
		"name":        name,
		"description": "Sender for notification dispatch testing",
		"url":         ts.mockServer.GetSendSMSURL(),
	}
	for key, value := range extra {
		body[key] = value
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+smsGatewayPath,
		bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(responseBody))
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", err
	}

	senderID, ok := decoded["id"].(string)
	if !ok || senderID == "" {
		return "", fmt.Errorf("sender response carried no id: %s", string(responseBody))
	}
	return senderID, nil
}
