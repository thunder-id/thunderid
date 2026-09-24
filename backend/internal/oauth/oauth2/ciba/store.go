// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// cibaStateField is the JSON field name that guards the atomic state transitions. It must match the
// marshaled field name of CIBAAuthRequest.State.
const cibaStateField = "State"

// CIBARequestStoreInterface defines the interface for CIBA authentication request storage.
type CIBARequestStoreInterface interface {
	Add(ctx context.Context, request *CIBAAuthRequest) error
	GetByID(ctx context.Context, authReqID string) (*CIBAAuthRequest, error)
	MarkAuthenticated(ctx context.Context, authReqID, userID, authorizedScopes, attributeCacheID,
		completedACR, sessionID string, authTime time.Time) error
	MarkConsumed(ctx context.Context, authReqID string) (bool, error)
	UpdateLastPolled(ctx context.Context, authReqID string, polledAt time.Time) error
	UpdateState(ctx context.Context, authReqID string, state CIBARequestState) error
}

// cibaStore adapts a runtime store provider to CIBA authentication request storage. Requests are
// stored under the CIBA namespace, keyed by auth request ID, as a serialized CIBAAuthRequest.
type cibaStore struct {
	store providers.RuntimeStoreProvider
}

// newCIBAStore creates a CIBA request store backed by the given runtime store provider.
func newCIBAStore(store providers.RuntimeStoreProvider) CIBARequestStoreInterface {
	return &cibaStore{store: store}
}

// Add inserts a new CIBA authentication request with a TTL derived from its expiry time.
// UserID is empty at creation — it is populated by MarkAuthenticated once the callback verifies
// the assertion.
func (s *cibaStore) Add(ctx context.Context, request *CIBAAuthRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal CIBA request: %w", err)
	}

	ttl := time.Until(request.ExpiryTime)
	if ttl <= 0 {
		return fmt.Errorf("CIBA request already expired")
	}

	return s.store.Put(ctx, providers.NamespaceCIBA, request.AuthReqID, data, int64(ttl.Seconds()))
}

// GetByID retrieves a CIBA authentication request by ID. Returns ErrCIBARequestNotFound if absent.
func (s *cibaStore) GetByID(ctx context.Context, authReqID string) (*CIBAAuthRequest, error) {
	if authReqID == "" {
		return nil, ErrCIBARequestNotFound
	}

	data, err := s.store.Get(ctx, providers.NamespaceCIBA, authReqID)
	if err != nil {
		return nil, fmt.Errorf("failed to get CIBA request: %w", err)
	}
	if data == nil {
		return nil, ErrCIBARequestNotFound
	}

	var request CIBAAuthRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CIBA request: %w", err)
	}
	return &request, nil
}

// MarkAuthenticated transitions a pending request to authenticated and records the user ID (from
// the assertion sub claim), authorized scopes, attribute cache ID, completed ACR, SSO session id, and
// authentication time. The compare-and-swap on the State field prevents a double-callback race condition.
func (s *cibaStore) MarkAuthenticated(ctx context.Context, authReqID, userID,
	authorizedScopes, attributeCacheID, completedACR, sessionID string, authTime time.Time) error {
	record, err := s.GetByID(ctx, authReqID)
	if err != nil {
		return err
	}
	if record.State != CIBAStatePending {
		return fmt.Errorf("CIBA request %s is not pending", authReqID)
	}

	record.State = CIBAStateAuthenticated
	record.UserID = userID
	record.AuthorizedScopes = authorizedScopes
	record.AttributeCacheID = attributeCacheID
	record.CompletedACR = completedACR
	record.SessionID = sessionID
	record.AuthTime = authTime

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal CIBA request: %w", err)
	}

	swapped, err := s.store.CompareFieldAndSwap(ctx, providers.NamespaceCIBA, authReqID,
		cibaStateField, string(CIBAStatePending), data)
	if err != nil {
		return fmt.Errorf("failed to mark CIBA request as authenticated: %w", err)
	}
	if !swapped {
		return fmt.Errorf("CIBA request %s is not pending", authReqID)
	}
	return nil
}

// MarkConsumed atomically transitions an authenticated request to consumed. It returns false when
// the request is not in the AUTHENTICATED state (already consumed or otherwise terminal), enabling
// one-time-use enforcement under concurrent polling.
func (s *cibaStore) MarkConsumed(ctx context.Context, authReqID string) (bool, error) {
	record, err := s.GetByID(ctx, authReqID)
	if errors.Is(err, ErrCIBARequestNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if record.State != CIBAStateAuthenticated {
		return false, nil
	}

	record.State = CIBAStateConsumed

	data, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("failed to marshal CIBA request: %w", err)
	}

	swapped, err := s.store.CompareFieldAndSwap(ctx, providers.NamespaceCIBA, authReqID,
		cibaStateField, string(CIBAStateAuthenticated), data)
	if err != nil {
		return false, fmt.Errorf("failed to consume CIBA request: %w", err)
	}
	return swapped, nil
}

// UpdateLastPolled updates the last polled timestamp of a CIBA authentication request.
func (s *cibaStore) UpdateLastPolled(ctx context.Context, authReqID string, polledAt time.Time) error {
	record, err := s.GetByID(ctx, authReqID)
	if err != nil {
		return err
	}
	record.LastPolledAt = polledAt
	return s.save(ctx, record)
}

// UpdateState updates the state of a CIBA authentication request.
func (s *cibaStore) UpdateState(ctx context.Context, authReqID string, state CIBARequestState) error {
	record, err := s.GetByID(ctx, authReqID)
	if err != nil {
		return err
	}
	record.State = state
	return s.save(ctx, record)
}

// save serializes and writes the record back to the store, preserving its TTL.
func (s *cibaStore) save(ctx context.Context, record *CIBAAuthRequest) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal CIBA request: %w", err)
	}
	return s.store.Update(ctx, providers.NamespaceCIBA, record.AuthReqID, data)
}
