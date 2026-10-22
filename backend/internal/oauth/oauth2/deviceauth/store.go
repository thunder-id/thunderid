// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	deviceCodeKeyPrefix = "device:"
	userCodeKeyPrefix   = "usercode:"
	requestIDKeyPrefix  = "request:"
)

const deviceStateField = "State"

// DeviceAuthRequestStoreInterface defines device authorization request storage operations.
type DeviceAuthRequestStoreInterface interface {
	Add(ctx context.Context, request *DeviceAuthRequest) error
	GetByDeviceCode(ctx context.Context, deviceCode string) (*DeviceAuthRequest, error)
	GetByUserCode(ctx context.Context, userCode string) (*DeviceAuthRequest, error)
	GetByRequestID(ctx context.Context, requestID string) (*DeviceAuthRequest, error)
	MarkRedeeming(ctx context.Context, requestID string) (bool, error)
	MarkApproved(ctx context.Context, requestID, userID, authorizedScopes, attributeCacheID,
		completedACR, sessionID string, authTime time.Time) error
	MarkFailed(ctx context.Context, requestID string, state DeviceRequestState) error
	UpdatePolling(ctx context.Context, requestID string, polledAt time.Time, interval int64) error
	UpdateState(ctx context.Context, requestID string, state DeviceRequestState) error
	MarkConsumed(ctx context.Context, requestID string) (bool, error)
}

// deviceAuthStore adapts a runtime store provider to device authorization request storage.
type deviceAuthStore struct {
	store providers.RuntimeStoreProvider
}

// newDeviceAuthStore creates a device authorization request store backed by the runtime store.
func newDeviceAuthStore(store providers.RuntimeStoreProvider) DeviceAuthRequestStoreInterface {
	return &deviceAuthStore{store: store}
}

// Add reserves the user code and stores the request with a TTL derived from its expiry time.
func (s *deviceAuthStore) Add(ctx context.Context, request *DeviceAuthRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal device authorization request: %w", err)
	}

	ttl := time.Until(request.ExpiryTime)
	if ttl <= 0 {
		return fmt.Errorf("device authorization request already expired")
	}
	ttlSeconds := int64(ttl.Seconds()) + oauth2const.DeviceAuthorizationExpiredRecordRetentionSeconds

	requestIDKey := requestIDKeyPrefix + request.RequestID
	added, err := s.store.PutIfNotExists(ctx, providers.NamespaceDeviceAuth, requestIDKey,
		[]byte(request.DeviceCodeHash), ttlSeconds)
	if err != nil {
		return fmt.Errorf("failed to reserve device authorization request ID: %w", err)
	}
	if !added {
		return ErrDeviceAuthRequestIDCollision
	}

	userCodeKey := userCodeKeyPrefix + normalizeUserCode(request.UserCode)
	added, err = s.store.PutIfNotExists(ctx, providers.NamespaceDeviceAuth, userCodeKey,
		[]byte(request.DeviceCodeHash), ttlSeconds)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to reserve device authorization user code: %w", err),
			s.rollbackReservations(ctx, requestIDKey))
	}
	if !added {
		if rollbackErr := s.rollbackReservations(ctx, requestIDKey); rollbackErr != nil {
			return fmt.Errorf("failed to rollback device authorization user code collision: %w", rollbackErr)
		}
		return ErrDeviceAuthUserCodeCollision
	}

	deviceCodeKey := deviceCodeKeyPrefix + request.DeviceCodeHash
	added, err = s.store.PutIfNotExists(ctx, providers.NamespaceDeviceAuth, deviceCodeKey, data, ttlSeconds)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to store device authorization request: %w", err),
			s.rollbackReservations(ctx, userCodeKey, requestIDKey))
	}
	if !added {
		return errors.Join(ErrDeviceAuthDeviceCodeCollision,
			s.rollbackReservations(ctx, userCodeKey, requestIDKey))
	}

	return nil
}

// rollbackReservations releases keys reserved while adding a device authorization request.
func (s *deviceAuthStore) rollbackReservations(ctx context.Context, keys ...string) error {
	var rollbackErr error
	for _, key := range keys {
		if err := s.store.Delete(ctx, providers.NamespaceDeviceAuth, key); err != nil {
			rollbackErr = errors.Join(rollbackErr,
				fmt.Errorf("failed to release device authorization reservation: %w", err))
		}
	}
	return rollbackErr
}

// GetByDeviceCode retrieves a device authorization request by its device code.
func (s *deviceAuthStore) GetByDeviceCode(ctx context.Context, deviceCode string) (*DeviceAuthRequest, error) {
	if deviceCode == "" {
		return nil, ErrDeviceAuthRequestNotFound
	}

	return s.getByDeviceCodeHash(ctx, hashDeviceCode(deviceCode))
}

// getByDeviceCodeHash retrieves a device authorization request by its stored code hash.
func (s *deviceAuthStore) getByDeviceCodeHash(
	ctx context.Context, deviceCodeHash string,
) (*DeviceAuthRequest, error) {
	data, err := s.store.Get(ctx, providers.NamespaceDeviceAuth, deviceCodeKeyPrefix+deviceCodeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get device authorization request: %w", err)
	}
	if data == nil {
		return nil, ErrDeviceAuthRequestNotFound
	}

	var request DeviceAuthRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal device authorization request: %w", err)
	}
	return &request, nil
}

// GetByUserCode retrieves a device authorization request through its user-code index.
func (s *deviceAuthStore) GetByUserCode(ctx context.Context, userCode string) (*DeviceAuthRequest, error) {
	normalizedUserCode := normalizeUserCode(userCode)
	if normalizedUserCode == "" {
		return nil, ErrDeviceAuthRequestNotFound
	}

	deviceCodeHash, err := s.store.Get(ctx, providers.NamespaceDeviceAuth,
		userCodeKeyPrefix+normalizedUserCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get device authorization user-code index: %w", err)
	}
	if deviceCodeHash == nil {
		return nil, ErrDeviceAuthRequestNotFound
	}

	return s.getByDeviceCodeHash(ctx, string(deviceCodeHash))
}

// GetByRequestID retrieves a device authorization request through its request-ID index.
func (s *deviceAuthStore) GetByRequestID(ctx context.Context, requestID string) (*DeviceAuthRequest, error) {
	if requestID == "" {
		return nil, ErrDeviceAuthRequestNotFound
	}

	deviceCodeHash, err := s.store.Get(ctx, providers.NamespaceDeviceAuth, requestIDKeyPrefix+requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device authorization request-ID index: %w", err)
	}
	if deviceCodeHash == nil {
		return nil, ErrDeviceAuthRequestNotFound
	}

	return s.getByDeviceCodeHash(ctx, string(deviceCodeHash))
}

// MarkRedeeming atomically claims a pending request for one browser authorization journey.
func (s *deviceAuthStore) MarkRedeeming(ctx context.Context, requestID string) (bool, error) {
	record, err := s.GetByRequestID(ctx, requestID)
	if err != nil {
		return false, err
	}
	if record.State != DeviceStatePending {
		return false, nil
	}

	record.State = DeviceStateRedeeming
	data, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("failed to marshal device authorization request: %w", err)
	}

	swapped, err := s.store.CompareFieldAndSwap(ctx, providers.NamespaceDeviceAuth,
		deviceCodeKeyPrefix+record.DeviceCodeHash, deviceStateField, string(DeviceStatePending), data)
	if err != nil {
		return false, fmt.Errorf("failed to claim device authorization request: %w", err)
	}
	return swapped, nil
}

// MarkApproved atomically completes a redeeming request and stores the authenticated user details.
func (s *deviceAuthStore) MarkApproved(ctx context.Context, requestID, userID, authorizedScopes,
	attributeCacheID, completedACR, sessionID string, authTime time.Time) error {
	record, err := s.GetByRequestID(ctx, requestID)
	if err != nil {
		return err
	}
	if record.State != DeviceStateRedeeming {
		return fmt.Errorf("device authorization request %s is not redeeming", requestID)
	}

	record.State = DeviceStateApproved
	record.UserID = userID
	record.AuthorizedScopes = authorizedScopes
	record.AttributeCacheID = attributeCacheID
	record.CompletedACR = completedACR
	record.SessionID = sessionID
	record.AuthTime = authTime

	return s.compareAndSwapState(ctx, record, DeviceStateRedeeming)
}

// MarkFailed atomically moves a redeeming request to DENIED or FAILED.
func (s *deviceAuthStore) MarkFailed(
	ctx context.Context, requestID string, state DeviceRequestState,
) error {
	if state != DeviceStateDenied && state != DeviceStateFailed {
		return fmt.Errorf("invalid terminal device authorization state %s", state)
	}
	record, err := s.GetByRequestID(ctx, requestID)
	if err != nil {
		return err
	}
	if record.State != DeviceStateRedeeming {
		return fmt.Errorf("device authorization request %s is not redeeming", requestID)
	}

	record.State = state
	return s.compareAndSwapState(ctx, record, DeviceStateRedeeming)
}

// UpdatePolling records the latest token poll and the interval required for the next poll.
func (s *deviceAuthStore) UpdatePolling(
	ctx context.Context, requestID string, polledAt time.Time, interval int64,
) error {
	record, err := s.GetByRequestID(ctx, requestID)
	if err != nil {
		return err
	}
	record.LastPolledAt = polledAt
	record.PollInterval = interval
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal device authorization request: %w", err)
	}
	if _, err := s.store.CompareFieldAndSwap(ctx, providers.NamespaceDeviceAuth,
		deviceCodeKeyPrefix+record.DeviceCodeHash, deviceStateField, string(record.State), data); err != nil {
		return fmt.Errorf("failed to update device authorization polling: %w", err)
	}
	return nil
}

// UpdateState updates a device authorization request state while preserving its storage lifetime.
func (s *deviceAuthStore) UpdateState(
	ctx context.Context, requestID string, state DeviceRequestState,
) error {
	record, err := s.GetByRequestID(ctx, requestID)
	if err != nil {
		return err
	}
	expected := record.State
	record.State = state
	return s.compareAndSwapState(ctx, record, expected)
}

// MarkConsumed atomically transitions an approved request to consumed.
func (s *deviceAuthStore) MarkConsumed(ctx context.Context, requestID string) (bool, error) {
	record, err := s.GetByRequestID(ctx, requestID)
	if errors.Is(err, ErrDeviceAuthRequestNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if record.State != DeviceStateApproved {
		return false, nil
	}

	record.State = DeviceStateConsumed
	data, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("failed to marshal device authorization request: %w", err)
	}
	swapped, err := s.store.CompareFieldAndSwap(ctx, providers.NamespaceDeviceAuth,
		deviceCodeKeyPrefix+record.DeviceCodeHash, deviceStateField, string(DeviceStateApproved), data)
	if err != nil {
		return false, fmt.Errorf("failed to consume device authorization request: %w", err)
	}
	return swapped, nil
}

// compareAndSwapState persists a request only when its stored state matches the expected state.
func (s *deviceAuthStore) compareAndSwapState(
	ctx context.Context, record *DeviceAuthRequest, expected DeviceRequestState,
) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal device authorization request: %w", err)
	}

	swapped, err := s.store.CompareFieldAndSwap(ctx, providers.NamespaceDeviceAuth,
		deviceCodeKeyPrefix+record.DeviceCodeHash, deviceStateField, string(expected), data)
	if err != nil {
		return fmt.Errorf("failed to update device authorization request: %w", err)
	}
	if !swapped {
		return fmt.Errorf("device authorization request %s is not %s", record.RequestID, expected)
	}
	return nil
}

// hashDeviceCode returns the storage-safe hash used to index a device code.
func hashDeviceCode(deviceCode string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(deviceCode)))
}
