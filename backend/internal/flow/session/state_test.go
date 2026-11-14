// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	// Unset and non-positive values fall back to the built-in defaults.
	s.Equal(DefaultTimeouts(), newTimeouts(Config{IdleTimeoutSeconds: -5}, 0))
}

func (s *StateTestSuite) TestNewTimeouts_Overrides() {
	got := newTimeouts(Config{IdleTimeoutSeconds: 120, AbsoluteTimeoutSeconds: 600}, 0)
	s.Equal(120*time.Second, got.Idle)
	s.Equal(600*time.Second, got.Absolute)
}

func (s *StateTestSuite) TestNewTimeouts_PartialOverride() {
	got := newTimeouts(Config{IdleTimeoutSeconds: 120}, 0)
	s.Equal(120*time.Second, got.Idle)
	s.Equal(DefaultAbsoluteTimeout, got.Absolute, "unset absolute falls back to default")
}

func (s *StateTestSuite) TestNewTimeouts_IdleClampedToAbsolute() {
	// A small configured absolute with a defaulted (larger) idle must not yield idle > absolute.
	got := newTimeouts(Config{AbsoluteTimeoutSeconds: 600}, 0)
	s.Equal(600*time.Second, got.Absolute)
	s.Equal(600*time.Second, got.Idle, "idle is clamped to the absolute lifetime")

	// An explicit idle larger than absolute is likewise clamped.
	got = newTimeouts(Config{IdleTimeoutSeconds: 900, AbsoluteTimeoutSeconds: 600}, 0)
	s.Equal(600*time.Second, got.Idle)
	s.Equal(600*time.Second, got.Absolute)
}

func (s *StateTestSuite) TestNewTimeouts_ActivityRefreshInterval() {
	// An unset interval yields the built-in default.
	s.Equal(defaultActivityRefreshInterval, newTimeouts(Config{}, 0).ActivityRefresh)

	// A configured interval below the idle window is honored exactly.
	timeouts := Config{IdleTimeoutSeconds: 1800, AbsoluteTimeoutSeconds: 28800}
	s.Equal(20*time.Second, newTimeouts(timeouts, 20*time.Second).ActivityRefresh)
	s.Equal(1200*time.Second, newTimeouts(timeouts, 1200*time.Second).ActivityRefresh,
		"an interval above half the idle window is honored, not clamped")
}

// An interval that reaches the idle window, as when the deployment raised it after the section was set,
// is kept below it so an active session is never skipped past its idle deadline.
func (s *StateTestSuite) TestNewTimeouts_ActivityRefreshKeptBelowIdle() {
	s.Equal(60*time.Second, newTimeouts(Config{IdleTimeoutSeconds: 120}, 5*time.Minute).ActivityRefresh)
	s.Equal(30*time.Second, newTimeouts(Config{IdleTimeoutSeconds: 60}, 0).ActivityRefresh)
	s.Equal(50*time.Second, newTimeouts(Config{AbsoluteTimeoutSeconds: 100}, 2*time.Minute).ActivityRefresh,
		"the idle window is clamped to the absolute lifetime first")
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
