// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
)

// recordingListener collects the events a revocation service announces.
type recordingListener struct {
	events []sharedrevocation.Event
}

func (l *recordingListener) OnEvent(_ context.Context, event sharedrevocation.Event) {
	l.events = append(l.events, event)
}

func newAnnouncingService(t *testing.T) (*revocationService, *revocationStoreInterfaceMock, *recordingListener) {
	store := newRevocationStoreInterfaceMock(t)
	service := buildRevocationService(nil, store, time.Hour, 0, false, nil, testMaxEntriesPerChange)
	listener := &recordingListener{}
	require.NoError(t, service.events.Hook().Add(listener))
	return service, store, listener
}

func TestRevocationService_AnnouncesARefreshTokenRevocation(t *testing.T) {
	service, store, listener := newAnnouncingService(t)
	store.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(nil)
	expiry := time.Now().Add(time.Hour).UTC()

	require.NoError(t, service.RevokeRefreshToken(context.Background(), "rotated-jti", expiry))

	require.Len(t, listener.events, 1)
	assert.Equal(t, "rotated-jti", listener.events[0].JTI)
	assert.Equal(t, RevocationReasonRefreshRotation, listener.events[0].Reason)
	assert.True(t, listener.events[0].ExpiryTime.Equal(expiry))
}

func TestRevocationService_AnnouncesATokenFamilyRevocation(t *testing.T) {
	service, store, listener := newAnnouncingService(t)
	store.On("insertCriterion", mock.Anything, mock.Anything).Return(nil)

	require.NoError(t, service.RevokeTokenFamily(context.Background(), "tfid-1", RevocationReasonSessionLogout))

	require.Len(t, listener.events, 1)
	assert.Equal(t, Criterion{Type: CriterionTypeTokenFamily, Value: "tfid-1"}, listener.events[0].Criterion)
	assert.Empty(t, listener.events[0].JTI)
	assert.Equal(t, RevocationReasonSessionLogout, listener.events[0].Reason)
}

func TestRevocationService_AnnouncesEveryRowOfABatch(t *testing.T) {
	service, store, listener := newAnnouncingService(t)
	store.On("insertCriteria", mock.Anything, mock.Anything).Return(nil)
	cutoff := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)

	svcErr := service.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{
		{Criterion: Criterion{Type: CriterionTypeSubject, Value: "user-1"}, Mode: RevocationModeAll,
			Reason: RevocationReasonUserDeleted},
		{Criterion: Criterion{Type: CriterionTypeApplicationKey, Value: "client-1"},
			Mode: RevocationModeBeforeAction, Cutoff: cutoff, Reason: RevocationReasonApplicationSecretRegenerated},
	})

	require.Nil(t, svcErr)
	require.Len(t, listener.events, 2)
	assert.Equal(t, "user-1", listener.events[0].Criterion.Value)
	assert.Equal(t, CriterionTypeApplicationKey, listener.events[1].Criterion.Type)
	assert.True(t, listener.events[1].RevokedAt.Equal(cutoff), "a bounded revocation announces its cutoff")
}

func TestRevocationService_AnnouncesNothingWhenTheWriteFails(t *testing.T) {
	service, store, listener := newAnnouncingService(t)
	store.On("InsertRevokedToken", mock.Anything, mock.Anything).Return(errors.New("db down"))
	store.On("insertCriteria", mock.Anything, mock.Anything).Return(errors.New("db down"))

	assert.Error(t, service.RevokeRefreshToken(context.Background(), "jti", time.Now().Add(time.Hour)))
	assert.NotNil(t, service.RevokeCriteriaBatch(context.Background(), []CriteriaRevocation{{
		Criterion: Criterion{Type: CriterionTypeSubject, Value: "user-1"}, Mode: RevocationModeAll,
		Reason: RevocationReasonUserDeleted}}))

	assert.Empty(t, listener.events)
}
