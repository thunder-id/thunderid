// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/suite"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

func (s *ModelTestSuite) TestNodeExecutionRecord_GetDuration() {
	tests := []struct {
		name      string
		startTime int64
		endTime   int64
		expected  int64
	}{
		{"Valid duration calculation", 100, 150, 50000},
		{"Zero start time", 0, 150, 0},
		{"Zero end time", 100, 0, 0},
		{"Both times zero", 0, 0, 0},
		{"Large time values", 1000000, 1000100, 100000},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			record := providers.NodeExecutionRecord{StartTime: tt.startTime, EndTime: tt.endTime}
			duration := record.GetDuration()
			s.Equal(tt.expected, duration)
		})
	}
}

func (s *ModelTestSuite) TestInput_IsSensitive() {
	tests := []struct {
		name     string
		input    providers.Input
		expected bool
	}{
		{"Password input is sensitive",
			providers.Input{Identifier: "password", Type: providers.InputTypePassword}, true},
		{"OTP input is sensitive", providers.Input{Identifier: "otp", Type: providers.InputTypeOTP}, true},
		{"Text input is not sensitive", providers.Input{Identifier: "username", Type: providers.InputTypeText}, false},
		{"Empty type is not sensitive", providers.Input{Identifier: "field", Type: ""}, false},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.Equal(tt.expected, tt.input.IsSensitive())
		})
	}
}

func (s *ModelTestSuite) TestInput_DisplayName_ExcludedFromJSON() {
	input := providers.Input{
		Ref:         "ref_email",
		Identifier:  "email",
		Type:        providers.InputTypeText,
		Required:    true,
		DisplayName: "Email Address",
	}

	data, err := json.Marshal(input)
	s.Require().NoError(err)

	jsonStr := string(data)
	s.NotContains(jsonStr, "DisplayName", "DisplayName field name must not appear in JSON")
	s.NotContains(jsonStr, "displayName", "displayName key must not appear in JSON")
	s.NotContains(jsonStr, "Email Address", "DisplayName value must not appear in JSON")
	s.Contains(jsonStr, "email", "Identifier must still be serialized")

	var decoded providers.Input
	s.Require().NoError(json.Unmarshal(data, &decoded))
	s.Equal("", decoded.DisplayName, "DisplayName must remain empty after unmarshalling")
	s.Equal("email", decoded.Identifier)
}

func (s *ModelTestSuite) TestInput_OneTimeUse_IncludedInJSON() {
	input := providers.Input{
		Identifier: "challenge",
		Type:       providers.InputTypeText,
		Required:   true,
		OneTimeUse: true,
	}

	data, err := json.Marshal(input)
	s.Require().NoError(err)

	jsonStr := string(data)
	s.Contains(jsonStr, `"oneTimeUse":true`, "OneTimeUse must be serialized in JSON")

	var decoded providers.Input
	s.Require().NoError(json.Unmarshal(data, &decoded))
	s.True(decoded.OneTimeUse, "OneTimeUse must be preserved after unmarshalling")
}

func (s *ModelTestSuite) TestInput_OneTimeUse_OmittedWhenFalse() {
	input := providers.Input{
		Identifier: "username",
		Type:       providers.InputTypeText,
		Required:   true,
		OneTimeUse: false,
	}

	data, err := json.Marshal(input)
	s.Require().NoError(err)

	jsonStr := string(data)
	s.NotContains(jsonStr, "oneTimeUse", "OneTimeUse must be omitted from JSON when false")
}

func (s *ModelTestSuite) TestExecutionAttempt_GetDuration() {
	tests := []struct {
		name      string
		startTime int64
		endTime   int64
		expected  int64
	}{
		{"Valid duration calculation", 200, 250, 50000},
		{"Zero start time", 0, 250, 0},
		{"Zero end time", 200, 0, 0},
		{"Both times zero", 0, 0, 0},
		{"Same start and end time", 500, 500, 0},
		{"One millisecond duration", 1, 2, 1000},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			attempt := providers.ExecutionAttempt{StartTime: tt.startTime, EndTime: tt.endTime}
			duration := attempt.GetDuration()
			s.Equal(tt.expected, duration)
		})
	}
}

func (s *ModelTestSuite) TestAction_WithTypeField() {
	action := Action{
		Ref:      "submit",
		Type:     "password_auth",
		NextNode: "auth-node",
	}

	s.Equal("submit", action.Ref)
	s.Equal("password_auth", action.Type)
	s.Equal("auth-node", action.NextNode)
}

func (s *ModelTestSuite) TestAction_WithoutTypeField() {
	action := Action{
		Ref:      "submit",
		NextNode: "auth-node",
	}

	s.Equal("submit", action.Ref)
	s.Equal("", action.Type)
	s.Equal("auth-node", action.NextNode)
}

func (s *ModelTestSuite) TestAction_JSONMarshaling_WithType() {
	action := Action{
		Ref:      "google",
		Type:     "social_google",
		NextNode: "google-auth",
	}

	jsonBytes, err := json.Marshal(action)
	s.NoError(err)

	var unmarshaled Action
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	s.NoError(err)

	s.Equal("google", unmarshaled.Ref)
	s.Equal("social_google", unmarshaled.Type)
	s.Equal("google-auth", unmarshaled.NextNode)
}

func (s *ModelTestSuite) TestAction_JSONMarshaling_WithoutType() {
	action := Action{
		Ref:      "continue",
		NextNode: "next-node",
	}

	jsonBytes, err := json.Marshal(action)
	s.NoError(err)

	// Verify Type field is omitted when empty
	s.NotContains(string(jsonBytes), "\"type\"")

	var unmarshaled Action
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	s.NoError(err)

	s.Equal("continue", unmarshaled.Ref)
	s.Equal("", unmarshaled.Type)
	s.Equal("next-node", unmarshaled.NextNode)
}

func (s *ModelTestSuite) TestAction_JSONUnmarshaling_WithType() {
	jsonStr := `{"ref":"login","type":"basic_auth","nextNode":"auth"}`

	var action Action
	err := json.Unmarshal([]byte(jsonStr), &action)
	s.NoError(err)

	s.Equal("login", action.Ref)
	s.Equal("basic_auth", action.Type)
	s.Equal("auth", action.NextNode)
}

func (s *ModelTestSuite) TestAction_JSONUnmarshaling_WithoutType() {
	jsonStr := `{"ref":"logout","nextNode":"end"}`

	var action Action
	err := json.Unmarshal([]byte(jsonStr), &action)
	s.NoError(err)

	s.Equal("logout", action.Ref)
	s.Equal("", action.Type)
	s.Equal("end", action.NextNode)
}

func (s *ModelTestSuite) TestAction_MultipleActionsWithDifferentTypes() {
	actions := []Action{
		{Ref: "action1", Type: "type1", NextNode: "node1"},
		{Ref: "action2", Type: "type2", NextNode: "node2"},
		{Ref: "action3", Type: "", NextNode: "node3"},
	}

	s.Equal("type1", actions[0].Type)
	s.Equal("type2", actions[1].Type)
	s.Equal("", actions[2].Type)
}

func (s *ModelTestSuite) TestAction_TypeFieldOmitEmpty() {
	// Verify omitempty tag works
	action1 := Action{Ref: "ref1", Type: "type_value", NextNode: "node1"}
	action2 := Action{Ref: "ref2", Type: "", NextNode: "node2"}

	json1, _ := json.Marshal(action1)
	json2, _ := json.Marshal(action2)

	// Type should be present only when set
	s.Contains(string(json1), "\"type\":")
	s.NotContains(string(json2), "\"type\"")
}

func (s *ModelTestSuite) TestParseExtensionProperties_ReadsEveryProperty() {
	props, invalid := ParseExtensionProperties(map[string]interface{}{
		NodePropertyConnectionID:    "  conn-1 ",
		NodePropertyInputKeys:       []interface{}{"username", " password "},
		NodePropertyRuntimeDataKeys: []interface{}{"extension.risk_check.riskScore", "extension."},
		NodePropertyResponseKeys:    []interface{}{"riskScore"},
		NodePropertyMaxAttempts:     float64(3),
	})

	s.Empty(invalid)
	s.Equal(ExtensionProperties{
		ConnectionID:    "conn-1",
		InputKeys:       []string{"username", "password"},
		RuntimeDataKeys: []string{"extension.risk_check.riskScore", "extension."},
		ResponseKeys:    []string{"riskScore"},
		MaxAttempts:     3,
	}, props)
}

func (s *ModelTestSuite) TestParseExtensionProperties_UnsetListsAndDefaults() {
	for name, properties := range map[string]map[string]interface{}{
		"absent": {NodePropertyConnectionID: "conn-1"},
		"null": {NodePropertyConnectionID: "conn-1", NodePropertyInputKeys: nil, NodePropertyRuntimeDataKeys: nil,
			NodePropertyResponseKeys: nil, NodePropertyMaxAttempts: nil},
	} {
		s.Run(name, func() {
			props, invalid := ParseExtensionProperties(properties)

			s.Empty(invalid)
			s.Nil(props.InputKeys)
			s.Equal([]string{"userID"}, props.RuntimeDataKeys)
			s.Nil(props.ResponseKeys)
			s.Equal(5, props.MaxAttempts)
		})
	}
}

func (s *ModelTestSuite) TestParseExtensionProperties_RuntimeDataKeysAllowsTheUserIDAndExtensionData() {
	props, invalid := ParseExtensionProperties(map[string]interface{}{
		NodePropertyConnectionID:    "conn-1",
		NodePropertyRuntimeDataKeys: []interface{}{" userID", "extension.risk_check.riskScore "},
	})

	s.Empty(invalid)
	s.Equal([]string{"userID", "extension.risk_check.riskScore"}, props.RuntimeDataKeys)
}

func (s *ModelTestSuite) TestParseExtensionProperties_ResponseKeysAcceptsACommaSeparatedString() {
	props, invalid := ParseExtensionProperties(map[string]interface{}{
		NodePropertyConnectionID: "conn-1", NodePropertyResponseKeys: " riskScore, challengeId ",
	})

	s.Empty(invalid)
	s.Equal([]string{"riskScore", "challengeId"}, props.ResponseKeys)
}

func (s *ModelTestSuite) TestParseExtensionProperties_RefusesUnusableValues() {
	cases := []struct {
		name     string
		property string
		value    interface{}
	}{
		{"connection absent", NodePropertyConnectionID, nil},
		{"connection blank", NodePropertyConnectionID, "  "},
		{"connection not a string", NodePropertyConnectionID, 42},
		{"input keys empty", NodePropertyInputKeys, []interface{}{}},
		{"input keys blank name", NodePropertyInputKeys, []interface{}{"username", " "}},
		{"input keys non-string", NodePropertyInputKeys, []interface{}{"username", 7}},
		{"input keys a string", NodePropertyInputKeys, "username,password"},
		{"runtime data keys empty", NodePropertyRuntimeDataKeys, []interface{}{}},
		{"runtime data keys blank name", NodePropertyRuntimeDataKeys, []interface{}{""}},
		{"runtime data keys non-string", NodePropertyRuntimeDataKeys, []interface{}{true}},
		{"runtime data keys a string", NodePropertyRuntimeDataKeys, "userID"},
		{"runtime data key outside the allow list", NodePropertyRuntimeDataKeys, []interface{}{"userID", "clientId"}},
		{"runtime data key a credential", NodePropertyRuntimeDataKeys, []interface{}{"password"}},
		{"runtime data key an attempt count", NodePropertyRuntimeDataKeys,
			[]interface{}{ExtensionAttemptCountKey("risk_check")}},
		{"runtime data key in another case", NodePropertyRuntimeDataKeys, []interface{}{"userId"}},
		{"response keys empty", NodePropertyResponseKeys, []interface{}{}},
		{"response keys blank name", NodePropertyResponseKeys, []interface{}{"riskScore", "\t"}},
		{"response keys non-string", NodePropertyResponseKeys, []interface{}{map[string]interface{}{}}},
		{"response keys empty string", NodePropertyResponseKeys, ""},
		{"response keys blank in string", NodePropertyResponseKeys, "riskScore,,challengeId"},
		{"response keys a number", NodePropertyResponseKeys, 3},
		{"max attempts zero", NodePropertyMaxAttempts, 0},
		{"max attempts over the bound", NodePropertyMaxAttempts, float64(21)},
		{"max attempts negative", NodePropertyMaxAttempts, -1},
		{"max attempts fractional", NodePropertyMaxAttempts, 2.5},
		{"max attempts a string", NodePropertyMaxAttempts, "5"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			properties := map[string]interface{}{NodePropertyConnectionID: "conn-1"}
			properties[tc.property] = tc.value

			_, invalid := ParseExtensionProperties(properties)

			s.Equal(tc.property, invalid)
		})
	}
}

func (s *ModelTestSuite) TestParseExtensionProperties_MaxAttemptsBounds() {
	for _, value := range []interface{}{1, 20, float64(20), int64(7)} {
		props, invalid := ParseExtensionProperties(map[string]interface{}{
			NodePropertyConnectionID: "conn-1", NodePropertyMaxAttempts: value,
		})

		s.Empty(invalid, "%v", value)
		s.NotZero(props.MaxAttempts)
	}
}
