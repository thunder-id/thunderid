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

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
)

type StateTestSuite struct {
	suite.Suite
}

func TestStateTestSuite(t *testing.T) {
	suite.Run(t, new(StateTestSuite))
}

func (s *StateTestSuite) TestNewTimeouts_Defaults() {
	// Unset and non-positive values fall back to the built-in defaults.
	s.Equal(newTimeouts(Config{}, deploymentLimits{}), newTimeouts(Config{IdleTimeoutSeconds: -5}, deploymentLimits{}))
}

// Each timeout is cut to the deployment's maximum, so a value stored before the operator lowered it applies
// within it, and so does a default longer than the maximum.
func (s *StateTestSuite) TestNewTimeouts_CutToTheDeploymentMaximums() {
	limits := deploymentLimits{maxIdle: time.Hour, maxAbsolute: 12 * time.Hour}
	got := newTimeouts(Config{IdleTimeoutSeconds: 7200, AbsoluteTimeoutSeconds: 86400}, limits)
	s.Equal(time.Hour, got.Idle)
	s.Equal(12*time.Hour, got.Absolute)

	got = newTimeouts(Config{}, deploymentLimits{maxAbsolute: time.Hour})
	s.Equal(time.Hour, got.Absolute)
	s.Equal(time.Hour, got.Idle, "the default idle timeout is cut to the absolute one")
}

// A limit the deployment leaves unset takes the built-in default: 60 seconds between activity refreshes,
// at most 30 days idle and at most 365 days absolute.
func (s *StateTestSuite) TestNewDeploymentLimits_Defaults() {
	s.Equal(deploymentLimits{
		activityRefresh: time.Minute,
		maxIdle:         30 * 24 * time.Hour,
		maxAbsolute:     365 * 24 * time.Hour,
	}, newDeploymentLimits(flowconfig.Config{}))
	s.Equal(deploymentLimits{activityRefresh: 30 * time.Second, maxIdle: time.Hour, maxAbsolute: 8 * time.Hour},
		newDeploymentLimits(flowconfig.Config{
			SessionActivityRefresh:    30 * time.Second,
			SessionMaxIdleTimeout:     time.Hour,
			SessionMaxAbsoluteTimeout: 8 * time.Hour,
		}))
}

func (s *StateTestSuite) TestNewTimeouts_Overrides() {
	got := newTimeouts(Config{IdleTimeoutSeconds: 120, AbsoluteTimeoutSeconds: 600}, deploymentLimits{})
	s.Equal(120*time.Second, got.Idle)
	s.Equal(600*time.Second, got.Absolute)
}

func (s *StateTestSuite) TestNewTimeouts_PartialOverride() {
	got := newTimeouts(Config{IdleTimeoutSeconds: 120}, deploymentLimits{})
	s.Equal(120*time.Second, got.Idle)
	s.Equal(defaultAbsoluteTimeout, got.Absolute, "unset absolute falls back to default")
}

func (s *StateTestSuite) TestNewTimeouts_IdleClampedToAbsolute() {
	// A small configured absolute with a defaulted (larger) idle must not yield idle > absolute.
	got := newTimeouts(Config{AbsoluteTimeoutSeconds: 600}, deploymentLimits{})
	s.Equal(600*time.Second, got.Absolute)
	s.Equal(600*time.Second, got.Idle, "idle is clamped to the absolute lifetime")

	// An explicit idle larger than absolute is likewise clamped.
	got = newTimeouts(Config{IdleTimeoutSeconds: 900, AbsoluteTimeoutSeconds: 600}, deploymentLimits{})
	s.Equal(600*time.Second, got.Idle)
	s.Equal(600*time.Second, got.Absolute)
}

func (s *StateTestSuite) TestNewTimeouts_ActivityRefreshInterval() {
	// An unset interval yields the built-in default.
	s.Equal(defaultActivityRefreshInterval, newTimeouts(Config{}, deploymentLimits{}).ActivityRefresh)

	// A configured interval below the idle window is honored exactly.
	timeouts := Config{IdleTimeoutSeconds: 1800, AbsoluteTimeoutSeconds: 28800}
	refreshEvery := func(d time.Duration) time.Duration {
		return newTimeouts(timeouts, deploymentLimits{activityRefresh: d}).ActivityRefresh
	}
	s.Equal(20*time.Second, refreshEvery(20*time.Second))
	s.Equal(1200*time.Second, refreshEvery(1200*time.Second),
		"an interval above half the idle window is honored, not clamped")
}

// An interval that reaches the idle window, as when the deployment raised it after the section was set,
// is kept below it so an active session is never skipped past its idle deadline.
func (s *StateTestSuite) TestNewTimeouts_ActivityRefreshKeptBelowIdle() {
	refreshEvery := func(cfg Config, d time.Duration) time.Duration {
		return newTimeouts(cfg, deploymentLimits{activityRefresh: d}).ActivityRefresh
	}
	s.Equal(60*time.Second, refreshEvery(Config{IdleTimeoutSeconds: 120}, 5*time.Minute))
	s.Equal(30*time.Second, refreshEvery(Config{IdleTimeoutSeconds: 60}, 0))
	s.Equal(50*time.Second, refreshEvery(Config{AbsoluteTimeoutSeconds: 100}, 2*time.Minute),
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
