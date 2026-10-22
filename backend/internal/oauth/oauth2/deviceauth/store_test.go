// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/runtimestoreprovidermock"
)

func TestDeviceAuthStoreAddAndGet(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	deviceCode := "device-code"
	authTime := time.Now().Add(-time.Minute).Truncate(time.Second)
	lastPolledAt := time.Now().Add(-30 * time.Second).Truncate(time.Second)
	request := &DeviceAuthRequest{
		RequestID:             "request-1",
		ApplicationID:         "application-1",
		ApplicationAuthFlowID: "authentication-flow-1",
		DeviceCodeHash:        hashDeviceCode(deviceCode),
		UserCode:              "BCDF-GHJK",
		ClientID:              "device-client",
		UserID:                "user-1",
		StandardScopes:        "openid profile",
		RequestedPermissions:  "booking:read",
		AuthorizedScopes:      "openid",
		Resources:             []string{"https://api.example.com"},
		State:                 DeviceStateApproved,
		AttributeCacheID:      "attribute-cache-1",
		CompletedACR:          "urn:acr:mfa",
		AuthTime:              authTime,
		LastPolledAt:          lastPolledAt,
		PollInterval:          5,
		ExpiryTime:            time.Now().Add(2 * time.Minute),
	}

	require.NoError(t, store.Add(context.Background(), request))

	byDeviceCode, err := store.GetByDeviceCode(context.Background(), deviceCode)
	require.NoError(t, err)
	require.Equal(t, request.RequestID, byDeviceCode.RequestID)
	require.Equal(t, request.ApplicationID, byDeviceCode.ApplicationID)
	require.Equal(t, request.ApplicationAuthFlowID, byDeviceCode.ApplicationAuthFlowID)
	require.Equal(t, request.DeviceCodeHash, byDeviceCode.DeviceCodeHash)
	require.Equal(t, request.UserCode, byDeviceCode.UserCode)
	require.Equal(t, request.ClientID, byDeviceCode.ClientID)
	require.Equal(t, request.UserID, byDeviceCode.UserID)
	require.Equal(t, request.StandardScopes, byDeviceCode.StandardScopes)
	require.Equal(t, request.RequestedPermissions, byDeviceCode.RequestedPermissions)
	require.Equal(t, request.AuthorizedScopes, byDeviceCode.AuthorizedScopes)
	require.Equal(t, request.Resources, byDeviceCode.Resources)
	require.Equal(t, request.State, byDeviceCode.State)
	require.Equal(t, request.AttributeCacheID, byDeviceCode.AttributeCacheID)
	require.Equal(t, request.CompletedACR, byDeviceCode.CompletedACR)
	require.Equal(t, request.AuthTime, byDeviceCode.AuthTime)
	require.Equal(t, request.LastPolledAt, byDeviceCode.LastPolledAt)
	require.Equal(t, request.PollInterval, byDeviceCode.PollInterval)
	require.WithinDuration(t, request.ExpiryTime, byDeviceCode.ExpiryTime, time.Second)

	byUserCode, err := store.GetByUserCode(context.Background(), request.UserCode)
	require.NoError(t, err)
	require.Equal(t, byDeviceCode, byUserCode)

	byRequestID, err := store.GetByRequestID(context.Background(), request.RequestID)
	require.NoError(t, err)
	require.Equal(t, byDeviceCode, byRequestID)

	byNormalizedUserCode, err := store.GetByUserCode(context.Background(), "bcdf ghjk")
	require.NoError(t, err)
	require.Equal(t, byDeviceCode, byNormalizedUserCode)
}

func TestDeviceAuthStoreReturnsRollbackFailure(t *testing.T) {
	runtimeStore := runtimestoreprovidermock.NewRuntimeStoreProviderMock(t)
	request := &DeviceAuthRequest{
		RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode: "BCDF-GHJK", ExpiryTime: time.Now().Add(2 * time.Minute),
	}
	storeFailure := errors.New("device code storage failed")
	rollbackFailure := errors.New("user code cleanup failed")
	runtimeStore.EXPECT().PutIfNotExists(mock.Anything, providers.NamespaceDeviceAuth,
		requestIDKeyPrefix+request.RequestID, mock.Anything, mock.Anything).Return(true, nil)
	runtimeStore.EXPECT().PutIfNotExists(mock.Anything, providers.NamespaceDeviceAuth,
		userCodeKeyPrefix+normalizeUserCode(request.UserCode), mock.Anything, mock.Anything).Return(true, nil)
	runtimeStore.EXPECT().PutIfNotExists(mock.Anything, providers.NamespaceDeviceAuth,
		deviceCodeKeyPrefix+request.DeviceCodeHash, mock.Anything, mock.Anything).Return(false, storeFailure)
	runtimeStore.EXPECT().Delete(mock.Anything, providers.NamespaceDeviceAuth,
		userCodeKeyPrefix+normalizeUserCode(request.UserCode)).Return(rollbackFailure)
	runtimeStore.EXPECT().Delete(mock.Anything, providers.NamespaceDeviceAuth,
		requestIDKeyPrefix+request.RequestID).Return(nil)

	err := newDeviceAuthStore(runtimeStore).Add(context.Background(), request)

	require.ErrorIs(t, err, storeFailure)
	require.ErrorIs(t, err, rollbackFailure)
}

func TestDeviceAuthStoreDoesNotRetryCollisionAfterRollbackFailure(t *testing.T) {
	runtimeStore := runtimestoreprovidermock.NewRuntimeStoreProviderMock(t)
	request := &DeviceAuthRequest{
		RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode: "BCDF-GHJK", ExpiryTime: time.Now().Add(2 * time.Minute),
	}
	rollbackFailure := errors.New("request ID cleanup failed")
	runtimeStore.EXPECT().PutIfNotExists(mock.Anything, providers.NamespaceDeviceAuth,
		requestIDKeyPrefix+request.RequestID, mock.Anything, mock.Anything).Return(true, nil)
	runtimeStore.EXPECT().PutIfNotExists(mock.Anything, providers.NamespaceDeviceAuth,
		userCodeKeyPrefix+normalizeUserCode(request.UserCode), mock.Anything, mock.Anything).Return(false, nil)
	runtimeStore.EXPECT().Delete(mock.Anything, providers.NamespaceDeviceAuth,
		requestIDKeyPrefix+request.RequestID).Return(rollbackFailure)

	err := newDeviceAuthStore(runtimeStore).Add(context.Background(), request)

	require.ErrorIs(t, err, rollbackFailure)
	require.NotErrorIs(t, err, ErrDeviceAuthUserCodeCollision)
}

func TestDeviceAuthStoreRejectsDuplicateUserCode(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	first := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code-1"),
		UserCode:   "BCDF-GHJK",
		ExpiryTime: time.Now().Add(2 * time.Minute)}
	second := &DeviceAuthRequest{RequestID: "request-2", DeviceCodeHash: hashDeviceCode("device-code-2"),
		UserCode:   first.UserCode,
		ExpiryTime: time.Now().Add(2 * time.Minute)}

	require.NoError(t, store.Add(context.Background(), first))
	require.ErrorIs(t, store.Add(context.Background(), second), ErrDeviceAuthUserCodeCollision)
}

func TestDeviceAuthStoreRejectsDuplicateDeviceCode(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	first := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode:   "BCDF-GHJK",
		ExpiryTime: time.Now().Add(2 * time.Minute)}
	second := &DeviceAuthRequest{RequestID: "request-2", DeviceCodeHash: first.DeviceCodeHash,
		UserCode:   "LMNP-QRST",
		ExpiryTime: time.Now().Add(2 * time.Minute)}

	require.NoError(t, store.Add(context.Background(), first))
	require.ErrorIs(t, store.Add(context.Background(), second), ErrDeviceAuthDeviceCodeCollision)

	_, err := store.GetByUserCode(context.Background(), second.UserCode)
	require.ErrorIs(t, err, ErrDeviceAuthRequestNotFound)
}

func TestDeviceAuthStoreRejectsExpiredRequest(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	request := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode:   "BCDF-GHJK",
		ExpiryTime: time.Now().Add(-time.Second)}

	err := store.Add(context.Background(), request)
	require.EqualError(t, err, "device authorization request already expired")
}

func TestDeviceAuthStoreReturnsNotFound(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))

	_, err := store.GetByDeviceCode(context.Background(), "missing")
	require.ErrorIs(t, err, ErrDeviceAuthRequestNotFound)

	_, err = store.GetByUserCode(context.Background(), "missing")
	require.ErrorIs(t, err, ErrDeviceAuthRequestNotFound)

	_, err = store.GetByRequestID(context.Background(), "missing")
	require.ErrorIs(t, err, ErrDeviceAuthRequestNotFound)

	_, err = store.GetByUserCode(context.Background(), " - ")
	require.ErrorIs(t, err, ErrDeviceAuthRequestNotFound)
}

func TestDeviceAuthStoreRejectsDuplicateRequestID(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	first := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code-1"),
		UserCode: "BCDF-GHJK", ExpiryTime: time.Now().Add(2 * time.Minute)}
	second := &DeviceAuthRequest{RequestID: first.RequestID, DeviceCodeHash: hashDeviceCode("device-code-2"),
		UserCode: "LMNP-QRST", ExpiryTime: time.Now().Add(2 * time.Minute)}

	require.NoError(t, store.Add(context.Background(), first))
	require.ErrorIs(t, store.Add(context.Background(), second), ErrDeviceAuthRequestIDCollision)
}

func TestDeviceAuthStoreMarkRedeemingIsAtomic(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	request := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode: "BCDF-GHJK", State: DeviceStatePending, ExpiryTime: time.Now().Add(2 * time.Minute)}
	require.NoError(t, store.Add(context.Background(), request))

	first, err := store.MarkRedeeming(context.Background(), request.RequestID)
	require.NoError(t, err)
	require.True(t, first)

	second, err := store.MarkRedeeming(context.Background(), request.RequestID)
	require.NoError(t, err)
	require.False(t, second)

	stored, err := store.GetByRequestID(context.Background(), request.RequestID)
	require.NoError(t, err)
	require.Equal(t, DeviceStateRedeeming, stored.State)
}

func TestDeviceAuthStoreUpdatePollingDoesNotRevertConcurrentState(t *testing.T) {
	runtimeStore := inmemory.Initialize("test-deployment")
	request := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode: "BCDF-GHJK", State: DeviceStatePending, ExpiryTime: time.Now().Add(2 * time.Minute)}
	initialStore := newDeviceAuthStore(runtimeStore)
	require.NoError(t, initialStore.Add(context.Background(), request))

	store := newDeviceAuthStore(&runtimeStoreStateTransitionOnRead{
		RuntimeStoreProvider: runtimeStore,
		targetKey:            deviceCodeKeyPrefix + request.DeviceCodeHash,
		transitionTo:         DeviceStateRedeeming,
	})
	err := store.UpdatePolling(context.Background(), request.RequestID, time.Now(), 5)
	require.NoError(t, err)

	stored, err := initialStore.GetByRequestID(context.Background(), request.RequestID)
	require.NoError(t, err)
	require.Equal(t, DeviceStateRedeeming, stored.State)
	require.True(t, stored.LastPolledAt.IsZero())
}

func TestDeviceAuthStoreUpdateStateDoesNotOverwriteConcurrentTransition(t *testing.T) {
	runtimeStore := inmemory.Initialize("test-deployment")
	request := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode: "BCDF-GHJK", State: DeviceStatePending, ExpiryTime: time.Now().Add(2 * time.Minute)}
	initialStore := newDeviceAuthStore(runtimeStore)
	require.NoError(t, initialStore.Add(context.Background(), request))

	store := newDeviceAuthStore(&runtimeStoreStateTransitionOnRead{
		RuntimeStoreProvider: runtimeStore,
		targetKey:            deviceCodeKeyPrefix + request.DeviceCodeHash,
		transitionTo:         DeviceStateRedeeming,
	})
	err := store.UpdateState(context.Background(), request.RequestID, DeviceStateExpired)
	require.Error(t, err)

	stored, err := initialStore.GetByRequestID(context.Background(), request.RequestID)
	require.NoError(t, err)
	require.Equal(t, DeviceStateRedeeming, stored.State)
}

type runtimeStoreStateTransitionOnRead struct {
	providers.RuntimeStoreProvider
	targetKey    string
	transitionTo DeviceRequestState
	transitioned bool
}

func (s *runtimeStoreStateTransitionOnRead) Get(
	ctx context.Context, namespace providers.RuntimeStoreNamespace, key string,
) ([]byte, error) {
	data, err := s.RuntimeStoreProvider.Get(ctx, namespace, key)
	if err != nil || key != s.targetKey || s.transitioned {
		return data, err
	}
	var record DeviceAuthRequest
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	expectedState := record.State
	record.State = s.transitionTo
	updatedData, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	s.transitioned, err = s.RuntimeStoreProvider.CompareFieldAndSwap(ctx, namespace, key,
		deviceStateField, string(expectedState), updatedData)
	if err != nil {
		return nil, err
	}
	if !s.transitioned {
		return nil, errors.New("concurrent state transition did not occur")
	}
	return data, nil
}
