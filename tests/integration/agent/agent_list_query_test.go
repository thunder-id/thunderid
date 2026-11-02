// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// I18nMessage is an API message, which the server emits as an object but older surfaces emit as a
// plain string.
type I18nMessage struct {
	Key          string `json:"key,omitempty"`
	DefaultValue string `json:"defaultValue,omitempty"`
}

// UnmarshalJSON accepts either a bare string or the structured form.
func (m *I18nMessage) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		m.DefaultValue = s
		return nil
	}
	type alias I18nMessage
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*m = I18nMessage(a)
	return nil
}

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Code        string      `json:"code"`
	Message     I18nMessage `json:"message"`
	Description I18nMessage `json:"description,omitempty"`
}

// AgentListQueryTestSuite covers the rejection branches of GET /agents query parsing.
//
// The suite creates no agents. Every case here is rejected inside parsePaginationParams or
// parseFilterParams (`agent/handler.go:219-263`) before the service or the database is reached, so
// fixtures would contribute to no assertion. Asserting the exact code is also what proves the
// server rejected the request rather than silently dropping an unparseable filter and answering
// with an unfiltered list.
type AgentListQueryTestSuite struct {
	suite.Suite
}

func TestAgentListQueryTestSuite(t *testing.T) {
	suite.Run(t, new(AgentListQueryTestSuite))
}

// getAgents issues a list request with the given raw query string.
func (s *AgentListQueryTestSuite) getAgents(rawQuery string) *http.Response {
	s.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+agentBasePath+"?"+rawQuery, nil)
	s.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err)
	return resp
}

// TestInvalidListQueryParametersRejected verifies each malformed pagination or filter parameter is
// refused with its own exact status and product error code, rather than being clamped or ignored.
func (s *AgentListQueryTestSuite) TestInvalidListQueryParametersRejected() {
	cases := []struct {
		name        string
		query       string
		code        string
		message     string
		description string
	}{
		{
			// Scenario 34: zero is not "unlimited" and not "use the default"; it is invalid.
			name:        "limit of zero",
			query:       "limit=0",
			code:        "AGT-1011",
			message:     "Invalid pagination parameter",
			description: "The limit parameter must be between 1 and 100",
		},
		{
			// Scenario 35: one past the documented maximum.
			name:        "limit above the maximum",
			query:       "limit=101",
			code:        "AGT-1011",
			message:     "Invalid pagination parameter",
			description: "The limit parameter must be between 1 and 100",
		},
		{
			// Scenario 36: non-numeric offset.
			name:        "offset not a number",
			query:       "offset=abc",
			code:        "AGT-1012",
			message:     "Invalid pagination parameter",
			description: "The offset parameter must be a non-negative integer",
		},
		{
			// Scenario 37: negative offset.
			name:        "negative offset",
			query:       "offset=-1",
			code:        "AGT-1012",
			message:     "Invalid pagination parameter",
			description: "The offset parameter must be a non-negative integer",
		},
		{
			// Scenario 39: one representative malformed filter. Every rejection branch of
			// parseFilterParams returns this same code, so further permutations add nothing.
			name:        "filter missing the eq operator",
			query:       "filter=" + url.QueryEscape("name"),
			code:        "AGT-1020",
			message:     "Invalid filter parameter",
			description: "The filter format is invalid",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			resp := s.getAgents(tc.query)
			defer func() { _ = resp.Body.Close() }()

			s.Require().Equal(http.StatusBadRequest, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			s.Require().NoError(err)

			var errResp ErrorResponse
			s.Require().NoError(json.Unmarshal(body, &errResp), "error body: %s", string(body))
			s.Equal(tc.code, errResp.Code, "error body: %s", string(body))
			s.Equal(tc.message, errResp.Message.DefaultValue)
			s.Equal(tc.description, errResp.Description.DefaultValue)
		})
	}
}

// TestListQueryBoundaryValuesAccepted is the control for the rejections above: the values one step
// inside each boundary are accepted. Without it, "limit=101 is rejected" and "offset=-1 is
// rejected" would both be satisfied by a server that rejected every limit and offset.
func (s *AgentListQueryTestSuite) TestListQueryBoundaryValuesAccepted() {
	cases := []struct {
		name  string
		query string
	}{
		{name: "limit at the maximum", query: "limit=100"},
		{name: "offset of zero", query: "offset=0"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			resp := s.getAgents(tc.query)
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			s.Require().NoError(err)
			s.Equal(http.StatusOK, resp.StatusCode, "response body: %s", string(body))
		})
	}
}
