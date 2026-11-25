// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// flowSaveSuite is the base of the suites that save flows over HTTP. ouID is the organization unit a saved flow
// belongs to when it names none.
type flowSaveSuite struct {
	suite.Suite
	ouID string
}

func (s *flowSaveSuite) request(method, path string, body interface{}) (int, []byte) {
	s.T().Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		s.Require().NoError(err, "Failed to marshal the request body")
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	s.Require().NoError(err, "Failed to build %s %s", method, path)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err, "%s %s failed", method, path)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	s.Require().NoError(err, "Failed to read the %s %s response", method, path)
	return resp.StatusCode, raw
}

// createFlow saves flow, deletes it when the test ends if it was saved, and returns the status, the
// body and the flow's ID.
func (s *flowSaveSuite) createFlow(flow testutils.Flow) (int, []byte, string) {
	s.T().Helper()
	if flow.OUID == "" {
		flow.OUID = s.ouID
	}
	status, body := s.request(http.MethodPost, flowsEndpoint, flow)
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &created)
	if created.ID != "" {
		s.T().Cleanup(func() {
			if err := testutils.DeleteFlow(created.ID); err != nil {
				s.T().Logf("Failed to delete flow %s: %v", created.ID, err)
			}
		})
	}
	return status, body, created.ID
}
