/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package session

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type StateTestSuite struct {
	suite.Suite
}

func TestStateTestSuite(t *testing.T) {
	suite.Run(t, new(StateTestSuite))
}

func (s *StateTestSuite) TestNewTimeouts_Defaults() {
	// Non-positive values fall back to the built-in defaults.
	got := NewTimeouts(-5, 0)
	s.Equal(DefaultTimeouts(), got)
}

func (s *StateTestSuite) TestNewTimeouts_Overrides() {
	got := NewTimeouts(60, 600)
	s.Equal(60*time.Second, got.Idle)
	s.Equal(600*time.Second, got.Absolute)
}

func (s *StateTestSuite) TestNewTimeouts_PartialOverride() {
	got := NewTimeouts(60, 0)
	s.Equal(60*time.Second, got.Idle)
	s.Equal(DefaultAbsoluteTimeout, got.Absolute, "unset absolute falls back to default")
}

func (s *StateTestSuite) TestNewTimeouts_IdleClampedToAbsolute() {
	// A small configured absolute with a defaulted (larger) idle must not yield idle > absolute.
	got := NewTimeouts(0, 60)
	s.Equal(60*time.Second, got.Absolute)
	s.Equal(60*time.Second, got.Idle, "idle is clamped to the absolute lifetime")

	// An explicit idle larger than absolute is likewise clamped.
	got = NewTimeouts(600, 300)
	s.Equal(300*time.Second, got.Idle)
	s.Equal(300*time.Second, got.Absolute)
}

func (s *StateTestSuite) TestSessionContext_PayloadRoundTrip() {
	c := SessionContext{
		SessionID:      "sess-1",
		RuntimeData:    map[string]string{"email": "a@b.com", "department": "eng"},
		AuthUser:       json.RawMessage(`{"entityReference":{"entityId":"user-1"}}`),
		CompletedSteps: map[string]StepFact{"basic_auth": {Executor: "BasicAuthExecutor", Status: "COMPLETE"}},
		ContextVersion: 1,
	}

	raw, err := c.serializePayload()
	s.Require().NoError(err)

	parsed, err := parseSessionContextPayload(raw)
	s.Require().NoError(err)
	s.Equal(c.RuntimeData, parsed.RuntimeData)
	s.JSONEq(string(c.AuthUser), string(parsed.AuthUser))
	s.Equal(c.CompletedSteps, parsed.CompletedSteps)
}

func (s *StateTestSuite) TestParseSessionContextPayload_Invalid() {
	_, err := parseSessionContextPayload("{not json")
	s.Error(err)
}

// TestSessionRowCarriesNoTransientExecutionState guards the lean, hot-path SESSION row: transient
// per-execution state (current node, partial inputs, runtime data, execution history, challenge
// token) must never land on it, so liveness/activity checks never pull execution state.
//
// The session-context sibling intentionally snapshots runtime state (RuntimeData, the resolved
// AuthUser, completed steps) so the SSO join can replay the effective attribute set — so this
// guard deliberately does NOT cover SessionContext.
func (s *StateTestSuite) TestSessionRowCarriesNoTransientExecutionState() {
	forbidden := []string{
		"runtimedata",
		"userinput",
		"currentnode",
		"currentaction",
		"currentsegment",
		"executionhistory",
		"challengetoken",
		"forwardeddata",
		"partialinput",
		"flowstate",
	}

	types := []reflect.Type{
		reflect.TypeOf(Session{}),
	}

	for _, typ := range types {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name)
			for _, f := range forbidden {
				s.NotContains(name, f,
					"%s.%s looks like transient execution state; it belongs in the flow store, not the session",
					typ.Name(), typ.Field(i).Name)
			}
		}
	}
}
