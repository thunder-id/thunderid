// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/serverconfig"
)

type ConfigTestSuite struct {
	suite.Suite
	handler serverconfig.ServerConfigHandlerInterface
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) SetupTest() {
	s.handler = newConfigHandler(flowconfig.Config{})
}

func (s *ConfigTestSuite) TestInitializeConfigHandler() {
	s.IsType(configHandler{}, InitializeConfigHandler(flowconfig.Config{}))
}

func (s *ConfigTestSuite) validate(value any) error {
	return s.handler.Validate(context.Background(), value, nil, nil)
}

func (s *ConfigTestSuite) TestValidate_UnsetUsesDefaults() {
	s.Require().NoError(s.validate(Config{}))
}

func (s *ConfigTestSuite) TestValidate_PositiveValues() {
	s.Require().NoError(s.validate(Config{IdleTimeoutSeconds: 1800, AbsoluteTimeoutSeconds: 28800}))
}

func (s *ConfigTestSuite) TestValidate_NegativeRejected() {
	s.Require().Error(s.validate(Config{IdleTimeoutSeconds: -1}))
	s.Require().Error(s.validate(Config{AbsoluteTimeoutSeconds: -1}))
}

func (s *ConfigTestSuite) TestValidate_IdleExceedsAbsolute() {
	s.Require().Error(s.validate(Config{IdleTimeoutSeconds: 28801, AbsoluteTimeoutSeconds: 28800}))
}

// An idle timeout is compared with the absolute timeout in effect, the one set with it or the default. An
// absolute timeout shorter than the default idle timeout is accepted on its own, since the idle timeout in
// effect is cut to it.
func (s *ConfigTestSuite) TestValidate_IdleIsComparedWithTheAbsoluteInEffect() {
	s.Require().NoError(s.validate(Config{AbsoluteTimeoutSeconds: 3600}))
	s.Require().NoError(s.validate(Config{IdleTimeoutSeconds: 86400}))
	s.Require().Error(s.validate(Config{IdleTimeoutSeconds: 86401}), "the default absolute timeout is 24 hours")
	s.Require().NoError(s.validate(Config{IdleTimeoutSeconds: 1800, AbsoluteTimeoutSeconds: 3600}))
}

// The idle timeout in effect is cut to the absolute timeout, so an absolute timeout no longer than the
// activity refresh interval is rejected even when the idle timeout is left unset.
func (s *ConfigTestSuite) TestValidate_AbsoluteCutsTheIdleTimeoutInEffect() {
	s.Require().Error(s.validate(Config{AbsoluteTimeoutSeconds: 60}))
	s.Require().NoError(s.validate(Config{AbsoluteTimeoutSeconds: 61}))
}

// Each timeout must be no longer than the deployment's maximum for it: 30 days idle and 365 days absolute
// unless deployment.yaml sets others.
func (s *ConfigTestSuite) TestValidate_TimeoutsWithinTheDeploymentMaximums() {
	s.Require().NoError(s.validate(Config{IdleTimeoutSeconds: 2592000, AbsoluteTimeoutSeconds: 31536000}))
	s.Require().Error(s.validate(Config{IdleTimeoutSeconds: 2592001, AbsoluteTimeoutSeconds: 31536000}))
	s.Require().Error(s.validate(Config{AbsoluteTimeoutSeconds: 31536001}))

	handler := newConfigHandler(flowconfig.Config{
		SessionMaxIdleTimeout:     time.Hour,
		SessionMaxAbsoluteTimeout: 8 * time.Hour,
	})
	ctx := context.Background()
	s.Require().NoError(
		handler.Validate(ctx, Config{IdleTimeoutSeconds: 3600, AbsoluteTimeoutSeconds: 28800}, nil, nil))
	s.Require().Error(handler.Validate(ctx, Config{IdleTimeoutSeconds: 3601}, nil, nil))
	s.Require().Error(handler.Validate(ctx, Config{AbsoluteTimeoutSeconds: 28801}, nil, nil))
}

func (s *ConfigTestSuite) TestValidate_OverflowingSecondsRejected() {
	// A second value beyond what a time.Duration can hold would wrap negative; it must be rejected
	// rather than slip past the invariant check.
	s.Require().Error(s.validate(Config{IdleTimeoutSeconds: maxTimeoutSeconds + 1}))
	s.Require().Error(s.validate(Config{AbsoluteTimeoutSeconds: maxTimeoutSeconds + 1}))
}

// An idle timeout must be longer than the deployment's activity refresh interval, or an active session
// could expire between two refreshes.
func (s *ConfigTestSuite) TestValidate_IdleLongerThanTheActivityRefreshInterval() {
	s.Require().Error(s.validate(Config{IdleTimeoutSeconds: 30}))
	s.Require().Error(s.validate(Config{IdleTimeoutSeconds: 60}), "the default interval is 60 seconds")
	s.Require().NoError(s.validate(Config{IdleTimeoutSeconds: 61}))

	handler := newConfigHandler(flowconfig.Config{SessionActivityRefresh: 10 * time.Minute})
	s.Require().Error(handler.Validate(context.Background(), Config{IdleTimeoutSeconds: 600}, nil, nil))
	s.Require().NoError(handler.Validate(context.Background(), Config{IdleTimeoutSeconds: 601}, nil, nil))
}

// A field the section does not have, such as a misspelled one, is refused on a write, while a value
// stored with one still loads.
func (s *ConfigTestSuite) TestValidate_UnknownFieldRejected() {
	got, err := s.handler.Decode(json.RawMessage(`{"idleTimeoutSecond":900,"absoluteTimeoutSeconds":3600}`))
	s.Require().NoError(err)
	s.Equal(int64(3600), got.(Config).AbsoluteTimeoutSeconds)
	s.Require().ErrorContains(s.validate(got), `"idleTimeoutSecond"`)
}

func (s *ConfigTestSuite) TestValidate_AnotherTypeRejected() {
	s.Require().Error(s.validate("not a section"))
}

func (s *ConfigTestSuite) TestHandler_DecodeEmptyIsZero() {
	got, err := s.handler.Decode(nil)
	s.Require().NoError(err)
	s.Equal(Config{}, got)
}

func (s *ConfigTestSuite) TestHandler_DecodeJSON() {
	got, err := s.handler.Decode(json.RawMessage(`{"idleTimeoutSeconds":900,"absoluteTimeoutSeconds":3600}`))
	s.Require().NoError(err)
	s.Equal(Config{IdleTimeoutSeconds: 900, AbsoluteTimeoutSeconds: 3600}, got)
}

// The activity refresh interval moved to the deployment configuration, and an export from earlier versions
// always carries it, so it is accepted and ignored.
func (s *ConfigTestSuite) TestHandler_TheActivityRefreshIntervalIsIgnored() {
	got, err := s.handler.Decode(json.RawMessage(`{"idleTimeoutSeconds":900,"activityRefreshIntervalSeconds":30}`))
	s.Require().NoError(err)
	s.Equal(Config{IdleTimeoutSeconds: 900}, got)
	s.Require().NoError(s.validate(got))
}

// JSON matches a field name without regard to case, so a known field written in another case is not an
// unknown one.
func (s *ConfigTestSuite) TestHandler_DecodeMatchesFieldNamesWithoutRegardToCase() {
	got, err := s.handler.Decode(json.RawMessage(`{"IdleTimeoutSeconds":900}`))
	s.Require().NoError(err)
	s.Equal(Config{IdleTimeoutSeconds: 900}, got)
	s.Require().NoError(s.validate(got))
}

// A declarative document is the configured value whole, and the writable layer is not used under it.
func (s *ConfigTestSuite) TestHandler_MergeDeclarativeWins() {
	readOnly := Config{IdleTimeoutSeconds: 1800}
	writable := Config{IdleTimeoutSeconds: 600, AbsoluteTimeoutSeconds: 3600}

	s.Equal(readOnly, s.handler.Merge(readOnly, writable))
	s.Equal(writable, s.handler.Merge(Config{}, writable))
	s.Equal(Config{}, s.handler.Merge(nil, nil))
}

func (s *ConfigTestSuite) TestHandler_IsReadOnly() {
	s.True(s.handler.IsReadOnly(Config{AbsoluteTimeoutSeconds: 3600}))
	s.False(s.handler.IsReadOnly(Config{}))
	s.False(s.handler.IsReadOnly(nil))
}

func (s *ConfigTestSuite) TestHandler_DefaultIsTheBuiltInTimeouts() {
	s.Equal(Config{IdleTimeoutSeconds: 7200, AbsoluteTimeoutSeconds: 86400}, s.handler.Default())
}

// A default longer than the deployment allows can't apply, so it is cut to the maximum.
func (s *ConfigTestSuite) TestHandler_DefaultIsCutToTheDeploymentMaximums() {
	handler := newConfigHandler(flowconfig.Config{
		SessionMaxIdleTimeout:     time.Hour,
		SessionMaxAbsoluteTimeout: 8 * time.Hour,
	})
	s.Equal(Config{IdleTimeoutSeconds: 3600, AbsoluteTimeoutSeconds: 28800}, handler.Default())
}
