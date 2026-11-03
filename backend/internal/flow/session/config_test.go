// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigTestSuite struct {
	suite.Suite
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) TestValidate_UnsetUsesDefaults() {
	s.Require().NoError(Config{}.Validate())
}

func (s *ConfigTestSuite) TestValidate_PositiveValues() {
	s.Require().NoError(Config{IdleTimeoutSeconds: 1800, AbsoluteTimeoutSeconds: 28800}.Validate())
}

func (s *ConfigTestSuite) TestValidate_NegativeRejected() {
	s.Require().Error(Config{IdleTimeoutSeconds: -1}.Validate())
	s.Require().Error(Config{AbsoluteTimeoutSeconds: -1}.Validate())
}

func (s *ConfigTestSuite) TestValidate_IdleExceedsAbsolute() {
	s.Require().Error(Config{IdleTimeoutSeconds: 28801, AbsoluteTimeoutSeconds: 28800}.Validate())
}

func (s *ConfigTestSuite) TestValidate_OverflowingSecondsRejected() {
	// A second value beyond what a time.Duration can hold would wrap negative; it must be rejected
	// rather than slip past the invariant check.
	s.Require().Error(Config{IdleTimeoutSeconds: maxTimeoutSeconds + 1}.Validate())
	s.Require().Error(Config{AbsoluteTimeoutSeconds: maxTimeoutSeconds + 1}.Validate())
	s.Require().Error(Config{ActivityRefreshIntervalSeconds: maxTimeoutSeconds + 1}.Validate())
}

func (s *ConfigTestSuite) TestValidate_ActivityRefreshInterval() {
	// An activity-refresh interval below the idle window is accepted.
	s.Require().NoError(Config{IdleTimeoutSeconds: 1800, ActivityRefreshIntervalSeconds: 60}.Validate())
	// A negative activity-refresh interval is rejected.
	s.Require().Error(Config{ActivityRefreshIntervalSeconds: -1}.Validate())
	// An activity-refresh interval equal to the idle window is rejected (must be strictly less).
	s.Require().Error(Config{IdleTimeoutSeconds: 60, ActivityRefreshIntervalSeconds: 60}.Validate())
	// An activity-refresh interval exceeding the idle window is rejected.
	s.Require().Error(Config{IdleTimeoutSeconds: 60, ActivityRefreshIntervalSeconds: 120}.Validate())
}

func (s *ConfigTestSuite) TestValidate_ActivityRefreshInvariantAcrossDefaults() {
	// A zero means "use the default", so the invariant is checked against the resolved durations.

	// Gap A: a small configured idle with a defaulted (60s) refresh must be rejected, even though the
	// refresh field itself is left unset.
	s.Require().Error(Config{IdleTimeoutSeconds: 30}.Validate())
	// One second above the default refresh is accepted.
	s.Require().NoError(Config{IdleTimeoutSeconds: 61}.Validate())

	// Gap B: a defaulted (1800s) idle with a large configured refresh must be rejected, even though
	// the idle field itself is left unset.
	s.Require().Error(Config{ActivityRefreshIntervalSeconds: 3600}.Validate())
	// A large refresh below the defaulted idle window is accepted.
	s.Require().NoError(Config{ActivityRefreshIntervalSeconds: 1799}.Validate())
}

func (s *ConfigTestSuite) TestHandler_DecodeEmptyIsZero() {
	got, err := ConfigHandler{}.Decode(nil)
	s.Require().NoError(err)
	s.Equal(Config{}, got)
}

func (s *ConfigTestSuite) TestHandler_DecodeJSON() {
	got, err := ConfigHandler{}.Decode(json.RawMessage(`{"idleTimeoutSeconds":900,"absoluteTimeoutSeconds":3600}`))
	s.Require().NoError(err)
	s.Equal(Config{IdleTimeoutSeconds: 900, AbsoluteTimeoutSeconds: 3600}, got)
}

func (s *ConfigTestSuite) TestHandler_ValidateRejectsIncoherent() {
	s.Require().Error(ConfigHandler{}.Validate(context.Background(), Config{IdleTimeoutSeconds: -5}, nil, nil))
}

func (s *ConfigTestSuite) TestHandler_MergeWritableWins() {
	readOnly := Config{IdleTimeoutSeconds: 1800, AbsoluteTimeoutSeconds: 28800, ActivityRefreshIntervalSeconds: 60}
	writable := Config{IdleTimeoutSeconds: 600, ActivityRefreshIntervalSeconds: 30}
	merged := ConfigHandler{}.Merge(readOnly, writable).(Config)
	// A positive writable field overrides read-only; an unset writable field keeps read-only.
	s.Equal(int64(600), merged.IdleTimeoutSeconds)
	s.Equal(int64(28800), merged.AbsoluteTimeoutSeconds)
	s.Equal(int64(30), merged.ActivityRefreshIntervalSeconds)
}
