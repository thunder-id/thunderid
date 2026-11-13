// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type ResolverTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	resolver       Resolver
	now            time.Time
}

func TestResolverTestSuite(t *testing.T) {
	suite.Run(t, new(ResolverTestSuite))
}

func (s *ResolverTestSuite) SetupTest() {
	s.mockDBProvider = &providermock.DBProviderInterfaceMock{}
	s.mockDBClient = &providermock.DBClientInterfaceMock{}
	s.now = time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	s.withTimeouts(DefaultTimeouts())
}

// withTimeouts builds the suite's resolver over timeouts as the ones in effect.
func (s *ResolverTestSuite) withTimeouts(timeouts Timeouts) {
	source := newTimeoutsSourceMock(s.T())
	source.EXPECT().current(mock.Anything).Return(timeouts).Maybe()
	s.resolver = newResolver(newStore(s.mockDBProvider, testDeploymentID), source)
}

// row builds a live session result row with the given state; idle/absolute deadlines default to
// open (nil) and are set per-test to exercise expiry.
func (s *ResolverTestSuite) row(state string) map[string]interface{} {
	return map[string]interface{}{
		"session_id":          "sess-1",
		"subject_id":          "user-1",
		"flow_id":             "flow-1",
		"flow_version":        int64(1),
		"flow_execution_id":   "exec-1",
		"handle_id":           "handle-abc",
		"authenticated_at":    s.now.Add(-time.Minute),
		"created_at":          s.now.Add(-time.Minute),
		"last_active_at":      s.now.Add(-time.Minute),
		"idle_expires_at":     nil,
		"absolute_expires_at": nil,
		"state":               state,
		"version":             int64(1),
	}
}

func (s *ResolverTestSuite) expectQuery(rows []map[string]interface{}, err error) {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionByHandle,
		"handle-abc", testDeploymentID).Return(rows, err)
}

func (s *ResolverTestSuite) TestResolve_Hit() {
	s.expectQuery([]map[string]interface{}{s.row("ACTIVE")}, nil)

	got, err := s.resolver.Resolve(context.Background(), "handle-abc", s.now)

	s.NoError(err)
	s.Require().NotNil(got)
	s.Equal("sess-1", got.SessionID)
	s.Equal("flow-1", got.FlowID)
	// The resolve hot path reads SESSION only — it must never load the session context.
	s.mockDBClient.AssertNotCalled(s.T(), "QueryContext", mock.Anything,
		queryGetSessionContextByCheckpoint, mock.Anything)
}

func (s *ResolverTestSuite) TestResolve_EmptyHandle() {
	got, err := s.resolver.Resolve(context.Background(), "", s.now)

	s.NoError(err)
	s.Nil(got)
	s.mockDBProvider.AssertNotCalled(s.T(), "GetRuntimePersistentDBClient")
}

func (s *ResolverTestSuite) TestResolve_AbsentNoRow() {
	s.expectQuery([]map[string]interface{}{}, nil)

	got, err := s.resolver.Resolve(context.Background(), "handle-abc", s.now)

	s.NoError(err)
	s.Nil(got)
}

func (s *ResolverTestSuite) TestResolve_Ended() {
	s.expectQuery([]map[string]interface{}{s.row("ENDED")}, nil)

	got, err := s.resolver.Resolve(context.Background(), "handle-abc", s.now)

	s.NoError(err)
	s.Nil(got)
}

func (s *ResolverTestSuite) TestResolve_IdleExpired() {
	r := s.row("ACTIVE")
	r["idle_expires_at"] = s.now.Add(-time.Minute)
	s.expectQuery([]map[string]interface{}{r}, nil)

	got, err := s.resolver.Resolve(context.Background(), "handle-abc", s.now)

	s.NoError(err)
	s.Nil(got, "a session past its idle deadline must not resolve")
}

func (s *ResolverTestSuite) TestResolve_AbsoluteExpired() {
	r := s.row("ACTIVE")
	r["absolute_expires_at"] = s.now.Add(-time.Minute)
	s.expectQuery([]map[string]interface{}{r}, nil)

	got, err := s.resolver.Resolve(context.Background(), "handle-abc", s.now)

	s.NoError(err)
	s.Nil(got, "a session past its absolute deadline must not resolve")
}

func (s *ResolverTestSuite) TestResolve_StoreError() {
	s.expectQuery(nil, errors.New("db down"))

	got, err := s.resolver.Resolve(context.Background(), "handle-abc", s.now)

	s.Error(err)
	s.Nil(got)
}

// A shorter absolute timeout ends an existing session older than it, though its stored deadline has
// not passed.
func (s *ResolverTestSuite) TestResolve_ShorterAbsoluteTimeoutEndsAnExistingSession() {
	r := s.row("ACTIVE")
	r["created_at"] = s.now.Add(-2 * time.Hour)
	r["absolute_expires_at"] = s.now.Add(6 * time.Hour)
	s.expectQuery([]map[string]interface{}{r}, nil)
	timeouts := DefaultTimeouts()
	timeouts.Absolute = time.Hour
	s.withTimeouts(timeouts)

	got, err := s.resolver.Resolve(context.Background(), "handle-abc", s.now)

	s.NoError(err)
	s.Nil(got, "a session older than the current absolute timeout must not resolve")
}

// A shorter idle timeout ends an existing session left unused for longer than it.
func (s *ResolverTestSuite) TestResolve_ShorterIdleTimeoutEndsAnIdleSession() {
	r := s.row("ACTIVE")
	r["last_active_at"] = s.now.Add(-45 * time.Minute)
	r["idle_expires_at"] = s.now.Add(75 * time.Minute)
	s.expectQuery([]map[string]interface{}{r}, nil)
	timeouts := DefaultTimeouts()
	timeouts.Idle = 30 * time.Minute
	s.withTimeouts(timeouts)

	got, err := s.resolver.Resolve(context.Background(), "handle-abc", s.now)

	s.NoError(err)
	s.Nil(got, "a session idle for longer than the current idle timeout must not resolve")
}

// A longer timeout never stretches the deadline stored with a session.
func (s *ResolverTestSuite) TestResolve_LongerTimeoutKeepsTheStoredDeadline() {
	r := s.row("ACTIVE")
	r["absolute_expires_at"] = s.now.Add(-time.Second)
	s.expectQuery([]map[string]interface{}{r}, nil)
	timeouts := DefaultTimeouts()
	timeouts.Absolute = 24 * time.Hour
	s.withTimeouts(timeouts)

	got, err := s.resolver.Resolve(context.Background(), "handle-abc", s.now)

	s.NoError(err)
	s.Nil(got, "a session past its stored deadline must not resolve")
}

// A session within both its stored deadlines and the current timeouts resolves.
func (s *ResolverTestSuite) TestResolve_WithinTheCurrentTimeouts() {
	r := s.row("ACTIVE")
	r["created_at"] = s.now.Add(-30 * time.Minute)
	r["last_active_at"] = s.now.Add(-10 * time.Minute)
	r["idle_expires_at"] = s.now.Add(110 * time.Minute)
	r["absolute_expires_at"] = s.now.Add(450 * time.Minute)
	s.expectQuery([]map[string]interface{}{r}, nil)
	timeouts := DefaultTimeouts()
	timeouts.Idle = 15 * time.Minute
	timeouts.Absolute = time.Hour
	s.withTimeouts(timeouts)

	got, err := s.resolver.Resolve(context.Background(), "handle-abc", s.now)

	s.NoError(err)
	s.NotNil(got)
}
