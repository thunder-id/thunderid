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

package revocationcache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
)

func TestInitialize_DisabledReturnsNoops(t *testing.T) {
	enforcer, syncer, err := Initialize(Config{Enabled: false})

	assert.NoError(t, err)
	assert.IsType(t, noopEnforcer{}, enforcer)
	assert.IsType(t, noopSyncer{}, syncer)
	assert.NoError(t, enforcer.EnsureNotRevoked(context.Background(), "anything", ""))
}

func TestInitialize_UnsupportedSource(t *testing.T) {
	enforcer, syncer, err := Initialize(Config{Enabled: true, Source: "events"})

	assert.ErrorIs(t, err, errUnsupportedSource)
	assert.Nil(t, enforcer)
	assert.Nil(t, syncer)
}

func TestInitializeWithSource_InitialLoadPopulatesCache(t *testing.T) {
	source := &fakeSource{entries: []revokedEntry{futureEntry("jti-1")}}

	enforcer, syncer := initializeWithSource(Config{Enabled: true, SyncInterval: time.Minute}, source)

	assert.Equal(t, 1, source.callCount(), "the initial snapshot is loaded synchronously")
	assert.ErrorIs(t, enforcer.EnsureNotRevoked(context.Background(), "jti-1", ""), errTokenRevoked)
	assert.NoError(t, enforcer.EnsureNotRevoked(context.Background(), "other", ""))
	assert.NotNil(t, syncer, "initializeWithSource returns a syncer whose loop the caller starts")
}

func TestInitializeWithSource_InitialLoadFailureStartsWithEmptyDenyList(t *testing.T) {
	source := &fakeSource{err: errors.New("source unavailable")}

	enforcer, syncer := initializeWithSource(Config{Enabled: true}, source)

	require.NotNil(t, enforcer, "a failed initial load must not stop startup")
	require.NotNil(t, syncer)
	// With no snapshot loaded, the deny list is empty and nothing is treated as revoked.
	assert.NoError(t, enforcer.EnsureNotRevoked(context.Background(), "jti-1", ""))
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
