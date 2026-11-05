// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/security"

	"github.com/thunder-id/thunderid/internal/system/config"
)

func TestInitialize_DisabledReturnsNoops(t *testing.T) {
	enforcer, syncer, err := Initialize(Config{Enabled: false}, nil, nil)

	assert.NoError(t, err)
	assert.IsType(t, noopEnforcer{}, enforcer)
	assert.IsType(t, noopSyncer{}, syncer)
	assert.NoError(t, enforcer.EnsureNotRevoked(context.Background(), security.RevocationIdentity{JTI: "anything"}))
}

func TestInitialize_UnsupportedSource(t *testing.T) {
	enforcer, syncer, err := Initialize(Config{Enabled: true, Source: "endpoint"}, nil, nil)

	assert.ErrorIs(t, err, errUnsupportedSource)
	assert.Nil(t, enforcer)
	assert.Nil(t, syncer)
}

func TestInitializeWithSource_InitialLoadPopulatesCache(t *testing.T) {
	source := &fakeSource{entries: []revokedEntry{futureEntry("jti-1")}}

	enforcer, syncer := initializeWithSource(Config{Enabled: true, SyncInterval: time.Minute}, source)

	assert.Equal(t, 1, source.callCount(), "the initial snapshot is loaded synchronously")
	assert.ErrorIs(t, enforcer.EnsureNotRevoked(context.Background(),
		security.RevocationIdentity{JTI: "jti-1"}), errTokenRevoked)
	assert.NoError(t, enforcer.EnsureNotRevoked(context.Background(), security.RevocationIdentity{JTI: "other"}))
	assert.NotNil(t, syncer, "initializeWithSource returns a syncer whose loop the caller starts")
}

func TestInitializeWithSource_InitialLoadFailureStartsWithEmptyDenyList(t *testing.T) {
	source := &fakeSource{err: errors.New("source unavailable")}

	enforcer, syncer := initializeWithSource(Config{Enabled: true}, source)

	require.NotNil(t, enforcer, "a failed initial load must not stop startup")
	require.NotNil(t, syncer)
	// With no snapshot loaded, the deny list is empty and nothing is treated as revoked.
	assert.NoError(t, enforcer.EnsureNotRevoked(context.Background(), security.RevocationIdentity{JTI: "jti-1"}))
}

func TestSelectSource(t *testing.T) {
	// newDBSource reads the deployment identifier from the server runtime, so it must be initialized.
	require.NoError(t, config.InitializeServerRuntime("test", &config.Config{}))
	defer config.ResetServerRuntime()

	dbFromDefault, err := selectSource("")
	assert.NoError(t, err)
	assert.NotNil(t, dbFromDefault)

	dbExplicit, err := selectSource(sourceDB)
	assert.NoError(t, err)
	assert.NotNil(t, dbExplicit)

	_, err = selectSource("endpoint")
	assert.ErrorIs(t, err, errUnsupportedSource)
}

func TestInitialize_EventsRequiresTrustedIssuer(t *testing.T) {
	enforcer, syncer, err := Initialize(Config{Enabled: true, Source: sourceEvents}, http.NewServeMux(), nil)

	assert.Error(t, err)
	assert.Nil(t, enforcer)
	assert.Nil(t, syncer)
}

func TestInitialize_EventsMountsThePushEndpoint(t *testing.T) {
	mux := http.NewServeMux()

	enforcer, syncer, err := Initialize(Config{Enabled: true, Source: sourceEvents, TrustedIssuer: testIssuer},
		mux, nil)

	require.NoError(t, err)
	assert.IsType(t, noopSyncer{}, syncer, "events are pushed, so nothing is synced on a schedule")
	assert.NoError(t, enforcer.EnsureNotRevoked(context.Background(), security.RevocationIdentity{JTI: "x"}))
	_, pattern := mux.Handler(httptest.NewRequest(http.MethodPost, EventsPath, nil))
	assert.Equal(t, "POST "+EventsPath, pattern)
}
