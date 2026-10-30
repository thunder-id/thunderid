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
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/runtimestoreprovidermock"
)

const testUserCodeIndexKey = "device-auth-test-index-key-with-sufficient-entropy"

func newTestDeviceAuthStore(store providers.RuntimeStoreProvider) *deviceAuthStore {
	return newDeviceAuthStore(store, []byte(testUserCodeIndexKey))
}

type DeviceAuthStoreTestSuite struct {
	suite.Suite
}

func TestDeviceAuthStoreTestSuite(t *testing.T) {
	suite.Run(t, new(DeviceAuthStoreTestSuite))
}

func (s *DeviceAuthStoreTestSuite) TestAddAndGet() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	deviceCode := "device-code"
	authTime := time.Now().Add(-time.Minute).Truncate(time.Second)
	lastPolledAt := time.Now().Add(-30 * time.Second).Truncate(time.Second)
	request := &DeviceAuthRequest{
		RequestID:            "request-1",
		ApplicationID:        "application-1",
		DeviceCodeHash:       hashDeviceCode(deviceCode),
		UserCode:             "BCDF-GHJK",
		ClientID:             "device-client",
		UserID:               "user-1",
		StandardScopes:       "openid profile",
		RequestedPermissions: "booking:read",
		AuthorizedScopes:     "openid",
		Resources:            []string{"https://api.example.com"},
		State:                DeviceStateApproved,
		AttributeCacheID:     "attribute-cache-1",
		CompletedACR:         "urn:acr:mfa",
		AuthTime:             authTime,
		LastPolledAt:         lastPolledAt,
		PollInterval:         5,
		ExpiryTime:           time.Now().Add(2 * time.Minute),
	}

	s.Require().NoError(store.Add(context.Background(), request))

	byDeviceCode, err := store.GetByDeviceCode(context.Background(), deviceCode)
	s.Require().NoError(err)
	storedJSON, err := store.store.Get(context.Background(), providers.NamespaceDeviceAuth,
		deviceCodeKeyPrefix+request.DeviceCodeHash)
	s.Require().NoError(err)
	s.Require().NotContains(string(storedJSON), request.UserCode)
	s.Require().Equal(request.RequestID, byDeviceCode.RequestID)
	s.Require().Equal(request.ApplicationID, byDeviceCode.ApplicationID)
	s.Require().Equal(request.DeviceCodeHash, byDeviceCode.DeviceCodeHash)
	s.Require().Empty(byDeviceCode.UserCode, "plaintext user code must not be persisted")
	s.Require().Equal(request.ClientID, byDeviceCode.ClientID)
	s.Require().Equal(request.UserID, byDeviceCode.UserID)
	s.Require().Equal(request.StandardScopes, byDeviceCode.StandardScopes)
	s.Require().Equal(request.RequestedPermissions, byDeviceCode.RequestedPermissions)
	s.Require().Equal(request.AuthorizedScopes, byDeviceCode.AuthorizedScopes)
	s.Require().Equal(request.Resources, byDeviceCode.Resources)
	s.Require().Equal(request.State, byDeviceCode.State)
	s.Require().Equal(request.AttributeCacheID, byDeviceCode.AttributeCacheID)
	s.Require().Equal(request.CompletedACR, byDeviceCode.CompletedACR)
	s.Require().True(request.AuthTime.Equal(byDeviceCode.AuthTime))
	s.Require().True(request.LastPolledAt.Equal(byDeviceCode.LastPolledAt))
	s.Require().Equal(request.PollInterval, byDeviceCode.PollInterval)
	s.Require().WithinDuration(request.ExpiryTime, byDeviceCode.ExpiryTime, time.Second)

	byUserCode, err := store.GetByUserCode(context.Background(), request.UserCode)
	s.Require().NoError(err)
	s.Require().Equal(byDeviceCode, byUserCode)

	byRequestID, err := store.GetByRequestID(context.Background(), request.RequestID)
	s.Require().NoError(err)
	s.Require().Equal(byDeviceCode, byRequestID)

	byNormalizedUserCode, err := store.GetByUserCode(context.Background(), "bcdf ghjk")
	s.Require().NoError(err)
	s.Require().Equal(byDeviceCode, byNormalizedUserCode)
	s.Require().Equal(hashUserCode(request.UserCode, []byte(testUserCodeIndexKey)),
		hashUserCode("bcdf ghjk", []byte(testUserCodeIndexKey)))
	s.Require().NotEqual(hashUserCode(request.UserCode, []byte(testUserCodeIndexKey)),
		hashUserCode(request.UserCode, []byte("different-key")))
}

func (s *DeviceAuthStoreTestSuite) TestReturnsRollbackFailure() {
	runtimeStore := runtimestoreprovidermock.NewRuntimeStoreProviderMock(s.T())
	request := &DeviceAuthRequest{
		RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode: "BCDF-GHJK", ExpiryTime: time.Now().Add(2 * time.Minute),
	}
	storeFailure := errors.New("device code storage failed")
	rollbackFailure := errors.New("user code cleanup failed")
	runtimeStore.EXPECT().PutIfNotExists(mock.Anything, providers.NamespaceDeviceAuth,
		requestIDKeyPrefix+request.RequestID, mock.Anything, mock.Anything).Return(true, nil)
	runtimeStore.EXPECT().PutIfNotExists(mock.Anything, providers.NamespaceDeviceAuth,
		userCodeKeyPrefix+hashUserCode(request.UserCode, []byte(testUserCodeIndexKey)), mock.Anything, mock.Anything).Return(true, nil)
	runtimeStore.EXPECT().PutIfNotExists(mock.Anything, providers.NamespaceDeviceAuth,
		deviceCodeKeyPrefix+request.DeviceCodeHash, mock.Anything, mock.Anything).Return(false, storeFailure)
	runtimeStore.EXPECT().Delete(mock.Anything, providers.NamespaceDeviceAuth,
		userCodeKeyPrefix+hashUserCode(request.UserCode, []byte(testUserCodeIndexKey))).Return(rollbackFailure)
	runtimeStore.EXPECT().Delete(mock.Anything, providers.NamespaceDeviceAuth,
		requestIDKeyPrefix+request.RequestID).Return(nil)

	err := newTestDeviceAuthStore(runtimeStore).Add(context.Background(), request)

	s.Require().ErrorIs(err, storeFailure)
	s.Require().ErrorIs(err, rollbackFailure)
}

func (s *DeviceAuthStoreTestSuite) TestDoesNotRetryCollisionAfterRollbackFailure() {
	runtimeStore := runtimestoreprovidermock.NewRuntimeStoreProviderMock(s.T())
	request := &DeviceAuthRequest{
		RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode: "BCDF-GHJK", ExpiryTime: time.Now().Add(2 * time.Minute),
	}
	rollbackFailure := errors.New("request ID cleanup failed")
	runtimeStore.EXPECT().PutIfNotExists(mock.Anything, providers.NamespaceDeviceAuth,
		requestIDKeyPrefix+request.RequestID, mock.Anything, mock.Anything).Return(true, nil)
	runtimeStore.EXPECT().PutIfNotExists(mock.Anything, providers.NamespaceDeviceAuth,
		userCodeKeyPrefix+hashUserCode(request.UserCode, []byte(testUserCodeIndexKey)), mock.Anything, mock.Anything).Return(false, nil)
	runtimeStore.EXPECT().Delete(mock.Anything, providers.NamespaceDeviceAuth,
		requestIDKeyPrefix+request.RequestID).Return(rollbackFailure)

	err := newTestDeviceAuthStore(runtimeStore).Add(context.Background(), request)

	s.Require().ErrorIs(err, rollbackFailure)
	s.Require().NotErrorIs(err, ErrDeviceAuthUserCodeCollision)
}

func (s *DeviceAuthStoreTestSuite) TestAddRejectsDuplicateIdentifiers() {
	tests := []struct {
		name                  string
		second                *DeviceAuthRequest
		wantErr               error
		checkRequestRollback  bool
		checkUserCodeRollback bool
	}{
		{
			name: "request ID",
			second: &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code-2"),
				UserCode: "LMNP-QRST"},
			wantErr: ErrDeviceAuthRequestIDCollision,
		},
		{
			name: "user code",
			second: &DeviceAuthRequest{RequestID: "request-2", DeviceCodeHash: hashDeviceCode("device-code-2"),
				UserCode: "BCDF-GHJK"},
			wantErr: ErrDeviceAuthUserCodeCollision, checkRequestRollback: true,
		},
		{
			name: "device code",
			second: &DeviceAuthRequest{RequestID: "request-2", DeviceCodeHash: hashDeviceCode("device-code-1"),
				UserCode: "LMNP-QRST"},
			wantErr: ErrDeviceAuthDeviceCodeCollision, checkRequestRollback: true,
			checkUserCodeRollback: true,
		},
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
			first := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code-1"),
				UserCode: "BCDF-GHJK", ExpiryTime: time.Now().Add(2 * time.Minute)}
			test.second.ExpiryTime = time.Now().Add(2 * time.Minute)
			s.Require().NoError(store.Add(context.Background(), first))
			s.Require().ErrorIs(store.Add(context.Background(), test.second), test.wantErr)

			if test.checkRequestRollback {
				_, err := store.GetByRequestID(context.Background(), test.second.RequestID)
				s.Require().ErrorIs(err, ErrDeviceAuthRequestNotFound)
			}
			if test.checkUserCodeRollback {
				_, err := store.GetByUserCode(context.Background(), test.second.UserCode)
				s.Require().ErrorIs(err, ErrDeviceAuthRequestNotFound)
			} else if errors.Is(test.wantErr, ErrDeviceAuthUserCodeCollision) {
				stored, err := store.GetByUserCode(context.Background(), test.second.UserCode)
				s.Require().NoError(err)
				s.Equal(first.RequestID, stored.RequestID)
			}
		})
	}
}

func (s *DeviceAuthStoreTestSuite) TestRejectsExpiredRequest() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	request := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode:   "BCDF-GHJK",
		ExpiryTime: time.Now().Add(-time.Second)}

	err := store.Add(context.Background(), request)
	s.Require().EqualError(err, "device authorization request already expired")
}

func (s *DeviceAuthStoreTestSuite) TestReturnsNotFound() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))

	_, err := store.GetByDeviceCode(context.Background(), "missing")
	s.Require().ErrorIs(err, ErrDeviceAuthRequestNotFound)

	_, err = store.GetByUserCode(context.Background(), "missing")
	s.Require().ErrorIs(err, ErrDeviceAuthRequestNotFound)

	_, err = store.GetByRequestID(context.Background(), "missing")
	s.Require().ErrorIs(err, ErrDeviceAuthRequestNotFound)

	_, err = store.GetByUserCode(context.Background(), " - ")
	s.Require().ErrorIs(err, ErrDeviceAuthRequestNotFound)
}

func (s *DeviceAuthStoreTestSuite) TestMarkRedeemingIsAtomic() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	request := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode: "BCDF-GHJK", State: DeviceStatePending, ExpiryTime: time.Now().Add(2 * time.Minute)}
	s.Require().NoError(store.Add(context.Background(), request))

	first, err := store.MarkRedeeming(context.Background(), request.RequestID)
	s.Require().NoError(err)
	s.Require().True(first)

	second, err := store.MarkRedeeming(context.Background(), request.RequestID)
	s.Require().NoError(err)
	s.Require().False(second)

	stored, err := store.GetByRequestID(context.Background(), request.RequestID)
	s.Require().NoError(err)
	s.Require().Equal(DeviceStateRedeeming, stored.State)
}

func (s *DeviceAuthStoreTestSuite) TestUpdatePollingDoesNotRevertConcurrentState() {
	runtimeStore := inmemory.Initialize("test-deployment")
	request := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode: "BCDF-GHJK", State: DeviceStatePending, ExpiryTime: time.Now().Add(2 * time.Minute)}
	initialStore := newTestDeviceAuthStore(runtimeStore)
	s.Require().NoError(initialStore.Add(context.Background(), request))

	mockStore := runtimestoreprovidermock.NewRuntimeStoreProviderMock(s.T())
	targetKey := deviceCodeKeyPrefix + request.DeviceCodeHash
	mockStore.EXPECT().Get(mock.Anything, providers.NamespaceDeviceAuth, mock.Anything).
		RunAndReturn(func(ctx context.Context, namespace providers.RuntimeStoreNamespace, key string) ([]byte, error) {
			data, err := runtimeStore.Get(ctx, namespace, key)
			if err != nil || key != targetKey {
				return data, err
			}
			var current DeviceAuthRequest
			if err := json.Unmarshal(data, &current); err != nil {
				return nil, err
			}
			expectedState := current.State
			current.State = DeviceStateRedeeming
			updatedData, err := json.Marshal(current)
			if err != nil {
				return nil, err
			}
			updated, err := runtimeStore.CompareFieldAndSwap(ctx, namespace, key,
				deviceStateField, string(expectedState), updatedData)
			if err != nil {
				return nil, err
			}
			if !updated {
				return nil, errors.New("concurrent state transition did not occur")
			}
			return data, nil
		}).Twice()
	mockStore.EXPECT().CompareFieldAndSwap(mock.Anything, providers.NamespaceDeviceAuth, targetKey,
		deviceStateField, string(DeviceStatePending), mock.Anything).Return(false, nil)
	store := newTestDeviceAuthStore(mockStore)
	err := store.UpdatePolling(context.Background(), request.RequestID, time.Now(), 5)
	s.Require().NoError(err)

	stored, err := initialStore.GetByRequestID(context.Background(), request.RequestID)
	s.Require().NoError(err)
	s.Require().Equal(DeviceStateRedeeming, stored.State)
	s.Require().True(stored.LastPolledAt.IsZero())
}

func (s *DeviceAuthStoreTestSuite) TestUpdateStateDoesNotOverwriteConcurrentTransition() {
	runtimeStore := inmemory.Initialize("test-deployment")
	request := &DeviceAuthRequest{RequestID: "request-1", DeviceCodeHash: hashDeviceCode("device-code"),
		UserCode: "BCDF-GHJK", State: DeviceStatePending, ExpiryTime: time.Now().Add(2 * time.Minute)}
	initialStore := newTestDeviceAuthStore(runtimeStore)
	s.Require().NoError(initialStore.Add(context.Background(), request))

	mockStore := runtimestoreprovidermock.NewRuntimeStoreProviderMock(s.T())
	targetKey := deviceCodeKeyPrefix + request.DeviceCodeHash
	mockStore.EXPECT().Get(mock.Anything, providers.NamespaceDeviceAuth, mock.Anything).
		RunAndReturn(func(ctx context.Context, namespace providers.RuntimeStoreNamespace, key string) ([]byte, error) {
			data, err := runtimeStore.Get(ctx, namespace, key)
			if err != nil || key != targetKey {
				return data, err
			}
			var current DeviceAuthRequest
			if err := json.Unmarshal(data, &current); err != nil {
				return nil, err
			}
			expectedState := current.State
			current.State = DeviceStateRedeeming
			updatedData, err := json.Marshal(current)
			if err != nil {
				return nil, err
			}
			updated, err := runtimeStore.CompareFieldAndSwap(ctx, namespace, key,
				deviceStateField, string(expectedState), updatedData)
			if err != nil {
				return nil, err
			}
			if !updated {
				return nil, errors.New("concurrent state transition did not occur")
			}
			return data, nil
		}).Twice()
	mockStore.EXPECT().CompareFieldAndSwap(mock.Anything, providers.NamespaceDeviceAuth, targetKey,
		deviceStateField, string(DeviceStatePending), mock.Anything).Return(false, nil)
	store := newTestDeviceAuthStore(mockStore)
	err := store.UpdateState(context.Background(), request.RequestID, DeviceStateExpired)
	s.Require().Error(err)

	stored, err := initialStore.GetByRequestID(context.Background(), request.RequestID)
	s.Require().NoError(err)
	s.Require().Equal(DeviceStateRedeeming, stored.State)
}
