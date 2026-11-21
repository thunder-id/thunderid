// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type SessionListTestSuite struct {
	suite.Suite
	store *sessionStoreMock
	svc   *service
	now   time.Time
}

func TestSessionListTestSuite(t *testing.T) {
	suite.Run(t, new(SessionListTestSuite))
}

func (s *SessionListTestSuite) SetupTest() {
	s.store = newSessionStoreMock(s.T())
	s.svc = &service{store: s.store}
	s.now = time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
}

func (s *SessionListTestSuite) liveSession(id string) Session {
	return Session{
		SessionID:         id,
		SubjectID:         "user-1",
		State:             StateActive,
		IdleExpiresAt:     s.now.Add(time.Minute),
		AbsoluteExpiresAt: s.now.Add(time.Hour),
	}
}

func (s *SessionListTestSuite) TestListLiveBySubject_GroupsParticipantsBySession() {
	ctx := context.Background()
	s.store.EXPECT().CountLiveBySubject(ctx, "user-1", s.now).Return(5, nil)
	s.store.EXPECT().ListLiveBySubject(ctx, "user-1", s.now, 2, 2).
		Return([]Session{s.liveSession("sess-2"), s.liveSession("sess-1")}, nil)
	s.store.EXPECT().ListBySessionIDs(ctx, []string{"sess-2", "sess-1"}).Return([]Participant{
		{SessionID: "sess-1", AppID: "app-a"},
		{SessionID: "sess-2", AppID: "app-b"},
		{SessionID: "sess-1", AppID: "app-c"},
	}, nil)

	page, err := s.svc.ListLiveBySubject(ctx, "user-1", s.now, 2, 2)

	s.NoError(err)
	s.Equal(5, page.TotalResults)
	s.Require().Len(page.Sessions, 2)
	s.Equal("sess-2", page.Sessions[0].SessionID, "the store's order is kept")
	s.Equal([]Participant{{SessionID: "sess-2", AppID: "app-b"}}, page.Sessions[0].Participants)
	s.Equal([]Participant{{SessionID: "sess-1", AppID: "app-a"}, {SessionID: "sess-1", AppID: "app-c"}},
		page.Sessions[1].Participants)
}

func (s *SessionListTestSuite) TestListLiveByApp_EmptyPage() {
	ctx := context.Background()
	s.store.EXPECT().CountLiveByApp(ctx, "app-1", s.now).Return(0, nil)
	s.store.EXPECT().ListLiveByApp(ctx, "app-1", s.now, 30, 0).Return([]Session{}, nil)
	s.store.EXPECT().ListBySessionIDs(ctx, []string{}).Return(nil, nil)

	page, err := s.svc.ListLiveByApp(ctx, "app-1", s.now, 30, 0)

	s.NoError(err)
	s.Zero(page.TotalResults)
	s.NotNil(page.Sessions)
	s.Empty(page.Sessions)
}

func (s *SessionListTestSuite) TestList_StoreErrors() {
	ctx := context.Background()
	storeErr := errors.New("db down")

	s.Run("count by subject", func() {
		s.SetupTest()
		s.store.EXPECT().CountLiveBySubject(ctx, "user-1", s.now).Return(0, storeErr)
		_, err := s.svc.ListLiveBySubject(ctx, "user-1", s.now, 30, 0)
		s.ErrorIs(err, storeErr)
	})
	s.Run("list by subject", func() {
		s.SetupTest()
		s.store.EXPECT().CountLiveBySubject(ctx, "user-1", s.now).Return(1, nil)
		s.store.EXPECT().ListLiveBySubject(ctx, "user-1", s.now, 30, 0).Return(nil, storeErr)
		_, err := s.svc.ListLiveBySubject(ctx, "user-1", s.now, 30, 0)
		s.ErrorIs(err, storeErr)
	})
	s.Run("count by app", func() {
		s.SetupTest()
		s.store.EXPECT().CountLiveByApp(ctx, "app-1", s.now).Return(0, storeErr)
		_, err := s.svc.ListLiveByApp(ctx, "app-1", s.now, 30, 0)
		s.ErrorIs(err, storeErr)
	})
	s.Run("list by app", func() {
		s.SetupTest()
		s.store.EXPECT().CountLiveByApp(ctx, "app-1", s.now).Return(1, nil)
		s.store.EXPECT().ListLiveByApp(ctx, "app-1", s.now, 30, 0).Return(nil, storeErr)
		_, err := s.svc.ListLiveByApp(ctx, "app-1", s.now, 30, 0)
		s.ErrorIs(err, storeErr)
	})
	s.Run("participants", func() {
		s.SetupTest()
		s.store.EXPECT().CountLiveByApp(ctx, "app-1", s.now).Return(1, nil)
		s.store.EXPECT().ListLiveByApp(ctx, "app-1", s.now, 30, 0).Return([]Session{s.liveSession("sess-1")}, nil)
		s.store.EXPECT().ListBySessionIDs(ctx, []string{"sess-1"}).Return(nil, storeErr)
		_, err := s.svc.ListLiveByApp(ctx, "app-1", s.now, 30, 0)
		s.ErrorIs(err, storeErr)
	})
}
