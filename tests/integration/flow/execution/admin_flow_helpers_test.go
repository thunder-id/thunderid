// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// executeAdminFlowRequest runs an administration flow by id and returns the status and the decoded
// step. When authenticated is false the request carries no token, which is how the flow's own
// PermissionValidator node is exercised.
//
// The bearer token is set on a raw client because the shared test clients treat /flow/execute as a
// public endpoint and skip token injection, which would make every administration request anonymous.
func executeAdminFlowRequest(t *testing.T, client *http.Client, flowID string,
	inputs map[string]string, authenticated bool) (int, map[string]interface{}) {
	t.Helper()

	reqBody, err := json.Marshal(map[string]interface{}{"flowId": flowID, "inputs": inputs})
	if err != nil {
		t.Fatalf("Failed to encode flow execution request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to build flow execution request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if authenticated {
		token, tokenErr := testutils.GetAccessToken()
		if tokenErr != nil {
			t.Fatalf("Failed to obtain admin access token: %v", tokenErr)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute administration flow: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read flow execution response: %v", err)
	}

	step := map[string]interface{}{}
	_ = json.Unmarshal(body, &step)
	return resp.StatusCode, step
}
