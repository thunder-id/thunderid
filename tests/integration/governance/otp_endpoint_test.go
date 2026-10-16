// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	otpSendEndpoint   = "/auth/otp/sms/send"
	otpVerifyEndpoint = "/auth/otp/sms/verify"

	// smsArrivalTimeout bounds the wait for a message the server is expected to send.
	smsArrivalTimeout = 5 * time.Second
)

// sendDirectOTP asks the direct send endpoint for a code to recipient through senderID, and returns
// the session token, the code the mock provider received, and the HTTP status. The endpoint is not
// governed, so it always sends; an empty code means nothing arrived.
func sendDirectOTP(t *testing.T, server *testutils.MockNotificationServer, senderID, recipient string) (
	sessionToken, code string, status int) {
	t.Helper()

	server.ClearMessages()

	payload, err := json.Marshal(map[string]interface{}{
		"senderId":  senderID,
		"recipient": recipient,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+otpSendEndpoint, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err, "the send request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var sent struct {
		SessionToken string `json:"sessionToken"`
	}
	_ = json.Unmarshal(body, &sent)

	if message := awaitSMS(server); message != nil {
		code = message.OTP
	}
	return sent.SessionToken, code, resp.StatusCode
}

// verifyDirectOTP presents a code against a session at the direct verify endpoint and returns the
// whole answer, since a held account's refusal must match a wrong code's.
func verifyDirectOTP(t *testing.T, sessionToken, code string) authResult {
	t.Helper()

	payload, err := json.Marshal(map[string]interface{}{
		"sessionToken": sessionToken,
		"otp":          code,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+otpVerifyEndpoint, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	require.NoError(t, err, "the verify request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return authResult{status: resp.StatusCode, body: string(body)}
}

// awaitSMS returns the next message the mock provider receives, or nil if none arrives in time.
func awaitSMS(server *testutils.MockNotificationServer) *testutils.SMSMessage {
	deadline := time.Now().Add(smsArrivalTimeout)
	for {
		if message := server.GetLastMessage(); message != nil {
			return message
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// smsCount returns how many messages the mock provider holds, without consuming them.
func smsCount(t *testing.T, server *testutils.MockNotificationServer) int {
	t.Helper()

	resp, err := http.Get(server.GetURL() + "/messages")
	require.NoError(t, err, "failed to read the mock provider's messages")
	defer resp.Body.Close()

	var messages []testutils.SMSMessage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&messages), "unreadable mock provider messages")
	return len(messages)
}
