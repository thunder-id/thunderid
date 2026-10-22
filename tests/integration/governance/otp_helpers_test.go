// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	otpSendEndpoint   = "/auth/otp/sms/send"
	otpVerifyEndpoint = "/auth/otp/sms/verify"
)

// sendDirectOTP asks the direct send endpoint for a code to recipient through senderID, and returns
// the session token, the code the mock provider received, and the HTTP status. The endpoint is not
// governed, so it always sends, and a send that delivers nothing fails the test.
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

	message := awaitDelivered(t, server.GetLastMessage, "the direct send must deliver a code")
	require.NotEmpty(t, message.OTP, "the delivered message must carry a code")
	return sent.SessionToken, message.OTP, resp.StatusCode
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

// wrongCodeFor returns a code of the same shape that differs from it in every digit.
func wrongCodeFor(code string) string {
	wrong := []rune(code)
	for i, r := range wrong {
		if r == '0' {
			wrong[i] = '1'
			continue
		}
		wrong[i] = '0'
	}
	return string(wrong)
}
