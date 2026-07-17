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
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// SessionPage is one page of a session listing plus the liveness-filtered total.
type SessionPage struct {
	Sessions     []Session
	TotalResults int
}

// ManagementService is the read-only session visibility surface consumed by the session
// management HTTP layer. It lists live sessions only; the credential (handle) is carried on the
// returned Session values and must never be exposed by callers.
type ManagementService interface {
	// ListBySubject returns one page of the subject's live sessions, most recently active first.
	ListBySubject(ctx context.Context, subjectID string, limit, offset int, now time.Time) (*SessionPage, error)
	// ListByApp returns one page of the live sessions the application has joined.
	ListByApp(ctx context.Context, appID string, limit, offset int, now time.Time) (*SessionPage, error)
	// ListParticipants returns the applications that have joined the session, oldest first.
	ListParticipants(ctx context.Context, sessionID string) ([]Participant, error)
}

// managementService is the store-backed implementation of ManagementService.
type managementService struct {
	store sessionStore
}

var _ ManagementService = (*managementService)(nil)

// NewManagementService builds the read-only session management service over the operation
// datasource. Store construction stays inside this package, mirroring Initialize.
func NewManagementService(dbProvider provider.DBProviderInterface, deploymentID string) ManagementService {
	return newManagementService(newStore(dbProvider, deploymentID))
}

func newManagementService(store sessionStore) *managementService {
	return &managementService{store: store}
}

// ListBySubject implements ManagementService.
func (m *managementService) ListBySubject(ctx context.Context, subjectID string, limit, offset int,
	now time.Time) (*SessionPage, error) {
	sessions, err := m.store.ListBySubject(ctx, subjectID, now, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions by subject: %w", err)
	}
	total, err := m.store.CountBySubject(ctx, subjectID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions by subject: %w", err)
	}
	return &SessionPage{Sessions: sessions, TotalResults: total}, nil
}

// ListByApp implements ManagementService.
func (m *managementService) ListByApp(ctx context.Context, appID string, limit, offset int,
	now time.Time) (*SessionPage, error) {
	sessions, err := m.store.ListByApp(ctx, appID, now, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions by application: %w", err)
	}
	total, err := m.store.CountByApp(ctx, appID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to count sessions by application: %w", err)
	}
	return &SessionPage{Sessions: sessions, TotalResults: total}, nil
}

// ListParticipants implements ManagementService.
func (m *managementService) ListParticipants(ctx context.Context, sessionID string) ([]Participant, error) {
	parts, err := m.store.ListBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list session participants: %w", err)
	}
	return parts, nil
}
