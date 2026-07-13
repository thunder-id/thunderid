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

package jti

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
)

// TestInitialize verifies that Initialize wires the returned JTIStoreInterface to the
// given store provider, i.e. it behaves like a store backed by that provider rather than
// returning a disconnected/no-op implementation.
func TestInitialize(t *testing.T) {
	storeProvider := inmemory.Initialize("test-deployment")

	store := Initialize(storeProvider)
	require.NotNil(t, store)

	ctx := context.Background()
	expiry := time.Now().Add(time.Minute)

	fresh, err := store.RecordJTI(ctx, "dpop", "jti-1", expiry)
	require.NoError(t, err)
	assert.True(t, fresh)

	replay, err := store.RecordJTI(ctx, "dpop", "jti-1", expiry)
	require.NoError(t, err)
	assert.False(t, replay)
}
