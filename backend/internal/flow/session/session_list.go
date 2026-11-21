// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"fmt"
	"time"
)

// SessionDetail is a live session together with the applications that have joined it.
type SessionDetail struct {
	Session
	Participants []Participant
}

// SessionPage is one page of a session listing, with the total number of live sessions that match.
type SessionPage struct {
	Sessions     []SessionDetail
	TotalResults int
}

// ListLiveBySubject implements Service.
func (s *service) ListLiveBySubject(ctx context.Context, subjectID string, now time.Time,
	limit, offset int) (*SessionPage, error) {
	total, err := s.store.CountLiveBySubject(ctx, subjectID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions by subject: %w", err)
	}
	sessions, err := s.store.ListLiveBySubject(ctx, subjectID, now, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions by subject: %w", err)
	}
	return s.page(ctx, sessions, total)
}

// ListLiveByApp implements Service.
func (s *service) ListLiveByApp(ctx context.Context, appID string, now time.Time,
	limit, offset int) (*SessionPage, error) {
	total, err := s.store.CountLiveByApp(ctx, appID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions by application: %w", err)
	}
	sessions, err := s.store.ListLiveByApp(ctx, appID, now, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions by application: %w", err)
	}
	return s.page(ctx, sessions, total)
}

// page attaches the participants to a page of sessions, reading them for the whole page in one query.
func (s *service) page(ctx context.Context, sessions []Session, total int) (*SessionPage, error) {
	sessionIDs := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		sessionIDs = append(sessionIDs, sess.SessionID)
	}
	participants, err := s.store.ListBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to list session participants: %w", err)
	}
	bySession := make(map[string][]Participant, len(sessions))
	for _, p := range participants {
		bySession[p.SessionID] = append(bySession[p.SessionID], p)
	}

	details := make([]SessionDetail, 0, len(sessions))
	for _, sess := range sessions {
		details = append(details, SessionDetail{Session: sess, Participants: bySession[sess.SessionID]})
	}
	return &SessionPage{Sessions: details, TotalResults: total}, nil
}
