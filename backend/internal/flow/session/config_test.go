// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

type ConfigTestSuite struct {
	suite.Suite
	handler serverconfig.ServerConfigHandlerInterface
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) SetupTest() {
	s.handler = NewConfigHandler(flowconfig.Config{})
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

// An idle timeout is compared with an absolute one only when both are set. Otherwise the idle timeout in
// effect is cut to the absolute one, so an absolute timeout shorter than the default idle timeout is
// accepted on its own.
func (s *ConfigTestSuite) TestValidate_IdleIsComparedWithAnAbsoluteSetWithIt() {
	s.Require().NoError(s.validate(Config{AbsoluteTimeoutSeconds: 3600}))
	s.Require().NoError(s.validate(Config{IdleTimeoutSeconds: 28801}))
	s.Require().NoError(s.validate(Config{IdleTimeoutSeconds: 1800, AbsoluteTimeoutSeconds: 3600}))
}

// The idle timeout in effect is cut to the absolute timeout, so an absolute timeout no longer than the
// activity refresh interval is rejected even when the idle timeout is left unset.
func (s *ConfigTestSuite) TestValidate_AbsoluteCutsTheIdleTimeoutInEffect() {
	s.Require().Error(s.validate(Config{AbsoluteTimeoutSeconds: 60}))
	s.Require().NoError(s.validate(Config{AbsoluteTimeoutSeconds: 61}))
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

	handler := NewConfigHandler(flowconfig.Config{SessionActivityRefresh: 10 * time.Minute})
	s.Require().Error(handler.Validate(context.Background(), Config{IdleTimeoutSeconds: 600}, nil, nil))
	s.Require().NoError(handler.Validate(context.Background(), Config{IdleTimeoutSeconds: 601}, nil, nil))
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

// The activity refresh interval moved to the deployment configuration, so a value stored with it still
// decodes.
func (s *ConfigTestSuite) TestHandler_DecodeIgnoresTheActivityRefreshInterval() {
	got, err := s.handler.Decode(json.RawMessage(`{"idleTimeoutSeconds":900,"activityRefreshIntervalSeconds":30}`))
	s.Require().NoError(err)
	s.Equal(Config{IdleTimeoutSeconds: 900}, got)
}

// A declarative document is the configured value whole, and the writable layer is not used under it.
func (s *ConfigTestSuite) TestHandler_MergeDeclarativeWins() {
	readOnly := Config{IdleTimeoutSeconds: 1800}
	writable := Config{IdleTimeoutSeconds: 600, AbsoluteTimeoutSeconds: 3600}

	s.Equal(readOnly, s.handler.Merge(readOnly, writable))
	s.Equal(writable, s.handler.Merge(Config{}, writable))
	s.Equal(Config{}, s.handler.Merge(nil, nil))
}

func (s *ConfigTestSuite) TestHandler_IsLocked() {
	s.True(s.handler.IsLocked(Config{AbsoluteTimeoutSeconds: 3600}))
	s.False(s.handler.IsLocked(Config{}))
	s.False(s.handler.IsLocked(nil))
}

func (s *ConfigTestSuite) TestHandler_DefaultIsTheBuiltInTimeouts() {
	s.Equal(Config{IdleTimeoutSeconds: 7200, AbsoluteTimeoutSeconds: 28800}, s.handler.Default())
}

// A change to the section applies on the next use, with no restart.
func (s *ConfigTestSuite) TestSectionTimeouts_ReadsTheSectionOnEveryUse() {
	reader := serverconfigmock.NewServerConfigServiceMock(s.T())
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).
		Return(Config{IdleTimeoutSeconds: 900, AbsoluteTimeoutSeconds: 3600}, nil).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).
		Return(Config{IdleTimeoutSeconds: 1200, AbsoluteTimeoutSeconds: 3600}, nil).Once()
	source := newSectionTimeouts(reader, time.Minute)

	first := source.current(context.Background())
	s.Equal(900*time.Second, first.Idle)
	s.Equal(time.Hour, first.Absolute)
	s.Equal(time.Minute, first.ActivityRefresh)
	s.Equal(1200*time.Second, source.current(context.Background()).Idle)
}

// Before any read succeeds, a failed read takes the defaults.
func (s *ConfigTestSuite) TestSectionTimeouts_UsesTheDefaultsWhenTheSectionCannotBeRead() {
	reader := serverconfigmock.NewServerConfigServiceMock(s.T())
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).
		Return(nil, &tidcommon.InternalServerError).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).
		Return("not a section", nil).Once()
	source := newSectionTimeouts(reader, time.Minute)

	s.Equal(newTimeouts(Config{}, time.Minute), source.current(context.Background()))
	s.Equal(newTimeouts(Config{}, time.Minute), source.current(context.Background()))
}

// A failed read keeps the timeouts last read, so a session started then is never given the longer
// defaults.
func (s *ConfigTestSuite) TestSectionTimeouts_KeepsTheTimeoutsLastReadWhenAReadFails() {
	reader := serverconfigmock.NewServerConfigServiceMock(s.T())
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).
		Return(Config{IdleTimeoutSeconds: 900, AbsoluteTimeoutSeconds: 3600}, nil).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).
		Return(nil, &tidcommon.InternalServerError).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).
		Return("not a section", nil).Once()
	source := newSectionTimeouts(reader, time.Minute)

	read := source.current(context.Background())
	s.Equal(15*time.Minute, read.Idle)
	s.Equal(read, source.current(context.Background()), "a failed read keeps the timeouts last read")
	s.Equal(read, source.current(context.Background()), "an unreadable value keeps the timeouts last read")
}

// Of two overlapping reads, the one that started first finishes last. It answers its own caller, but
// doesn't replace the later read's timeouts, so a failed read after both falls back to the later ones.
func (s *ConfigTestSuite) TestSectionTimeouts_AnEarlierReadFinishingLastDoesNotReplaceALaterOne() {
	reader := serverconfigmock.NewServerConfigServiceMock(s.T())
	started := make(chan struct{})
	release := make(chan struct{})
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).
		RunAndReturn(func(context.Context, string) (any, *tidcommon.ServiceError) {
			close(started)
			<-release
			return Config{IdleTimeoutSeconds: 7200}, nil
		}).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).
		Return(Config{IdleTimeoutSeconds: 600}, nil).Once()
	reader.EXPECT().GetMergedConfig(mock.Anything, string(serverconfig.ConfigNameSession)).
		Return(nil, &tidcommon.InternalServerError).Once()
	source := newSectionTimeouts(reader, time.Minute)

	earlier := make(chan Timeouts)
	go func() { earlier <- source.current(context.Background()) }()
	<-started
	s.Equal(10*time.Minute, source.current(context.Background()).Idle)
	close(release)
	s.Equal(2*time.Hour, (<-earlier).Idle, "the earlier read answers its own caller")

	s.Equal(10*time.Minute, source.current(context.Background()).Idle,
		"a failed read must fall back to the later read")
}
