// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

// AuthUser tests live in auth_user_test.go — the multi-provider AuthUser API
// (ProviderNames / StateFor / SetStateFor) is exercised there.

// ----- NodeDefinition YAML -----

func (suite *ModelTestSuite) TestNodeDefinition_MarshalYAML_NoMeta() {
	nd := NodeDefinition{ID: "start", Type: "START"}
	out, err := yaml.Marshal(&nd)
	suite.Require().NoError(err)
	assert.Contains(suite.T(), string(out), "start")
}

func (suite *ModelTestSuite) TestNodeDefinition_YAML_RoundTrip_WithMeta() {
	meta := map[string]interface{}{"key": "value", "count": float64(3)}
	nd := NodeDefinition{ID: "node1", Type: "PROMPT", Meta: meta}

	out, err := yaml.Marshal(&nd)
	suite.Require().NoError(err)

	var restored NodeDefinition
	suite.Require().NoError(yaml.Unmarshal(out, &restored))

	assert.Equal(suite.T(), nd.ID, restored.ID)
	assert.Equal(suite.T(), nd.Type, restored.Type)
	restoredMeta, ok := restored.Meta.(map[string]interface{})
	suite.Require().True(ok)
	assert.Equal(suite.T(), "value", restoredMeta["key"])
}

func (suite *ModelTestSuite) TestNodeDefinition_UnmarshalYAML_InvalidMetaJSON() {
	raw := `id: node1
type: PROMPT
meta: "not-valid-json{{"`
	var nd NodeDefinition
	suite.Require().NoError(yaml.Unmarshal([]byte(raw), &nd))
	assert.Equal(suite.T(), "node1", nd.ID)
}

// ----- GetDuration -----

func (suite *ModelTestSuite) TestNodeExecutionRecord_GetDuration() {
	suite.T().Run("zero times returns 0", func(t *testing.T) {
		assert.Equal(t, int64(0), (&NodeExecutionRecord{}).GetDuration())
	})

	suite.T().Run("only start time returns 0", func(t *testing.T) {
		assert.Equal(t, int64(0), (&NodeExecutionRecord{StartTime: 1000}).GetDuration())
	})

	suite.T().Run("calculates duration in ms", func(t *testing.T) {
		r := &NodeExecutionRecord{StartTime: 1000, EndTime: 1002}
		assert.Equal(t, int64(2000), r.GetDuration())
	})
}

func (suite *ModelTestSuite) TestExecutionAttempt_GetDuration() {
	suite.T().Run("zero times returns 0", func(t *testing.T) {
		assert.Equal(t, int64(0), (&ExecutionAttempt{}).GetDuration())
	})

	suite.T().Run("calculates duration in ms", func(t *testing.T) {
		e := &ExecutionAttempt{StartTime: 500, EndTime: 503}
		assert.Equal(t, int64(3000), e.GetDuration())
	})
}

// ----- Input.IsSensitive -----

func (suite *ModelTestSuite) TestInput_IsSensitive() {
	sensitive := []string{InputTypePassword, InputTypeOTP}
	for _, typ := range sensitive {
		assert.True(suite.T(), Input{Type: typ}.IsSensitive(), "expected %q to be sensitive", typ)
	}

	notSensitive := []string{InputTypeText, InputTypeEmail, InputTypePhone, InputTypeHidden, InputTypeSelect}
	for _, typ := range notSensitive {
		assert.False(suite.T(), Input{Type: typ}.IsSensitive(), "expected %q to not be sensitive", typ)
	}
}

// ----- Event -----

func (suite *ModelTestSuite) TestEvent_WithStatus() {
	evt := &Event{}
	assert.Same(suite.T(), evt, evt.WithStatus("success"))
	assert.Equal(suite.T(), "success", evt.Status)
}

func (suite *ModelTestSuite) TestEvent_WithData() {
	evt := &Event{}
	assert.Same(suite.T(), evt, evt.WithData("user_id", "u-1"))
	assert.Equal(suite.T(), "u-1", evt.Data["user_id"])

	evt.WithData("client_id", "c-1")
	assert.Equal(suite.T(), "c-1", evt.Data["client_id"])
}

func (suite *ModelTestSuite) TestEvent_WithDataMap() {
	evt := &Event{Data: map[string]interface{}{"existing": true}}
	evt.WithDataMap(map[string]interface{}{
		"user_id":  "u-1",
		"existing": false,
	})
	assert.Equal(suite.T(), false, evt.Data["existing"])
	assert.Equal(suite.T(), "u-1", evt.Data["user_id"])
}

func (suite *ModelTestSuite) TestEvent_Validate() {
	now := time.Now()

	suite.T().Run("nil event fails", func(t *testing.T) {
		var evt *Event
		assert.ErrorContains(t, evt.Validate(), "event is nil")
	})

	suite.T().Run("valid event passes", func(t *testing.T) {
		evt := &Event{
			TraceID:   "trace-1",
			EventID:   "event-1",
			Type:      "user.login",
			Component: "auth",
			Timestamp: now,
		}
		assert.NoError(t, evt.Validate())
	})

	suite.T().Run("missing required fields fail", func(t *testing.T) {
		base := Event{
			TraceID:   "trace-1",
			EventID:   "event-1",
			Type:      "user.login",
			Component: "auth",
			Timestamp: now,
		}

		cases := []struct {
			name    string
			mutate  func(*Event)
			contain string
		}{
			{"trace_id", func(e *Event) { e.TraceID = "" }, "trace_id"},
			{"event_id", func(e *Event) { e.EventID = "" }, "event_id"},
			{"type", func(e *Event) { e.Type = "" }, "type"},
			{"component", func(e *Event) { e.Component = "" }, "component"},
			{"timestamp", func(e *Event) { e.Timestamp = time.Time{} }, "timestamp"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				evt := base
				tc.mutate(&evt)
				assert.ErrorContains(t, evt.Validate(), tc.contain)
			})
		}
	})
}

// ----- NodeContext consumed inputs -----

func (suite *ModelTestSuite) TestNodeContext_ConsumeInput_RecordsAndReturnsValue() {
	nc := &NodeContext{UserInputs: map[string]string{"captcha": "abc"}}

	v, ok := nc.ConsumeInput("captcha")

	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "abc", v)
	assert.Equal(suite.T(), []string{"captcha"}, nc.GetConsumedInputs())
}

func (suite *ModelTestSuite) TestNodeContext_ConsumeInput_MissingKeyDoesNotRecord() {
	nc := &NodeContext{UserInputs: map[string]string{"other": "x"}}

	v, ok := nc.ConsumeInput("captcha")

	assert.False(suite.T(), ok)
	assert.Equal(suite.T(), "", v)
	assert.Empty(suite.T(), nc.GetConsumedInputs())
}

func (suite *ModelTestSuite) TestNodeContext_AppendConsumedInputs_AppendsWithoutReading() {
	nc := &NodeContext{UserInputs: map[string]string{"a": "1"}}

	nc.AppendConsumedInputs([]string{"a", "b"})

	assert.Equal(suite.T(), []string{"a", "b"}, nc.GetConsumedInputs())
	assert.Equal(suite.T(), "1", nc.UserInputs["a"], "AppendConsumedInputs must not mutate UserInputs")
}

func (suite *ModelTestSuite) TestNodeContext_AppendConsumedInputs_EmptyIsNoop() {
	nc := &NodeContext{}

	nc.AppendConsumedInputs(nil)
	nc.AppendConsumedInputs([]string{})

	assert.Empty(suite.T(), nc.GetConsumedInputs())
}

func (suite *ModelTestSuite) TestNodeContext_ConsumeInput_AccumulatesAcrossCalls() {
	nc := &NodeContext{UserInputs: map[string]string{"a": "1", "b": "2"}}

	nc.ConsumeInput("a")
	nc.AppendConsumedInputs([]string{"c"})
	nc.ConsumeInput("b")

	assert.Equal(suite.T(), []string{"a", "c", "b"}, nc.GetConsumedInputs())
}

// ----- NodeContext initiator request -----

func (suite *ModelTestSuite) TestNodeContext_GetInitiatorRequest_NilByDefault() {
	nc := &NodeContext{}

	assert.Nil(suite.T(), nc.GetInitiatorRequest())
}

func (suite *ModelTestSuite) TestNodeContext_SetAndGetInitiatorRequest() {
	nc := &NodeContext{}
	req := &InitiatorRequest{
		Headers:     map[string][]string{"X-Custom": {"val"}},
		QueryParams: map[string][]string{"client_id": {"my-client"}},
	}

	nc.SetInitiatorRequest(req)

	got := nc.GetInitiatorRequest()
	assert.Equal(suite.T(), req, got)
	assert.Equal(suite.T(), []string{"val"}, got.Headers["X-Custom"])
	assert.Equal(suite.T(), []string{"my-client"}, got.QueryParams["client_id"])
}

func (suite *ModelTestSuite) TestNodeContext_SetInitiatorRequest_Nil() {
	nc := &NodeContext{}
	nc.SetInitiatorRequest(&InitiatorRequest{})

	nc.SetInitiatorRequest(nil)

	assert.Nil(suite.T(), nc.GetInitiatorRequest())
}

// ----- AttestationConfig -----

func (suite *ModelTestSuite) TestAttestationConfig_WithoutCredentials_NilReceiver() {
	var cfg *AttestationConfig
	assert.Nil(suite.T(), cfg.WithoutCredentials())
}

func (suite *ModelTestSuite) TestAttestationConfig_WithoutCredentials_StripsAndroidSecret() {
	cfg := &AttestationConfig{
		Android: &AndroidAttestationConfig{
			PackageName:               "com.example.app",
			CertificateSha256Digests:  []string{"AA:BB"},
			ServiceAccountCredentials: "secret",
		},
	}

	sanitized := cfg.WithoutCredentials()

	assert.Equal(suite.T(), "com.example.app", sanitized.Android.PackageName)
	assert.Equal(suite.T(), []string{"AA:BB"}, sanitized.Android.CertificateSha256Digests)
	assert.Empty(suite.T(), sanitized.Android.ServiceAccountCredentials)
}

func (suite *ModelTestSuite) TestAttestationConfig_WithoutCredentials_PassesApplePassThrough() {
	cfg := &AttestationConfig{
		Apple: &AppleAttestationConfig{TeamID: "TEAM123", BundleID: "com.example.app"},
	}

	sanitized := cfg.WithoutCredentials()

	assert.NotNil(suite.T(), sanitized.Apple)
	assert.Equal(suite.T(), "TEAM123", sanitized.Apple.TeamID)
	assert.Equal(suite.T(), "com.example.app", sanitized.Apple.BundleID)
	assert.Nil(suite.T(), sanitized.Android)
}

func (suite *ModelTestSuite) TestAttestationConfig_WithoutCredentials_PassesDevModeThrough() {
	cfg := &AttestationConfig{DevMode: true}

	sanitized := cfg.WithoutCredentials()

	assert.True(suite.T(), sanitized.DevMode)
}

// ----- AuthorizationMapping -----

type AuthorizationMappingModelTestSuite struct {
	suite.Suite
}

func TestAuthorizationMappingModelTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationMappingModelTestSuite))
}

func (s *AuthorizationMappingModelTestSuite) TestUnmarshalJSONAcceptsCurrentRulesShape() {
	raw := `{
		"claim": "level",
		"valueType": "number",
		"values": [
			{"operator": "greater_than", "value": "5", "targets": [{"type": "role", "id": "role-1"}]}
		]
	}`

	var mapping AuthorizationMapping
	s.Require().NoError(json.Unmarshal([]byte(raw), &mapping))

	s.Equal(AuthorizationValueTypeNumber, mapping.ValueType)
	s.Equal([]AuthorizationRule{
		{Operator: AuthorizationOperatorGreaterThan, Value: "5", Targets: []AuthorizationTarget{
			{Type: AuthorizationTargetRole, ID: "role-1"},
		}},
	}, mapping.Values)
}

func (s *AuthorizationMappingModelTestSuite) TestUnmarshalJSONHandlesMissingAndNullValues() {
	var withoutValues AuthorizationMapping
	s.Require().NoError(json.Unmarshal([]byte(`{"claim": "groups"}`), &withoutValues))
	s.Empty(withoutValues.Values)

	var nullValues AuthorizationMapping
	s.Require().NoError(json.Unmarshal([]byte(`{"claim": "groups", "values": null}`), &nullValues))
	s.Empty(nullValues.Values)
}

func (s *AuthorizationMappingModelTestSuite) TestUnmarshalJSONRejectsMalformedValues() {
	var mapping AuthorizationMapping
	s.Error(json.Unmarshal([]byte(`{"claim": "groups", "values": "not-an-object-or-array"}`), &mapping))
}

func (s *AuthorizationMappingModelTestSuite) TestEffectiveValueTypeDefaultsToString() {
	s.Equal(AuthorizationValueTypeString, AuthorizationMapping{}.EffectiveValueType())
	s.Equal(AuthorizationValueTypeNumber,
		AuthorizationMapping{ValueType: AuthorizationValueTypeNumber}.EffectiveValueType())
}

func (s *AuthorizationMappingModelTestSuite) TestAuthorizationOperatorIsValid() {
	s.True(AuthorizationOperatorEquals.IsValid())
	s.True(AuthorizationOperatorGreaterThanOrEqual.IsValid())
	s.False(AuthorizationOperator("not-a-real-operator").IsValid())
}

func (s *AuthorizationMappingModelTestSuite) TestAuthorizationOperatorIsOrdering() {
	s.False(AuthorizationOperatorEquals.IsOrdering())
	s.False(AuthorizationOperatorNotEquals.IsOrdering())
	s.True(AuthorizationOperatorGreaterThan.IsOrdering())
	s.True(AuthorizationOperatorLessThan.IsOrdering())
	s.True(AuthorizationOperatorGreaterThanOrEqual.IsOrdering())
	s.True(AuthorizationOperatorLessThanOrEqual.IsOrdering())
}

func (s *AuthorizationMappingModelTestSuite) TestAuthorizationValueTypeIsValid() {
	s.True(AuthorizationValueTypeString.IsValid())
	s.True(AuthorizationValueTypeNumber.IsValid())
	s.True(AuthorizationValueTypeBoolean.IsValid())
	s.True(AuthorizationValueTypeArray.IsValid())
	s.False(AuthorizationValueType("not-a-real-type").IsValid())
}

func (s *AuthorizationMappingModelTestSuite) TestAuthorizationOperatorIsMembership() {
	s.True(AuthorizationOperatorIncludes.IsMembership())
	s.True(AuthorizationOperatorNotIncludes.IsMembership())
	s.False(AuthorizationOperatorEquals.IsMembership())
	s.False(AuthorizationOperatorNotEquals.IsMembership())
	s.False(AuthorizationOperatorGreaterThan.IsMembership())
}

func (s *AuthorizationMappingModelTestSuite) TestIsMultiValued() {
	s.True(AuthorizationMapping{ValueType: AuthorizationValueTypeArray}.IsMultiValued(),
		"an array value type is always multi-valued")
	s.True(AuthorizationMapping{ValueType: AuthorizationValueTypeString, Delimiter: ","}.IsMultiValued(),
		"a string with a delimiter is multi-valued")
	s.False(AuthorizationMapping{ValueType: AuthorizationValueTypeString}.IsMultiValued(),
		"a string with no delimiter is a single value")
	s.False(AuthorizationMapping{}.IsMultiValued(),
		"the default (unset) value type is string with no delimiter, a single value")
	s.False(AuthorizationMapping{ValueType: AuthorizationValueTypeNumber}.IsMultiValued(),
		"a number is always a single value, delimiter is not applicable to it")
	s.False(AuthorizationMapping{ValueType: AuthorizationValueTypeBoolean}.IsMultiValued(),
		"a boolean is always a single value, delimiter is not applicable to it")
}
