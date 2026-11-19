// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/serverconfigmock"
)

type LiveTimeoutsTestSuite struct {
	suite.Suite
}

func TestLiveTimeoutsTestSuite(t *testing.T) {
	suite.Run(t, new(LiveTimeoutsTestSuite))
}

// A change to the section applies on the next use, with no restart.
func (s *LiveTimeoutsTestSuite) TestSectionTimeouts_ReadsTheSectionOnEveryUse() {
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
func (s *LiveTimeoutsTestSuite) TestSectionTimeouts_UsesTheDefaultsWhenTheSectionCannotBeRead() {
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
func (s *LiveTimeoutsTestSuite) TestSectionTimeouts_KeepsTheTimeoutsLastReadWhenAReadFails() {
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
func (s *LiveTimeoutsTestSuite) TestSectionTimeouts_AnEarlierReadFinishingLastDoesNotReplaceALaterOne() {
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

	earlier := make(chan resolvedTimeouts)
	go func() { earlier <- source.current(context.Background()) }()
	<-started
	s.Equal(10*time.Minute, source.current(context.Background()).Idle)
	close(release)
	s.Equal(2*time.Hour, (<-earlier).Idle, "the earlier read answers its own caller")

	s.Equal(10*time.Minute, source.current(context.Background()).Idle,
		"a failed read must fall back to the later read")
}
