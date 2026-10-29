// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// flowExtInvalidExecConfigCode is the flow save error for an executor's invalid properties, which
	// covers a missing connection, malformed key lists and maxAttempts out of range.
	flowExtInvalidExecConfigCode = "FLM-1023"

	flowExtConnectionURL = "https://ext.example.com/flow-ext-validation"
)

// flowExtOU owns every connection the extension validation suite creates.
var flowExtOU = testutils.OrganizationUnit{Handle: "flow-ext-validation-ou", Name: "Flow Extension Validation OU"}

// flowExtError is the part of a flow save error these tests read.
type flowExtError struct {
	Code        string `json:"code"`
	Description struct {
		Params map[string]string `json:"params"`
	} `json:"description"`
}

// FlowExtensionValidationTestSuite covers what saving a flow with an extension step refuses: no
// connection or one that does not exist (AC1.5), an inline endpoint, malformed property values and
// runtime-data keys outside the allow list.
type FlowExtensionValidationTestSuite struct {
	suite.Suite
	ouID         string
	connectionID string
}

func TestFlowExtensionValidationTestSuite(t *testing.T) {
	suite.Run(t, new(FlowExtensionValidationTestSuite))
}

func (s *FlowExtensionValidationTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(flowExtOU)
	s.Require().NoError(err, "Failed to create the organization unit")
	s.ouID = ouID

	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: "flow-ext-validation", OUID: s.ouID, URL: flowExtConnectionURL,
	})
	s.Require().NoError(err, "Failed to create the HTTP connection")
	s.connectionID = connectionID
}

func (s *FlowExtensionValidationTestSuite) TearDownSuite() {
	if s.connectionID != "" {
		if err := testutils.DeleteHTTPConnection(s.connectionID); err != nil {
			s.T().Logf("Failed to delete HTTP connection during teardown: %v", err)
		}
	}
	// The OU goes last: an OU that owns a connection cannot be deleted.
	if s.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(s.ouID); err != nil {
			s.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
}

func (s *FlowExtensionValidationTestSuite) request(method, path string, body interface{}) (int, []byte) {
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
func (s *FlowExtensionValidationTestSuite) createFlow(flow testutils.Flow) (int, []byte, string) {
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

// assertRefused checks that status and body are the invalid executor configuration error, naming
// property when it is not "".
func (s *FlowExtensionValidationTestSuite) assertRefused(status int, body []byte, property, context string) {
	s.T().Helper()
	s.Equal(http.StatusBadRequest, status, "%s should be refused at save: %s", context, string(body))
	var e flowExtError
	s.Require().NoError(json.Unmarshal(body, &e), "%s: the error should decode: %s", context, string(body))
	s.Equal(flowExtInvalidExecConfigCode, e.Code, "%s should be an executor config error", context)
	if property != "" {
		s.Equal(property, e.Description.Params["propertyKey"], "%s: the error should name the property", context)
	}
}

func (s *FlowExtensionValidationTestSuite) assertFlowRefused(flow testutils.Flow, property, context string) {
	s.T().Helper()
	status, body, _ := s.createFlow(flow)
	s.assertRefused(status, body, property, context)
}

// extensionStepProperties reads the stored properties of ext_step in flowID.
func (s *FlowExtensionValidationTestSuite) extensionStepProperties(flowID string) map[string]interface{} {
	s.T().Helper()
	status, body := s.request(http.MethodGet, flowsEndpoint+"/"+flowID, nil)
	s.Require().Equal(http.StatusOK, status, "The flow should be readable: %s", string(body))
	var flow struct {
		Nodes []struct {
			ID         string                 `json:"id"`
			Properties map[string]interface{} `json:"properties"`
		} `json:"nodes"`
	}
	s.Require().NoError(json.Unmarshal(body, &flow), "The flow should decode: %s", string(body))
	for _, node := range flow.Nodes {
		if node.ID == "ext_step" {
			return node.Properties
		}
	}
	s.Require().Fail("The flow should hold ext_step", string(body))
	return nil
}

// savedFlow saves a well-formed flow and returns its ID.
func (s *FlowExtensionValidationTestSuite) savedFlow(name, handle string) string {
	s.T().Helper()
	status, body, flowID := s.createFlow(testutils.ExtensionAuthFlow(name, handle,
		map[string]interface{}{"connectionId": s.connectionID}))
	s.Require().Equal(http.StatusCreated, status, "The flow should be saved: %s", string(body))
	s.Require().NotEmpty(flowID, "The saved flow should have an ID: %s", string(body))
	return flowID
}

// deletedConnectionID returns the ID of a connection that existed and is gone.
func (s *FlowExtensionValidationTestSuite) deletedConnectionID() string {
	s.T().Helper()
	goneID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: "flow-ext-validation-deleted", OUID: s.ouID, URL: flowExtConnectionURL,
	})
	s.Require().NoError(err, "Failed to create the connection to delete")
	s.Require().NoError(testutils.DeleteHTTPConnection(goneID), "Failed to delete the unused connection")
	return goneID
}

func (s *FlowExtensionValidationTestSuite) TestExtensionWithoutConnectionIsRefused() {
	for name, properties := range map[string]map[string]interface{}{
		"Absent": nil,
		"Empty":  {"connectionId": ""},
	} {
		s.Run(name, func() {
			s.assertFlowRefused(testutils.ExtensionAuthFlow("Flow Ext No Connection "+name,
				"flow_ext_none_"+strings.ToLower(name), properties), "connectionId", "A step without a connection")
		})
	}
}

func (s *FlowExtensionValidationTestSuite) TestExtensionWithMissingConnectionIsRefused() {
	goneID := s.deletedConnectionID()
	for name, connectionID := range map[string]string{
		"Deleted":       goneID,
		"PaddedDeleted": "  " + goneID + "  ",
		"NeverExisted":  "00000000-0000-0000-0000-00000000c0de",
	} {
		s.Run(name, func() {
			s.assertFlowRefused(testutils.ExtensionAuthFlow("Flow Ext Missing Connection "+name,
				"flow_ext_missing_"+strings.ToLower(name), map[string]interface{}{"connectionId": connectionID}),
				"connectionId", "A step naming a connection that does not exist")
		})
	}
}

func (s *FlowExtensionValidationTestSuite) TestUpdateToAMissingConnectionIsRefused() {
	flowID := s.savedFlow("Flow Ext Update Missing Connection", "flow_ext_update_missing")

	status, body := s.request(http.MethodPut, flowsEndpoint+"/"+flowID,
		testutils.ExtensionAuthFlow("Flow Ext Update Missing Connection", "flow_ext_update_missing",
			map[string]interface{}{"connectionId": s.deletedConnectionID()}))
	s.assertRefused(status, body, "connectionId", "An update naming a missing connection")
	s.Equal(s.connectionID, s.extensionStepProperties(flowID)["connectionId"],
		"A refused update must leave the stored flow unchanged")
}

func (s *FlowExtensionValidationTestSuite) TestMalformedPropertiesAreRefused() {
	cases := []struct {
		name     string
		property string
		value    interface{}
	}{
		{name: "InputKeysEmpty", property: "inputKeys", value: []string{}},
		{name: "InputKeysBlank", property: "inputKeys", value: []string{" "}},
		{name: "InputKeysNonString", property: "inputKeys", value: []interface{}{1}},
		{name: "InputKeysString", property: "inputKeys", value: "username"},
		{name: "RuntimeDataKeysEmpty", property: "runtimeDataKeys", value: []string{}},
		{name: "RuntimeDataKeysBlank", property: "runtimeDataKeys", value: []string{"userID", ""}},
		{name: "RuntimeDataKeysNonString", property: "runtimeDataKeys", value: []interface{}{true}},
		{name: "RuntimeDataKeysString", property: "runtimeDataKeys", value: "userID"},
		{name: "ResponseKeysEmpty", property: "responseKeys", value: []string{}},
		{name: "ResponseKeysBlank", property: "responseKeys", value: []string{"riskScore", " "}},
		{name: "ResponseKeysNonString", property: "responseKeys", value: []interface{}{1}},
		{name: "ResponseKeysBlankString", property: "responseKeys", value: " "},
		{name: "MaxAttemptsZero", property: "maxAttempts", value: 0},
		{name: "MaxAttemptsTwentyOne", property: "maxAttempts", value: 21},
		{name: "MaxAttemptsNegative", property: "maxAttempts", value: -1},
		{name: "MaxAttemptsFraction", property: "maxAttempts", value: 2.5},
		{name: "MaxAttemptsString", property: "maxAttempts", value: "5"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.assertFlowRefused(testutils.ExtensionAuthFlow("Flow Ext Malformed "+tc.name,
				"flow_ext_malformed_"+strings.ToLower(tc.name),
				map[string]interface{}{"connectionId": s.connectionID, tc.property: tc.value}),
				tc.property, tc.property+" "+tc.name)
		})
	}

	flowID := s.savedFlow("Flow Ext Malformed Update", "flow_ext_malformed_update")
	status, body := s.request(http.MethodPut, flowsEndpoint+"/"+flowID,
		testutils.ExtensionAuthFlow("Flow Ext Malformed Update", "flow_ext_malformed_update",
			map[string]interface{}{"connectionId": s.connectionID, "inputKeys": []string{}}))
	s.assertRefused(status, body, "inputKeys", "An update with an empty inputKeys")
	s.NotContains(s.extensionStepProperties(flowID), "inputKeys", "A refused update must leave the flow unchanged")
}

func (s *FlowExtensionValidationTestSuite) TestRuntimeDataKeyOutsideTheAllowListIsRefused() {
	// Only userID and extension data may be sent. Each name below is runtime data a flow can hold,
	// or a near miss of an allowed name.
	for i, key := range []string{"applicationId", "clientId", "otpSessionToken", "oauthState", "failureReason",
		"riskScore", "userId", "UserID", "extension", "extensions.ext_risk.riskScore"} {
		s.Run(key, func() {
			s.assertFlowRefused(testutils.ExtensionAuthFlow("Flow Ext Runtime Key "+key,
				fmt.Sprintf("flow_ext_runtime_key_%d", i),
				map[string]interface{}{"connectionId": s.connectionID,
					"runtimeDataKeys": []string{"userID", key}}),
				"runtimeDataKeys", "runtimeDataKeys naming "+key)
		})
	}
}

func (s *FlowExtensionValidationTestSuite) TestWellFormedPropertiesAreAccepted() {
	// The controls for the refusals: the bounds of maxAttempts, each accepted form of the key lists and
	// both kinds of allowed runtime-data key.
	cases := []struct {
		name     string
		property string
		value    interface{}
	}{
		{name: "MaxAttemptsOne", property: "maxAttempts", value: 1},
		{name: "MaxAttemptsTwenty", property: "maxAttempts", value: 20},
		{name: "InputKeysList", property: "inputKeys", value: []string{"username"}},
		{name: "RuntimeDataKeysList", property: "runtimeDataKeys", value: []string{"userID",
			"extension.ext_risk.riskScore"}},
		{name: "RuntimeDataKeysBareNamespace", property: "runtimeDataKeys", value: []string{"extension."}},
		{name: "ResponseKeysList", property: "responseKeys", value: []string{"riskScore"}},
		{name: "ResponseKeysString", property: "responseKeys", value: "riskScore,challengeId"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			status, body, flowID := s.createFlow(testutils.ExtensionAuthFlow("Flow Ext Well Formed "+tc.name,
				"flow_ext_wellformed_"+strings.ToLower(tc.name),
				map[string]interface{}{"connectionId": s.connectionID, tc.property: tc.value}))
			s.Equal(http.StatusCreated, status, "%s %s should be accepted: %s", tc.property, tc.name, string(body))
			s.NotEmpty(flowID, "%s: the saved flow should have an ID", tc.name)
		})
	}
}

func (s *FlowExtensionValidationTestSuite) TestInlineEndpointPropertiesAreRefused() {
	for property, value := range map[string]interface{}{
		"url":     flowExtConnectionURL,
		"headers": map[string]interface{}{"X-Inline": "inline"},
		"timeout": 5,
	} {
		s.Run(property, func() {
			s.assertFlowRefused(testutils.ExtensionAuthFlow("Flow Ext Inline "+property, "flow_ext_inline_"+property,
				map[string]interface{}{"connectionId": s.connectionID, property: value}),
				"", "An inline "+property)
		})
	}
}
