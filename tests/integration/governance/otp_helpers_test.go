// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	otpSendEndpoint   = "/auth/otp/sms/send"
	otpVerifyEndpoint = "/auth/otp/sms/verify"
)

// mobileSeq numbers the mobile numbers newMobile hands out.
var mobileSeq atomic.Int64

// newMobile returns a mobile number no other account in this package holds, so an OTP recipient
// always names one account.
func newMobile() string {
	return fmt.Sprintf("+1600%07d", mobileSeq.Add(1))
}

// newSMSSender starts a mock SMS provider and creates a notification sender for it. Both are removed
// when t ends, after anything registered later that names the sender.
func newSMSSender(t *testing.T, name string) (*testutils.MockNotificationServer, string) {
	t.Helper()

	server := testutils.NewMockNotificationServer(0)
	require.NoError(t, server.Start(), "failed to start the mock SMS provider")
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Logf("teardown: failed to stop the mock SMS provider: %v", err)
		}
	})

	senderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:     "Governance " + name + " SMS sender",
		Provider: "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: server.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	require.NoError(t, err, "failed to create the SMS sender")
	t.Cleanup(func() {
		if err := testutils.DeleteNotificationSender(senderID); err != nil {
			t.Logf("teardown: failed to delete the SMS sender: %v", err)
		}
	})
	return server, senderID
}

// sendDirectOTP asks the direct send endpoint for a code to recipient through senderID, and returns
// the session token and the code the mock provider received. The endpoint is not governed, so it
// always sends, and a refused send or one that delivers nothing fails the test.
func sendDirectOTP(t *testing.T, server *testutils.MockNotificationServer, senderID, recipient string) (
	sessionToken, code string) {
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
	require.Equal(t, http.StatusOK, resp.StatusCode, "the direct send must be accepted: %s", body)

	var sent struct {
		SessionToken string `json:"sessionToken"`
	}
	_ = json.Unmarshal(body, &sent)

	message := awaitDelivered(t, server.GetLastMessage, "the direct send must deliver a code")
	require.NotEmpty(t, message.OTP, "the delivered message must carry a code")
	return sent.SessionToken, message.OTP
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

// failDirectOTP presents count wrong codes through the direct endpoints, each against its own freshly
// issued session so that nothing but the wrong code is tested.
func failDirectOTP(t *testing.T, server *testutils.MockNotificationServer, senderID, recipient string,
	count int) {
	t.Helper()

	for i := range count {
		sessionToken, code := sendDirectOTP(t, server, senderID, recipient)
		require.NotEmpty(t, sessionToken, "attempt %d produced no session", i+1)
		require.False(t, verifyDirectOTP(t, sessionToken, wrongCodeFor(code)).succeeded(),
			"the wrong code must be refused on attempt %d", i+1)
	}
}

// requireNoCodeSent requires that the step just taken sent nothing. The SMS executor sends before its
// step returns (the notification sender service posts to the provider within the request), so a sent
// code is already at the mock by then.
func requireNoCodeSent(t *testing.T, server *testutils.MockNotificationServer, msg string) {
	t.Helper()
	require.Empty(t, server.GetMessages(), msg)
}
